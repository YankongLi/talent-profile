package publishing

import (
	"errors"
	"strings"
)

const (
	MinSlugLength = 3
	MaxSlugLength = 30
)

var (
	ErrSlugRequired      = errors.New("slug is required")
	ErrSlugTooShort      = errors.New("slug is too short")
	ErrSlugTooLong       = errors.New("slug is too long")
	ErrSlugInvalidChar   = errors.New("slug contains invalid characters")
	ErrSlugInvalidHyphen = errors.New("slug cannot start or end with hyphen")
	ErrSlugReserved      = errors.New("slug is reserved")
)

var reservedSlugs = map[string]struct{}{
	"admin":  {},
	"api":    {},
	"app":    {},
	"assets": {},
	"cdn":    {},
	"login":  {},
	"mail":   {},
	"static": {},
	"www":    {},
}

func ValidateSlug(slug string) error {
	if slug == "" {
		return ErrSlugRequired
	}
	if len(slug) < MinSlugLength {
		return ErrSlugTooShort
	}
	if len(slug) > MaxSlugLength {
		return ErrSlugTooLong
	}
	if strings.HasPrefix(slug, "-") || strings.HasSuffix(slug, "-") {
		return ErrSlugInvalidHyphen
	}
	if _, ok := reservedSlugs[slug]; ok {
		return ErrSlugReserved
	}

	for _, r := range slug {
		if r >= 'a' && r <= 'z' {
			continue
		}
		if r >= '0' && r <= '9' {
			continue
		}
		if r == '-' {
			continue
		}
		return ErrSlugInvalidChar
	}

	return nil
}

func IsReservedSlug(slug string) bool {
	_, ok := reservedSlugs[slug]
	return ok
}
