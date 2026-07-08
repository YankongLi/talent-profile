package resume

import (
	"context"
	"database/sql"
)

type PostgresStore struct {
	db *sql.DB
}

func NewPostgresStore(db *sql.DB) *PostgresStore {
	return &PostgresStore{db: db}
}

func (s *PostgresStore) Create(ctx context.Context, input CreateInput) (Resume, error) {
	var resume Resume
	err := s.db.QueryRowContext(ctx, `
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
	).Scan(
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
	if err != nil {
		return Resume{}, err
	}
	return resume, nil
}
