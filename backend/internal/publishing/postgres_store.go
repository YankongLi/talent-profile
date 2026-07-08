package publishing

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
