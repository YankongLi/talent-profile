package resume

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
	return s.updateParseState(ctx, resumeID, StatusParsing, nil, nil)
}

func (s *PostgresStore) MarkParsed(ctx context.Context, resumeID string, extractedText []byte) error {
	return s.updateParseState(ctx, resumeID, StatusParsed, extractedText, nil)
}

func (s *PostgresStore) MarkFailed(ctx context.Context, resumeID string, reason string) error {
	return s.updateParseState(ctx, resumeID, StatusFailed, nil, &reason)
}

func (s *PostgresStore) updateParseState(ctx context.Context, resumeID string, status string, extractedText []byte, parseError *string) error {
	extractedTextSet := extractedText != nil
	parseErrorSet := parseError != nil
	parseErrorValue := ""
	if parseError != nil {
		parseErrorValue = *parseError
	}

	result, err := s.db.ExecContext(ctx, `
		UPDATE resumes
		SET parse_status = $2,
			extracted_text_encrypted = CASE WHEN $3 THEN $4 ELSE extracted_text_encrypted END,
			parse_error = CASE
				WHEN $5 THEN $6
				WHEN $2 IN ('parsing', 'parsed') THEN NULL
				ELSE parse_error
			END
		WHERE id = $1
			AND deleted_at IS NULL
	`, resumeID, status, extractedTextSet, extractedText, parseErrorSet, parseErrorValue)
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
