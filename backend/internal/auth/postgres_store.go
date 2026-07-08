package auth

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

type PostgresStore struct {
	db *sql.DB
}

func NewPostgresStore(db *sql.DB) *PostgresStore {
	return &PostgresStore{db: db}
}

func (s *PostgresStore) SaveEmailCode(ctx context.Context, email string, codeHash string, expiresAt time.Time) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO email_login_codes (email, code_hash, expires_at)
		VALUES ($1, $2, $3)
	`, email, codeHash, expiresAt)
	return err
}

func (s *PostgresStore) LatestEmailCode(ctx context.Context, email string, now time.Time) (EmailCode, error) {
	var code EmailCode
	err := s.db.QueryRowContext(ctx, `
		SELECT id::text, email, code_hash, attempts, expires_at
		FROM email_login_codes
		WHERE lower(email) = lower($1)
			AND consumed_at IS NULL
			AND expires_at > $2
		ORDER BY created_at DESC
		LIMIT 1
	`, email, now).Scan(&code.ID, &code.Email, &code.CodeHash, &code.Attempts, &code.ExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return EmailCode{}, ErrEmailCodeNotFound
	}
	if err != nil {
		return EmailCode{}, err
	}
	return code, nil
}

func (s *PostgresStore) RecordEmailCodeFailure(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE email_login_codes
		SET attempts = attempts + 1
		WHERE id = $1
			AND consumed_at IS NULL
	`, id)
	return err
}

func (s *PostgresStore) ConsumeEmailCode(ctx context.Context, id string, consumedAt time.Time) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE email_login_codes
		SET consumed_at = $2
		WHERE id = $1
			AND consumed_at IS NULL
	`, id, consumedAt)
	if err != nil {
		return err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return ErrInvalidCode
	}
	return nil
}

func (s *PostgresStore) UpsertVerifiedUser(ctx context.Context, email string, verifiedAt time.Time) (User, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return User{}, err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	var user User
	var verifiedAtValue sql.NullTime
	err = tx.QueryRowContext(ctx, `
		INSERT INTO users (email, email_verified_at, status)
		VALUES ($1, $2, 'active')
		ON CONFLICT (lower(email)) WHERE deleted_at IS NULL
		DO UPDATE SET
			email_verified_at = COALESCE(users.email_verified_at, EXCLUDED.email_verified_at),
			status = 'active',
			updated_at = now()
		RETURNING id::text, email, email_verified_at, status
	`, email, verifiedAt).Scan(&user.ID, &user.Email, &verifiedAtValue, &user.Status)
	if err != nil {
		return User{}, err
	}
	if verifiedAtValue.Valid {
		user.EmailVerifiedAt = &verifiedAtValue.Time
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO profiles (user_id)
		VALUES ($1)
		ON CONFLICT (user_id) DO NOTHING
	`, user.ID); err != nil {
		return User{}, err
	}
	return user, tx.Commit()
}

func (s *PostgresStore) CreateSession(ctx context.Context, userID string, tokenHash string, expiresAt time.Time) (Session, error) {
	var session Session
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO user_sessions (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)
		RETURNING id::text, user_id::text, expires_at
	`, userID, tokenHash, expiresAt).Scan(&session.ID, &session.UserID, &session.ExpiresAt)
	if err != nil {
		return Session{}, err
	}
	return session, nil
}

func (s *PostgresStore) RevokeSessionByTokenHash(ctx context.Context, tokenHash string, revokedAt time.Time) (bool, error) {
	result, err := s.db.ExecContext(ctx, `
		UPDATE user_sessions
		SET revoked_at = $2
		WHERE token_hash = $1
			AND revoked_at IS NULL
			AND expires_at > $2
	`, tokenHash, revokedAt)
	if err != nil {
		return false, err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return rowsAffected > 0, nil
}

func (s *PostgresStore) CurrentUserBySessionTokenHash(ctx context.Context, tokenHash string, now time.Time) (User, error) {
	var user User
	var verifiedAtValue sql.NullTime
	err := s.db.QueryRowContext(ctx, `
		SELECT u.id::text, u.email, u.email_verified_at, u.status
		FROM user_sessions us
		JOIN users u ON u.id = us.user_id
		WHERE us.token_hash = $1
			AND us.revoked_at IS NULL
			AND us.expires_at > $2
			AND u.deleted_at IS NULL
			AND u.status = 'active'
	`, tokenHash, now).Scan(&user.ID, &user.Email, &verifiedAtValue, &user.Status)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrUnauthorized
	}
	if err != nil {
		return User{}, err
	}
	if verifiedAtValue.Valid {
		user.EmailVerifiedAt = &verifiedAtValue.Time
	}
	return user, nil
}

func (s *PostgresStore) SoftDeleteUserBySessionTokenHash(ctx context.Context, tokenHash string, deletedAt time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	var userID string
	err = tx.QueryRowContext(ctx, `
		SELECT u.id::text
		FROM user_sessions us
		JOIN users u ON u.id = us.user_id
		WHERE us.token_hash = $1
			AND us.revoked_at IS NULL
			AND us.expires_at > $2
			AND u.deleted_at IS NULL
			AND u.status = 'active'
		FOR UPDATE OF u
	`, tokenHash, deletedAt).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrUnauthorized
	}
	if err != nil {
		return err
	}

	result, err := tx.ExecContext(ctx, `
		UPDATE users
		SET deleted_at = $2,
			status = 'disabled'
		WHERE id = $1
			AND deleted_at IS NULL
	`, userID, deletedAt)
	if err != nil {
		return err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return ErrUnauthorized
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE user_sessions
		SET revoked_at = $2
		WHERE user_id = $1
			AND revoked_at IS NULL
	`, userID, deletedAt); err != nil {
		return err
	}

	return tx.Commit()
}
