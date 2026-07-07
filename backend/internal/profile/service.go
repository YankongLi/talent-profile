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

	defaultTemplateID = "default"
	maxHeadlineLength = 160
	maxSummaryLength  = 2000
	maxRoleCount      = 20
	maxRoleLength     = 80
	maxTemplateLength = 80
	maxThemeBytes     = 20 * 1024
)

var (
	ErrNotFound          = errors.New("profile not found")
	ErrInvalidHeadline   = errors.New("invalid headline")
	ErrInvalidSummary    = errors.New("invalid summary")
	ErrInvalidTargetRole = errors.New("invalid target role")
	ErrInvalidVisibility = errors.New("invalid visibility")
	ErrInvalidTemplateID = errors.New("invalid template id")
	ErrInvalidTheme      = errors.New("invalid theme")
)

type Store interface {
	GetByUserID(ctx context.Context, userID string) (Profile, error)
	EnsureDefaultByUserID(ctx context.Context, userID string) (Profile, error)
	UpdateByUserID(ctx context.Context, userID string, input UpdateInput) (Profile, error)
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

func validVisibility(value string) bool {
	switch value {
	case VisibilityDraft, VisibilityUnlisted, VisibilityPublic:
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

func DefaultTemplateID() string {
	return defaultTemplateID
}
