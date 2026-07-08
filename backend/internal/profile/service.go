package profile

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

const (
	VisibilityDraft    = "draft"
	VisibilityUnlisted = "unlisted"
	VisibilityPublic   = "public"

	defaultTemplateID      = "default"
	maxHeadlineLength      = 160
	maxSummaryLength       = 2000
	maxRoleCount           = 20
	maxRoleLength          = 80
	maxTemplateLength      = 80
	maxThemeBytes          = 20 * 1024
	maxSectionIDLength     = 120
	maxSectionTypeLength   = 80
	maxSectionContentBytes = 64 * 1024
	maxSortOrder           = 100_000
)

var (
	ErrNotFound          = errors.New("profile not found")
	ErrInvalidHeadline   = errors.New("invalid headline")
	ErrInvalidSummary    = errors.New("invalid summary")
	ErrInvalidTargetRole = errors.New("invalid target role")
	ErrInvalidVisibility = errors.New("invalid visibility")
	ErrInvalidTemplateID = errors.New("invalid template id")
	ErrInvalidTheme      = errors.New("invalid theme")
	ErrInvalidSectionID  = errors.New("invalid section id")
	ErrInvalidSection    = errors.New("invalid section")
	ErrInvalidSortOrder  = errors.New("invalid sort order")
)

type Store interface {
	GetByUserID(ctx context.Context, userID string) (Profile, error)
	EnsureDefaultByUserID(ctx context.Context, userID string) (Profile, error)
	UpdateByUserID(ctx context.Context, userID string, input UpdateInput) (Profile, error)
	PublishByUserID(ctx context.Context, userID string, visibility string) (Profile, error)
	UnpublishByUserID(ctx context.Context, userID string) (Profile, error)
	ListSectionsByUserID(ctx context.Context, userID string) ([]Section, error)
	CreateSection(ctx context.Context, profileID string, input CreateSectionInput) (Section, error)
	GetSectionByUserID(ctx context.Context, userID string, sectionID string) (Section, error)
	UpdateSectionByUserID(ctx context.Context, userID string, sectionID string, input UpdateSectionInput) (Section, error)
	DeleteSectionByUserID(ctx context.Context, userID string, sectionID string) error
	ReorderSectionsByUserID(ctx context.Context, userID string, sectionIDs []string) ([]Section, error)
}

type Service struct {
	store Store
}

type Profile struct {
	ID          string         `json:"id"`
	UserID      string         `json:"user_id"`
	Headline    string         `json:"headline"`
	Summary     string         `json:"summary"`
	TargetRoles []string       `json:"target_roles"`
	Visibility  string         `json:"visibility"`
	TemplateID  string         `json:"template_id"`
	Theme       map[string]any `json:"theme"`
	PublishedAt *time.Time     `json:"published_at,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

type UpdateInput struct {
	Headline    *string
	Summary     *string
	TargetRoles *[]string
	Visibility  *string
	TemplateID  *string
	Theme       *map[string]any
}

type Section struct {
	ID              string         `json:"id"`
	ProfileID       string         `json:"profile_id"`
	SectionType     string         `json:"section_type"`
	Content         map[string]any `json:"content"`
	SortOrder       int            `json:"sort_order"`
	IsVisible       bool           `json:"is_visible"`
	IsUserConfirmed bool           `json:"is_user_confirmed"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
}

type CreateSectionInput struct {
	SectionType     string
	Content         map[string]any
	SortOrder       *int
	IsVisible       *bool
	IsUserConfirmed *bool
}

type UpdateSectionInput struct {
	SectionType     *string
	Content         *map[string]any
	SortOrder       *int
	IsVisible       *bool
	IsUserConfirmed *bool
}

func NewService(store Store) *Service {
	return &Service{store: store}
}

func (s *Service) Get(ctx context.Context, userID string) (Profile, error) {
	profile, err := s.store.GetByUserID(ctx, userID)
	if errors.Is(err, ErrNotFound) {
		return s.store.EnsureDefaultByUserID(ctx, userID)
	}
	return profile, err
}

