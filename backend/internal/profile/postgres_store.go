package profile

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
)

type PostgresStore struct {
	db *sql.DB
}

func NewPostgresStore(db *sql.DB) *PostgresStore {
	return &PostgresStore{db: db}
}

func (s *PostgresStore) GetByUserID(ctx context.Context, userID string) (Profile, error) {
	return s.scanProfile(s.db.QueryRowContext(ctx, `
		SELECT id::text, user_id::text, headline, summary, target_roles_json, visibility,
			template_id, theme_json, published_at, created_at, updated_at
		FROM profiles
		WHERE user_id = $1
	`, userID))
}

func (s *PostgresStore) EnsureDefaultByUserID(ctx context.Context, userID string) (Profile, error) {
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO profiles (user_id)
		VALUES ($1)
		ON CONFLICT (user_id) DO NOTHING
	`, userID); err != nil {
		return Profile{}, err
	}
	return s.GetByUserID(ctx, userID)
}

func (s *PostgresStore) UpdateByUserID(ctx context.Context, userID string, input UpdateInput) (Profile, error) {
	headlineSet, headline := stringValue(input.Headline)
	summarySet, summary := stringValue(input.Summary)
	visibilitySet, visibility := stringValue(input.Visibility)
	templateSet, templateID := stringValue(input.TemplateID)

	targetRolesSet := input.TargetRoles != nil
	targetRolesJSON := "[]"
	if input.TargetRoles != nil {
		encoded, err := json.Marshal(input.TargetRoles)
		if err != nil {
			return Profile{}, err
		}
		targetRolesJSON = string(encoded)
	}

	themeSet := input.Theme != nil
	themeJSON := "{}"
	if input.Theme != nil {
		encoded, err := json.Marshal(input.Theme)
		if err != nil {
			return Profile{}, err
		}
		themeJSON = string(encoded)
	}

	return s.scanProfile(s.db.QueryRowContext(ctx, `
		UPDATE profiles
		SET headline = CASE WHEN $2 THEN $3 ELSE headline END,
			summary = CASE WHEN $4 THEN $5 ELSE summary END,
			target_roles_json = CASE WHEN $6 THEN $7::jsonb ELSE target_roles_json END,
			visibility = CASE WHEN $8 THEN $9 ELSE visibility END,
			template_id = CASE WHEN $10 THEN $11 ELSE template_id END,
			theme_json = CASE WHEN $12 THEN $13::jsonb ELSE theme_json END
		WHERE user_id = $1
		RETURNING id::text, user_id::text, headline, summary, target_roles_json, visibility,
			template_id, theme_json, published_at, created_at, updated_at
	`, userID,
		headlineSet, headline,
		summarySet, summary,
		targetRolesSet, targetRolesJSON,
		visibilitySet, visibility,
		templateSet, templateID,
		themeSet, themeJSON,
	))
}

type profileRow interface {
	Scan(dest ...any) error
}

func (s *PostgresStore) scanProfile(row profileRow) (Profile, error) {
	var profile Profile
	var targetRolesJSON []byte
	var themeJSON []byte
	var publishedAt sql.NullTime

	err := row.Scan(
		&profile.ID,
		&profile.UserID,
		&profile.Headline,
		&profile.Summary,
		&targetRolesJSON,
		&profile.Visibility,
		&profile.TemplateID,
		&themeJSON,
		&publishedAt,
		&profile.CreatedAt,
		&profile.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Profile{}, ErrNotFound
	}
	if err != nil {
		return Profile{}, err
	}
	if err := json.Unmarshal(targetRolesJSON, &profile.TargetRoles); err != nil {
		return Profile{}, err
	}
	if err := json.Unmarshal(themeJSON, &profile.Theme); err != nil {
		return Profile{}, err
	}
	if profile.TargetRoles == nil {
		profile.TargetRoles = []string{}
	}
	if profile.Theme == nil {
		profile.Theme = map[string]any{}
	}
	if publishedAt.Valid {
		profile.PublishedAt = &publishedAt.Time
	}
	return profile, nil
}

func stringValue(value *string) (bool, string) {
	if value == nil {
		return false, ""
	}
	return true, *value
}
