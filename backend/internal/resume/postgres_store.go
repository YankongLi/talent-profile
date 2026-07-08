package resume

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

type PostgresStore struct {
	db *sql.DB
}

func NewPostgresStore(db *sql.DB) *PostgresStore {
	return &PostgresStore{db: db}
}

func (s *PostgresStore) Create(ctx context.Context, input CreateInput) (Resume, error) {
	return scanResume(s.db.QueryRowContext(ctx, `
		INSERT INTO resumes (
			user_id, storage_key, original_filename, mime_type, file_size, content_hash, parse_status
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id::text, user_id::text, storage_key, original_filename, mime_type,
			file_size, content_hash, parse_status, COALESCE(parse_error, ''), created_at
	`, input.UserID,
		input.StorageKey,
		input.OriginalFilename,
		input.MimeType,
		input.FileSize,
		input.ContentHash,
		input.ParseStatus,
	))
}

func (s *PostgresStore) GetByUserID(ctx context.Context, userID string, resumeID string) (Resume, error) {
	return scanResume(s.db.QueryRowContext(ctx, `
		SELECT id::text, user_id::text, storage_key, original_filename, mime_type,
			file_size, content_hash, parse_status, COALESCE(parse_error, ''), created_at
		FROM resumes
		WHERE id = $1
			AND user_id = $2
			AND deleted_at IS NULL
	`, resumeID, userID))
}

func (s *PostgresStore) GetByID(ctx context.Context, resumeID string) (Resume, error) {
	return scanResume(s.db.QueryRowContext(ctx, `
		SELECT id::text, user_id::text, storage_key, original_filename, mime_type,
			file_size, content_hash, parse_status, COALESCE(parse_error, ''), created_at
		FROM resumes
		WHERE id = $1
			AND deleted_at IS NULL
	`, resumeID))
}

func (s *PostgresStore) SoftDeleteByUserID(ctx context.Context, userID string, resumeID string, deletedAt time.Time) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE resumes
		SET deleted_at = $3
		WHERE id = $1
			AND user_id = $2
			AND deleted_at IS NULL
	`, resumeID, userID, deletedAt)
	if err != nil {
		return err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PostgresStore) MarkParsing(ctx context.Context, resumeID string) error {
	return s.updateParseState(ctx, resumeID, StatusParsing, nil)
}

func (s *PostgresStore) MarkParsed(ctx context.Context, resumeID string, result ParseResult) error {
	sensitiveFieldsJSON, err := json.Marshal(result.SensitiveFields)
	if err != nil {
		return err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO resume_parse_results (
			resume_id, extracted_text_encrypted, redacted_text, sensitive_fields_json
		)
		VALUES ($1, $2, $3, $4::jsonb)
		ON CONFLICT (resume_id) DO UPDATE SET
			extracted_text_encrypted = EXCLUDED.extracted_text_encrypted,
			redacted_text = EXCLUDED.redacted_text,
			sensitive_fields_json = EXCLUDED.sensitive_fields_json
	`, resumeID, result.ExtractedText, result.RedactedText, string(sensitiveFieldsJSON)); err != nil {
		return err
	}

	if err := updateParseStateTx(ctx, tx, resumeID, StatusParsed, nil); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *PostgresStore) MarkFailed(ctx context.Context, resumeID string, reason string) error {
	return s.updateParseState(ctx, resumeID, StatusFailed, &reason)
}

func (s *PostgresStore) updateParseState(ctx context.Context, resumeID string, status string, parseError *string) error {
	return updateParseStateTx(ctx, s.db, resumeID, status, parseError)
}

type parseStateExecutor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func updateParseStateTx(ctx context.Context, executor parseStateExecutor, resumeID string, status string, parseError *string) error {
	parseErrorSet := parseError != nil
	parseErrorValue := ""
	if parseError != nil {
		parseErrorValue = *parseError
	}

	result, err := executor.ExecContext(ctx, `
		UPDATE resumes
		SET parse_status = $2,
			parse_error = CASE
				WHEN $3 THEN $4
				WHEN $2 IN ('parsing', 'parsed') THEN NULL
				ELSE parse_error
			END
		WHERE id = $1
			AND deleted_at IS NULL
	`, resumeID, status, parseErrorSet, parseErrorValue)
	if err != nil {
		return err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

type resumeScanner interface {
	Scan(dest ...any) error
}

func scanResume(row resumeScanner) (Resume, error) {
	var resume Resume
	err := row.Scan(
		&resume.ID,
		&resume.UserID,
		&resume.StorageKey,
		&resume.OriginalFilename,
		&resume.MimeType,
		&resume.FileSize,
		&resume.ContentHash,
		&resume.ParseStatus,
		&resume.ParseError,
		&resume.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Resume{}, ErrNotFound
	}
	if err != nil {
		return Resume{}, err
	}
	return resume, nil
}
