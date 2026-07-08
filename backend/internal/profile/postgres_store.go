package profile

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
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

func (s *PostgresStore) ListSectionsByUserID(ctx context.Context, userID string) ([]Section, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT ps.id::text, ps.profile_id::text, ps.section_type, ps.content_json,
			ps.sort_order, ps.is_visible, ps.is_user_confirmed, ps.created_at, ps.updated_at
		FROM profile_sections ps
		JOIN profiles p ON p.id = ps.profile_id
		WHERE p.user_id = $1
		ORDER BY ps.sort_order ASC, ps.created_at ASC, ps.id ASC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	sections := []Section{}
	for rows.Next() {
		section, err := scanSection(rows)
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

func (s *PostgresStore) CreateSection(ctx context.Context, profileID string, input CreateSectionInput) (Section, error) {
	sortOrder := 0
	if input.SortOrder != nil {
		sortOrder = *input.SortOrder
	}
	isVisible := true
	if input.IsVisible != nil {
		isVisible = *input.IsVisible
	}
	isUserConfirmed := false
	if input.IsUserConfirmed != nil {
		isUserConfirmed = *input.IsUserConfirmed
	}
	contentJSON, err := json.Marshal(input.Content)
	if err != nil {
		return Section{}, err
	}

	return scanSection(s.db.QueryRowContext(ctx, `
		INSERT INTO profile_sections (
			profile_id, section_type, content_json, sort_order, is_visible, is_user_confirmed
		)
		VALUES ($1, $2, $3::jsonb, $4, $5, $6)
		RETURNING id::text, profile_id::text, section_type, content_json,
			sort_order, is_visible, is_user_confirmed, created_at, updated_at
	`, profileID, input.SectionType, string(contentJSON), sortOrder, isVisible, isUserConfirmed))
}

func (s *PostgresStore) GetSectionByUserID(ctx context.Context, userID string, sectionID string) (Section, error) {
	return scanSection(s.db.QueryRowContext(ctx, `
		SELECT ps.id::text, ps.profile_id::text, ps.section_type, ps.content_json,
			ps.sort_order, ps.is_visible, ps.is_user_confirmed, ps.created_at, ps.updated_at
		FROM profile_sections ps
		JOIN profiles p ON p.id = ps.profile_id
		WHERE p.user_id = $1
			AND ps.id = $2
	`, userID, sectionID))
}

func (s *PostgresStore) UpdateSectionByUserID(ctx context.Context, userID string, sectionID string, input UpdateSectionInput) (Section, error) {
	sectionTypeSet, sectionType := stringValue(input.SectionType)
	sortOrderSet, sortOrder := intValue(input.SortOrder)
	isVisibleSet, isVisible := boolValue(input.IsVisible)
	isUserConfirmedSet, isUserConfirmed := boolValue(input.IsUserConfirmed)

	contentSet := input.Content != nil
	contentJSON := "{}"
	if input.Content != nil {
		encoded, err := json.Marshal(input.Content)
		if err != nil {
			return Section{}, err
		}
		contentJSON = string(encoded)
	}

	return scanSection(s.db.QueryRowContext(ctx, `
		UPDATE profile_sections ps
		SET section_type = CASE WHEN $3 THEN $4 ELSE ps.section_type END,
			content_json = CASE WHEN $5 THEN $6::jsonb ELSE ps.content_json END,
			sort_order = CASE WHEN $7 THEN $8 ELSE ps.sort_order END,
			is_visible = CASE WHEN $9 THEN $10 ELSE ps.is_visible END,
			is_user_confirmed = CASE WHEN $11 THEN $12 ELSE ps.is_user_confirmed END
		FROM profiles p
		WHERE p.id = ps.profile_id
			AND p.user_id = $1
			AND ps.id = $2
		RETURNING ps.id::text, ps.profile_id::text, ps.section_type, ps.content_json,
			ps.sort_order, ps.is_visible, ps.is_user_confirmed, ps.created_at, ps.updated_at
	`, userID, sectionID,
		sectionTypeSet, sectionType,
		contentSet, contentJSON,
		sortOrderSet, sortOrder,
		isVisibleSet, isVisible,
		isUserConfirmedSet, isUserConfirmed,
	))
}

func (s *PostgresStore) DeleteSectionByUserID(ctx context.Context, userID string, sectionID string) error {
	result, err := s.db.ExecContext(ctx, `
		DELETE FROM profile_sections ps
		USING profiles p
		WHERE p.id = ps.profile_id
			AND p.user_id = $1
			AND ps.id = $2
	`, userID, sectionID)
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

func (s *PostgresStore) ReorderSectionsByUserID(ctx context.Context, userID string, sectionIDs []string) ([]Section, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var profileID string
	if err := tx.QueryRowContext(ctx, `
		SELECT id::text
		FROM profiles
		WHERE user_id = $1
		FOR UPDATE
	`, userID).Scan(&profileID); errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, err
	}

	rows, err := tx.QueryContext(ctx, `
		SELECT ps.id::text
		FROM profile_sections ps
		WHERE ps.profile_id = $1
		ORDER BY ps.sort_order ASC, ps.created_at ASC, ps.id ASC
		FOR UPDATE
	`, profileID)
	if err != nil {
		return nil, err
	}

	existing := make(map[string]struct{}, len(sectionIDs))
	for rows.Next() {
		var sectionID string
		if err := rows.Scan(&sectionID); err != nil {
			rows.Close()
			return nil, err
		}
		existing[sectionID] = struct{}{}
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(existing) != len(sectionIDs) {
		return nil, ErrNotFound
	}
	for _, sectionID := range sectionIDs {
		if _, ok := existing[sectionID]; !ok {
			return nil, ErrNotFound
		}
	}

	for index, sectionID := range sectionIDs {
		result, err := tx.ExecContext(ctx, `
			UPDATE profile_sections ps
			SET sort_order = $3
			FROM profiles p
			WHERE p.id = ps.profile_id
				AND p.user_id = $1
				AND ps.id = $2
		`, userID, sectionID, index)
		if err != nil {
			return nil, err
		}
		rowsAffected, err := result.RowsAffected()
		if err != nil {
			return nil, err
		}
		if rowsAffected != 1 {
			return nil, fmt.Errorf("profile section reorder affected %d rows", rowsAffected)
		}
	}

	sections, err := listSectionsByUserIDTx(ctx, tx, userID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return sections, nil
}

type profileRow interface {
	Scan(dest ...any) error
}

type sectionRow interface {
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

func scanSection(row sectionRow) (Section, error) {
	var section Section
	var contentJSON []byte

	err := row.Scan(
		&section.ID,
		&section.ProfileID,
		&section.SectionType,
		&contentJSON,
		&section.SortOrder,
		&section.IsVisible,
		&section.IsUserConfirmed,
		&section.CreatedAt,
		&section.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Section{}, ErrNotFound
	}
	if err != nil {
		return Section{}, err
	}
	if err := json.Unmarshal(contentJSON, &section.Content); err != nil {
		return Section{}, err
	}
	if section.Content == nil {
		section.Content = map[string]any{}
	}
	return section, nil
}

func listSectionsByUserIDTx(ctx context.Context, tx *sql.Tx, userID string) ([]Section, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT ps.id::text, ps.profile_id::text, ps.section_type, ps.content_json,
			ps.sort_order, ps.is_visible, ps.is_user_confirmed, ps.created_at, ps.updated_at
		FROM profile_sections ps
		JOIN profiles p ON p.id = ps.profile_id
		WHERE p.user_id = $1
		ORDER BY ps.sort_order ASC, ps.created_at ASC, ps.id ASC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	sections := []Section{}
	for rows.Next() {
		section, err := scanSection(rows)
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

func stringValue(value *string) (bool, string) {
	if value == nil {
		return false, ""
	}
	return true, *value
}

func intValue(value *int) (bool, int) {
	if value == nil {
		return false, 0
	}
	return true, *value
}

func boolValue(value *bool) (bool, bool) {
	if value == nil {
		return false, false
	}
	return true, *value
}
