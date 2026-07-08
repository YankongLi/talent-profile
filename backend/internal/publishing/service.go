package publishing

import (
	"context"
	"errors"
	"strings"
	"time"
)

const (
	DefaultDomainRedirectTTL = 7 * 24 * time.Hour

	SlugUnavailableRequired      = "required"
	SlugUnavailableTooShort      = "too_short"
	SlugUnavailableTooLong       = "too_long"
	SlugUnavailableInvalidChar   = "invalid_char"
	SlugUnavailableInvalidHyphen = "invalid_hyphen"
	SlugUnavailableReserved      = "reserved"
	SlugUnavailableTaken         = "taken"
)

var ErrSlugTaken = errors.New("slug is already taken")
var ErrPublicProfileNotFound = errors.New("public profile not found")

type Store interface {
	SlugExists(ctx context.Context, slug string) (bool, error)
	SetPrimaryDomainByUserID(ctx context.Context, userID string, slug string, redirectTTL time.Duration) (SetPrimaryDomainResult, error)
	GetPublicProfileBySlug(ctx context.Context, slug string, now time.Time) (PublicProfileResult, error)
}

type Service struct {
	store Store
}

type Domain struct {
	ID                string     `json:"id"`
	ProfileID         string     `json:"profile_id"`
	Slug              string     `json:"slug"`
	IsPrimary         bool       `json:"is_primary"`
	RedirectToSlug    string     `json:"redirect_to_slug,omitempty"`
	RedirectExpiresAt *time.Time `json:"redirect_expires_at,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

type SlugAvailability struct {
	Slug      string `json:"slug"`
	Available bool   `json:"available"`
	Reason    string `json:"reason,omitempty"`
}

type SetPrimaryDomainResult struct {
	Domain         Domain  `json:"domain"`
	PreviousDomain *Domain `json:"previous_domain,omitempty"`
}

type PublicProfileResult struct {
	Page     *PublicProfilePage `json:"page,omitempty"`
	Redirect *PublicRedirect    `json:"redirect,omitempty"`
}

type PublicRedirect struct {
	Slug           string `json:"slug"`
	RedirectToSlug string `json:"redirect_to_slug"`
}

type PublicProfilePage struct {
	Slug          string          `json:"slug"`
	CanonicalSlug string          `json:"canonical_slug"`
	Visibility    string          `json:"visibility"`
	NoIndex       bool            `json:"noindex"`
	Profile       PublicProfile   `json:"profile"`
	Sections      []PublicSection `json:"sections"`
}

type PublicProfile struct {
	Headline    string         `json:"headline"`
	Summary     string         `json:"summary"`
	TargetRoles []string       `json:"target_roles"`
	TemplateID  string         `json:"template_id"`
	Theme       map[string]any `json:"theme"`
	PublishedAt *time.Time     `json:"published_at,omitempty"`
}

type PublicSection struct {
	SectionType string         `json:"section_type"`
	Content     map[string]any `json:"content"`
	SortOrder   int            `json:"sort_order"`
}

func NewService(store Store) *Service {
	return &Service{store: store}
}

func (s *Service) CheckSlug(ctx context.Context, slug string) (SlugAvailability, error) {
	slug = strings.TrimSpace(slug)
	if err := ValidateSlug(slug); err != nil {
		return SlugAvailability{
			Slug:      slug,
			Available: false,
			Reason:    slugUnavailableReason(err),
		}, nil
	}

	exists, err := s.store.SlugExists(ctx, slug)
	if err != nil {
		return SlugAvailability{}, err
	}
	if exists {
		return SlugAvailability{
			Slug:      slug,
			Available: false,
			Reason:    SlugUnavailableTaken,
		}, nil
	}

	return SlugAvailability{
		Slug:      slug,
		Available: true,
	}, nil
}

func (s *Service) SetPrimaryDomain(ctx context.Context, userID string, slug string) (SetPrimaryDomainResult, error) {
	slug = strings.TrimSpace(slug)
	if err := ValidateSlug(slug); err != nil {
		return SetPrimaryDomainResult{}, err
	}
	return s.store.SetPrimaryDomainByUserID(ctx, userID, slug, DefaultDomainRedirectTTL)
}

func (s *Service) GetPublicProfile(ctx context.Context, slug string) (PublicProfileResult, error) {
	slug = strings.TrimSpace(slug)
	if err := ValidateSlug(slug); err != nil {
		return PublicProfileResult{}, ErrPublicProfileNotFound
	}
	return s.store.GetPublicProfileBySlug(ctx, slug, time.Now().UTC())
}

func slugUnavailableReason(err error) string {
	switch {
	case errors.Is(err, ErrSlugRequired):
		return SlugUnavailableRequired
	case errors.Is(err, ErrSlugTooShort):
		return SlugUnavailableTooShort
	case errors.Is(err, ErrSlugTooLong):
		return SlugUnavailableTooLong
	case errors.Is(err, ErrSlugInvalidChar):
		return SlugUnavailableInvalidChar
	case errors.Is(err, ErrSlugInvalidHyphen):
		return SlugUnavailableInvalidHyphen
	case errors.Is(err, ErrSlugReserved):
		return SlugUnavailableReserved
	default:
		return SlugUnavailableInvalidChar
	}
}
