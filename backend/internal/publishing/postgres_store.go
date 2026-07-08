package publishing

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

type PostgresStore struct {
	db *sql.DB
}

func NewPostgresStore(db *sql.DB) *PostgresStore {
	return &PostgresStore{db: db}
}

func (s *PostgresStore) SlugExists(ctx context.Context, slug string) (bool, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM profile_domains
			WHERE slug = $1
		)
	`, slug).Scan(&exists)
	return exists, err
}

func (s *PostgresStore) SetPrimaryDomainByUserID(ctx context.Context, userID string, slug string, redirectTTL time.Duration) (SetPrimaryDomainResult, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return SetPrimaryDomainResult{}, err
	}
	defer tx.Rollback()

	profileID, err := ensureProfileIDTx(ctx, tx, userID)
	if err != nil {
		return SetPrimaryDomainResult{}, err
	}

	existing, err := getDomainBySlugTx(ctx, tx, slug)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return SetPrimaryDomainResult{}, err
	}
	if err == nil && existing.ProfileID != profileID {
		return SetPrimaryDomainResult{}, ErrSlugTaken
	}

	var previousDomain *Domain
	redirectExpiresAt := time.Now().UTC().Add(redirectTTL)
	previousRows, err := tx.QueryContext(ctx, `
		UPDATE profile_domains
		SET is_primary = false,
			redirect_to_slug = $2,
			redirect_expires_at = $3
		WHERE profile_id = $1
			AND is_primary = true
			AND slug <> $2
		RETURNING id::text, profile_id::text, slug, is_primary, redirect_to_slug,
			redirect_expires_at, created_at, updated_at
	`, profileID, slug, redirectExpiresAt)
	if err != nil {
		return SetPrimaryDomainResult{}, err
	}
	for previousRows.Next() {
		domain, err := scanDomain(previousRows)
		if err != nil {
			previousRows.Close()
			return SetPrimaryDomainResult{}, err
		}
		if previousDomain == nil {
			previousDomain = &domain
		}
	}
	if err := previousRows.Close(); err != nil {
		return SetPrimaryDomainResult{}, err
	}
	if err := previousRows.Err(); err != nil {
		return SetPrimaryDomainResult{}, err
	}

	var primaryDomain Domain
	if existing.ID != "" && existing.ProfileID == profileID {
		primaryDomain, err = scanDomain(tx.QueryRowContext(ctx, `
			UPDATE profile_domains
			SET is_primary = true,
				redirect_to_slug = NULL,
				redirect_expires_at = NULL
			WHERE id = $1
			RETURNING id::text, profile_id::text, slug, is_primary, redirect_to_slug,
				redirect_expires_at, created_at, updated_at
		`, existing.ID))
	} else {
		primaryDomain, err = scanDomain(tx.QueryRowContext(ctx, `
			INSERT INTO profile_domains (profile_id, slug, is_primary)
			VALUES ($1, $2, true)
			RETURNING id::text, profile_id::text, slug, is_primary, redirect_to_slug,
				redirect_expires_at, created_at, updated_at
		`, profileID, slug))
	}
	if isUniqueViolation(err) {
		return SetPrimaryDomainResult{}, ErrSlugTaken
	}
	if err != nil {
		return SetPrimaryDomainResult{}, err
	}

	if err := tx.Commit(); err != nil {
		return SetPrimaryDomainResult{}, err
	}
	return SetPrimaryDomainResult{
		Domain:         primaryDomain,
		PreviousDomain: previousDomain,
	}, nil
}

type domainRow interface {
	Scan(dest ...any) error
}

func ensureProfileIDTx(ctx context.Context, tx *sql.Tx, userID string) (string, error) {
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO profiles (user_id)
		VALUES ($1)
		ON CONFLICT (user_id) DO NOTHING
	`, userID); err != nil {
		return "", err
	}

	var profileID string
	err := tx.QueryRowContext(ctx, `
		SELECT id::text
		FROM profiles
		WHERE user_id = $1
		FOR UPDATE
	`, userID).Scan(&profileID)
	return profileID, err
}

func getDomainBySlugTx(ctx context.Context, tx *sql.Tx, slug string) (Domain, error) {
	return scanDomain(tx.QueryRowContext(ctx, `
		SELECT id::text, profile_id::text, slug, is_primary, redirect_to_slug,
			redirect_expires_at, created_at, updated_at
		FROM profile_domains
		WHERE slug = $1
		FOR UPDATE
	`, slug))
}

func scanDomain(row domainRow) (Domain, error) {
	var domain Domain
	var redirectToSlug sql.NullString
	var redirectExpiresAt sql.NullTime

	if err := row.Scan(
		&domain.ID,
		&domain.ProfileID,
		&domain.Slug,
		&domain.IsPrimary,
		&redirectToSlug,
		&redirectExpiresAt,
		&domain.CreatedAt,
		&domain.UpdatedAt,
	); err != nil {
		return Domain{}, err
	}
	if redirectToSlug.Valid {
		domain.RedirectToSlug = redirectToSlug.String
	}
	if redirectExpiresAt.Valid {
		domain.RedirectExpiresAt = &redirectExpiresAt.Time
	}
	return domain, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