func (s *Service) Update(ctx context.Context, userID string, input UpdateInput) (Profile, error) {
	if err := validateUpdate(input); err != nil {
		return Profile{}, err
	}
	if !input.hasChanges() {
		return s.Get(ctx, userID)
	}
	if _, err := s.store.GetByUserID(ctx, userID); errors.Is(err, ErrNotFound) {
		if _, ensureErr := s.store.EnsureDefaultByUserID(ctx, userID); ensureErr != nil {
			return Profile{}, ensureErr
		}
	} else if err != nil {
		return Profile{}, err
	}
	return s.store.UpdateByUserID(ctx, userID, input)
}

func (s *Service) Publish(ctx context.Context, userID string, visibility string) (Profile, error) {
	visibility = strings.TrimSpace(visibility)
	if visibility == "" {
		visibility = VisibilityPublic
	}
	if !validPublishedVisibility(visibility) {
		return Profile{}, ErrInvalidVisibility
	}
	return s.store.PublishByUserID(ctx, userID, visibility)
}

func (s *Service) Unpublish(ctx context.Context, userID string) (Profile, error) {
	return s.store.UnpublishByUserID(ctx, userID)
}

func (s *Service) Sections(ctx context.Context, userID string) ([]Section, error) {
	if _, err := s.Get(ctx, userID); err != nil {
		return nil, err
	}
	return s.store.ListSectionsByUserID(ctx, userID)
}

func (s *Service) CreateSection(ctx context.Context, userID string, input CreateSectionInput) (Section, error) {
	if err := validateCreateSection(&input); err != nil {
		return Section{}, err
	}
	profile, err := s.Get(ctx, userID)
	if err != nil {
		return Section{}, err
	}
	return s.store.CreateSection(ctx, profile.ID, input)
}

func (s *Service) UpdateSection(ctx context.Context, userID string, sectionID string, input UpdateSectionInput) (Section, error) {
	if err := validateSectionID(sectionID); err != nil {
		return Section{}, err
	}
	if err := validateUpdateSection(input); err != nil {
		return Section{}, err
	}
	if !input.hasChanges() {
		return s.store.GetSectionByUserID(ctx, userID, sectionID)
	}
	return s.store.UpdateSectionByUserID(ctx, userID, sectionID, input)
}

func (s *Service) DeleteSection(ctx context.Context, userID string, sectionID string) error {
	if err := validateSectionID(sectionID); err != nil {
		return err
	}
	return s.store.DeleteSectionByUserID(ctx, userID, sectionID)
}

func (s *Service) ReorderSections(ctx context.Context, userID string, sectionIDs []string) ([]Section, error) {
	if err := validateSectionIDs(sectionIDs); err != nil {
		return nil, err
	}
	return s.store.ReorderSectionsByUserID(ctx, userID, sectionIDs)
}

func validateUpdate(input UpdateInput) error {
	if input.Headline != nil {
		*input.Headline = strings.TrimSpace(*input.Headline)
		if len(*input.Headline) > maxHeadlineLength {
			return ErrInvalidHeadline
		}
	}
	if input.Summary != nil {
		*input.Summary = strings.TrimSpace(*input.Summary)
		if len(*input.Summary) > maxSummaryLength {
			return ErrInvalidSummary
		}
	}
	if input.TargetRoles != nil {
		if len(*input.TargetRoles) > maxRoleCount {
			return ErrInvalidTargetRole
		}
		seen := make(map[string]struct{}, len(*input.TargetRoles))
		for index, role := range *input.TargetRoles {
			normalized := strings.TrimSpace(role)
			if normalized == "" || len(normalized) > maxRoleLength {
				return ErrInvalidTargetRole
			}
			key := strings.ToLower(normalized)
			if _, ok := seen[key]; ok {
				return ErrInvalidTargetRole
			}
			seen[key] = struct{}{}
			(*input.TargetRoles)[index] = normalized
		}
	}
	if input.Visibility != nil && !validVisibility(*input.Visibility) {
		return ErrInvalidVisibility
	}
	if input.TemplateID != nil {
		templateID := strings.TrimSpace(*input.TemplateID)
		if templateID == "" || len(templateID) > maxTemplateLength {
			return ErrInvalidTemplateID
		}
		*input.TemplateID = templateID
	}
	if input.Theme != nil {
		encoded, err := json.Marshal(input.Theme)
		if err != nil || len(encoded) > maxThemeBytes {
			return ErrInvalidTheme
		}
	}
	return nil
}

