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
	var user User
	var verifiedAtValue sql.NullTime
	err := s.db.QueryRowContext(ctx, `
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
	return user, nil
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
