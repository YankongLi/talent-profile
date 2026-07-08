package publishing

import (
	"context"
	"errors"
	"testing"
)

type testStore struct {
	slugs map[string]bool
	err   error
}

func (s testStore) SlugExists(_ context.Context, slug string) (bool, error) {
	if s.err != nil {
		return false, s.err
	}
	return s.slugs[slug], nil
}

func TestServiceCheckSlugAvailable(t *testing.T) {
	service := NewService(testStore{slugs: map[string]bool{}})

	result, err := service.CheckSlug(context.Background(), " zhangsan ")
	if err != nil {
		t.Fatalf("CheckSlug returned error: %v", err)
	}
	if result.Slug != "zhangsan" {
		t.Fatalf("slug = %q, want zhangsan", result.Slug)
	}
	if !result.Available {
		t.Fatalf("available = false, reason %q", result.Reason)
	}
	if result.Reason != "" {
		t.Fatalf("reason = %q, want empty", result.Reason)
	}
}

func TestServiceCheckSlugTaken(t *testing.T) {
	service := NewService(testStore{slugs: map[string]bool{"zhangsan": true}})

	result, err := service.CheckSlug(context.Background(), "zhangsan")
	if err != nil {
		t.Fatalf("CheckSlug returned error: %v", err)
	}
	if result.Available {
		t.Fatal("slug should not be available")
	}
	if result.Reason != SlugUnavailableTaken {
		t.Fatalf("reason = %q, want %q", result.Reason, SlugUnavailableTaken)
	}
}

func TestServiceCheckSlugInvalid(t *testing.T) {
	tests := []struct {
		name string
		slug string
		want string
	}{
		{name: "required", slug: "   ", want: SlugUnavailableRequired},
		{name: "too short", slug: "ab", want: SlugUnavailableTooShort},
		{name: "too long", slug: "abcdefghijklmnopqrstuvwxyzabcde", want: SlugUnavailableTooLong},
		{name: "invalid char", slug: "zhang_san", want: SlugUnavailableInvalidChar},
		{name: "invalid hyphen", slug: "-zhangsan", want: SlugUnavailableInvalidHyphen},
		{name: "reserved", slug: "api", want: SlugUnavailableReserved},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewService(testStore{slugs: map[string]bool{}})

			result, err := service.CheckSlug(context.Background(), tt.slug)
			if err != nil {
				t.Fatalf("CheckSlug returned error: %v", err)
			}
			if result.Available {
				t.Fatal("slug should not be available")
			}
			if result.Reason != tt.want {
				t.Fatalf("reason = %q, want %q", result.Reason, tt.want)
			}
		})
	}
}

func TestServiceCheckSlugPropagatesStoreError(t *testing.T) {
	wantErr := errors.New("database failed")
	service := NewService(testStore{err: wantErr})

	_, err := service.CheckSlug(context.Background(), "zhangsan")
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}
}
