package publishing

import (
	"context"
	"errors"
	"testing"
	"time"
)

type testStore struct {
	slugs        map[string]bool
	err          error
	setResult    SetPrimaryDomainResult
	setErr       error
	setUserID    string
	setSlug      string
	setTTL       time.Duration
	publicResult PublicProfileResult
	publicErr    error
	publicSlug   string
}

func (s *testStore) SlugExists(_ context.Context, slug string) (bool, error) {
	if s.err != nil {
		return false, s.err
	}
	return s.slugs[slug], nil
}

func (s *testStore) SetPrimaryDomainByUserID(_ context.Context, userID string, slug string, redirectTTL time.Duration) (SetPrimaryDomainResult, error) {
	s.setUserID = userID
	s.setSlug = slug
	s.setTTL = redirectTTL
	if s.setErr != nil {
		return SetPrimaryDomainResult{}, s.setErr
	}
	return s.setResult, nil
}

func (s *testStore) GetPublicProfileBySlug(_ context.Context, slug string, _ time.Time) (PublicProfileResult, error) {
	s.publicSlug = slug
	if s.publicErr != nil {
		return PublicProfileResult{}, s.publicErr
	}
	return s.publicResult, nil
}

func TestServiceCheckSlugAvailable(t *testing.T) {
	service := NewService(&testStore{slugs: map[string]bool{}})

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
	service := NewService(&testStore{slugs: map[string]bool{"zhangsan": true}})

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
			service := NewService(&testStore{slugs: map[string]bool{}})

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
	service := NewService(&testStore{err: wantErr})

	_, err := service.CheckSlug(context.Background(), "zhangsan")
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}
}

func TestServiceSetPrimaryDomainNormalizesAndDelegates(t *testing.T) {
	store := &testStore{
		setResult: SetPrimaryDomainResult{
			Domain: Domain{Slug: "zhangsan", IsPrimary: true},
		},
	}
	service := NewService(store)

	result, err := service.SetPrimaryDomain(context.Background(), "user_1", " zhangsan ")
	if err != nil {
		t.Fatalf("SetPrimaryDomain returned error: %v", err)
	}
	if result.Domain.Slug != "zhangsan" {
		t.Fatalf("domain slug = %q, want zhangsan", result.Domain.Slug)
	}
	if store.setUserID != "user_1" {
		t.Fatalf("set user id = %q, want user_1", store.setUserID)
	}
	if store.setSlug != "zhangsan" {
		t.Fatalf("set slug = %q, want zhangsan", store.setSlug)
	}
	if store.setTTL != DefaultDomainRedirectTTL {
		t.Fatalf("redirect ttl = %s, want %s", store.setTTL, DefaultDomainRedirectTTL)
	}
}

func TestServiceSetPrimaryDomainRejectsInvalidSlug(t *testing.T) {
	service := NewService(&testStore{})

	_, err := service.SetPrimaryDomain(context.Background(), "user_1", "api")
	if !errors.Is(err, ErrSlugReserved) {
		t.Fatalf("error = %v, want %v", err, ErrSlugReserved)
	}
}

func TestServiceSetPrimaryDomainPropagatesTaken(t *testing.T) {
	service := NewService(&testStore{setErr: ErrSlugTaken})

	_, err := service.SetPrimaryDomain(context.Background(), "user_1", "zhangsan")
	if !errors.Is(err, ErrSlugTaken) {
		t.Fatalf("error = %v, want %v", err, ErrSlugTaken)
	}
}

func TestServiceGetPublicProfileNormalizesSlug(t *testing.T) {
	store := &testStore{
		publicResult: PublicProfileResult{
			Page: &PublicProfilePage{Slug: "zhangsan"},
		},
	}
	service := NewService(store)

	result, err := service.GetPublicProfile(context.Background(), " zhangsan ")
	if err != nil {
		t.Fatalf("GetPublicProfile returned error: %v", err)
	}
	if result.Page == nil || result.Page.Slug != "zhangsan" {
		t.Fatalf("result = %#v, want page zhangsan", result)
	}
	if store.publicSlug != "zhangsan" {
		t.Fatalf("public slug = %q, want zhangsan", store.publicSlug)
	}
}

func TestServiceGetPublicProfileRejectsInvalidSlugAsNotFound(t *testing.T) {
	service := NewService(&testStore{})

	_, err := service.GetPublicProfile(context.Background(), "Zhangsan")
	if !errors.Is(err, ErrPublicProfileNotFound) {
		t.Fatalf("error = %v, want %v", err, ErrPublicProfileNotFound)
	}
}