func validateCreateSection(input *CreateSectionInput) error {
	sectionType := strings.TrimSpace(input.SectionType)
	if !validSectionType(sectionType) {
		return ErrInvalidSection
	}
	input.SectionType = sectionType
	if input.Content == nil {
		input.Content = map[string]any{}
	}
	if err := validateSectionContent(input.Content); err != nil {
		return err
	}
	if input.SortOrder != nil && !validSortOrder(*input.SortOrder) {
		return ErrInvalidSortOrder
	}
	return nil
}

func validateUpdateSection(input UpdateSectionInput) error {
	if input.SectionType != nil {
		*input.SectionType = strings.TrimSpace(*input.SectionType)
		if !validSectionType(*input.SectionType) {
			return ErrInvalidSection
		}
	}
	if input.Content != nil {
		if *input.Content == nil {
			*input.Content = map[string]any{}
		}
		if err := validateSectionContent(*input.Content); err != nil {
			return err
		}
	}
	if input.SortOrder != nil && !validSortOrder(*input.SortOrder) {
		return ErrInvalidSortOrder
	}
	return nil
}

func validateSectionID(sectionID string) error {
	sectionID = strings.TrimSpace(sectionID)
	if sectionID == "" || len(sectionID) > maxSectionIDLength {
		return ErrInvalidSectionID
	}
	return nil
}

func validateSectionIDs(sectionIDs []string) error {
	if len(sectionIDs) == 0 {
		return ErrInvalidSectionID
	}
	seen := make(map[string]struct{}, len(sectionIDs))
	for index, sectionID := range sectionIDs {
		normalized := strings.TrimSpace(sectionID)
		if normalized == "" || len(normalized) > maxSectionIDLength {
			return ErrInvalidSectionID
		}
		if _, ok := seen[normalized]; ok {
			return ErrInvalidSectionID
		}
		seen[normalized] = struct{}{}
		sectionIDs[index] = normalized
	}
	return nil
}

func validSectionType(value string) bool {
	return value != "" && len(value) <= maxSectionTypeLength
}

func validateSectionContent(content map[string]any) error {
	encoded, err := json.Marshal(content)
	if err != nil || len(encoded) > maxSectionContentBytes {
		return ErrInvalidSection
	}
	return nil
}

func validSortOrder(value int) bool {
	return value >= 0 && value <= maxSortOrder
}

func validVisibility(value string) bool {
	switch value {
	case VisibilityDraft, VisibilityUnlisted, VisibilityPublic:
		return true
	default:
		return false
	}
}

func validPublishedVisibility(value string) bool {
	switch value {
	case VisibilityUnlisted, VisibilityPublic:
		return true
	default:
		return false
	}
}

func (input UpdateInput) hasChanges() bool {
	return input.Headline != nil ||
		input.Summary != nil ||
		input.TargetRoles != nil ||
		input.Visibility != nil ||
		input.TemplateID != nil ||
		input.Theme != nil
}

func (input UpdateSectionInput) hasChanges() bool {
	return input.SectionType != nil ||
		input.Content != nil ||
		input.SortOrder != nil ||
		input.IsVisible != nil ||
		input.IsUserConfirmed != nil
}

func DefaultTemplateID() string {
	return defaultTemplateID
}
