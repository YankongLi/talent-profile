package publishing

import (
	"context"
	"errors"
	"strings"
)

const (
	SlugUnavailableRequired      = "required"
	SlugUnavailableTooShort      = "too_short"
	SlugUnavailableTooLong       = "too_long"
	SlugUnavailableInvalidChar   = "invalid_char"
	SlugUnavailableInvalidHyphen = "invalid_hyphen"
	SlugUnavailableReserved      = "reserved"
	SlugUnavailableTaken         = "taken"
)

type Store interface {
	SlugExists(ctx context.Context, slug string) (bool, error)
}

type Service struct {
	store Store
}

type SlugAvailability struct {
	Slug      string `json:"slug"`
	Available bool   `json:"available"`
	Reason    string `json:"reason,omitempty"`
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
