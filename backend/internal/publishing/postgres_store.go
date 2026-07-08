package publishing

import (
	"context"
	"database/sql"
	"encoding/json"
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

func (s *PostgresStore) GetPublicProfileBySlug(ctx context.Context, slug string, now time.Time) (PublicProfileResult, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT p.id::text, pd.slug, pd.is_primary, pd.redirect_to_slug, pd.redirect_expires_at,
			p.headline, p.summary, p.target_roles_json, p.visibility, p.template_id,
			p.theme_json, p.published_at
		FROM profile_domains pd
		JOIN profiles p ON p.id = pd.profile_id
		WHERE pd.slug = $1
			AND p.visibility IN ('unlisted', 'public')
			AND (
				pd.is_primary = true
				OR (
					pd.redirect_to_slug IS NOT NULL
					AND pd.redirect_expires_at IS NOT NULL
					AND pd.redirect_expires_at > $2
				)
			)
	`, slug, now)

	record, err := scanPublicProfileRecord(row)
	if errors.Is(err, sql.ErrNoRows) {
		return PublicProfileResult{}, ErrPublicProfileNotFound
	}
	if err != nil {
		return PublicProfileResult{}, err
	}

	if !record.isPrimary && record.redirectToSlug != "" {
		return PublicProfileResult{
			Redirect: &PublicRedirect{
				Slug:           record.slug,
				RedirectToSlug: record.redirectToSlug,
			},
		}, nil
	}

	sections, err := s.listPublicSections(ctx, record.profileID)
	if err != nil {
		return PublicProfileResult{}, err
	}
	return PublicProfileResult{
		Page: &PublicProfilePage{
			Slug:          record.slug,
			CanonicalSlug: record.slug,
			Visibility:    record.visibility,
			NoIndex:       record.visibility == "unlisted",
			Profile: PublicProfile{
				Headline:    record.headline,
				Summary:     record.summary,
				TargetRoles: record.targetRoles,
				TemplateID:  record.templateID,
				Theme:       record.theme,
				PublishedAt: record.publishedAt,
			},
			Sections: sections,
		},
	}, nil
}

type domainRow interface {
	Scan(dest ...any) error
}

type publicProfileRecord struct {
	profileID      string
	slug           string
	isPrimary      bool
	redirectToSlug string
	headline       string
	summary        string
	targetRoles    []string
	visibility     string
	templateID     string
	theme          map[string]any
	publishedAt    *time.Time
}

type publicProfileRow interface {
	Scan(dest ...any) error
}

type publicSectionRow interface {
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

func scanPublicProfileRecord(row publicProfileRow) (publicProfileRecord, error) {
	var record publicProfileRecord
	var redirectToSlug sql.NullString
	var redirectExpiresAt sql.NullTime
	var targetRolesJSON []byte
	var themeJSON []byte
	var publishedAt sql.NullTime

	if err := row.Scan(
		&record.profileID,
		&record.slug,
		&record.isPrimary,
		&redirectToSlug,
		&redirectExpiresAt,
		&record.headline,
		&record.summary,
		&targetRolesJSON,
		&record.visibility,
		&record.templateID,
		&themeJSON,
		&publishedAt,
	); err != nil {
		return publicProfileRecord{}, err
	}
	if redirectToSlug.Valid {
		record.redirectToSlug = redirectToSlug.String
	}
	if err := json.Unmarshal(targetRolesJSON, &record.targetRoles); err != nil {
		return publicProfileRecord{}, err
	}
	if err := json.Unmarshal(themeJSON, &record.theme); err != nil {
		return publicProfileRecord{}, err
	}
	if record.targetRoles == nil {
		record.targetRoles = []string{}
	}
	if record.theme == nil {
		record.theme = map[string]any{}
	}
	if publishedAt.Valid {
		record.publishedAt = &publishedAt.Time
	}
	return record, nil
}

func (s *PostgresStore) listPublicSections(ctx context.Context, profileID string) ([]PublicSection, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT section_type, content_json, sort_order
		FROM profile_sections
		WHERE profile_id = $1
			AND is_visible = true
		ORDER BY sort_order ASC, created_at ASC, id ASC
	`, profileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	sections := []PublicSection{}
	for rows.Next() {
		section, err := scanPublicSection(rows)
		if err != nil {
			return nil, err
		}
		sections = append(sections, section)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return sections, nil
}

func scanPublicSection(row publicSectionRow) (PublicSection, error) {
	var section PublicSection
	var contentJSON []byte

	if err := row.Scan(&section.SectionType, &contentJSON, &section.SortOrder); err != nil {
		return PublicSection{}, err
	}
	if err := json.Unmarshal(contentJSON, &section.Content); err != nil {
		return PublicSection{}, err
	}
	if section.Content == nil {
		section.Content = map[string]any{}
	}
	return section, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
