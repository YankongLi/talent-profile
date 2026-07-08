package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/YankongLi/talent-profile/backend/internal/auth"
	"github.com/YankongLi/talent-profile/backend/internal/publishing"
)

type domainTestStore struct {
	slugs     map[string]bool
	err       error
	setResult publishing.SetPrimaryDomainResult
	setErr    error
	setUserID string
	setSlug   string
	setTTL    time.Duration
}

func (s *domainTestStore) SlugExists(_ context.Context, slug string) (bool, error) {
	if s.err != nil {
		return false, s.err
	}
	return s.slugs[slug], nil
}

func (s *domainTestStore) SetPrimaryDomainByUserID(_ context.Context, userID string, slug string, redirectTTL time.Duration) (publishing.SetPrimaryDomainResult, error) {
	s.setUserID = userID
	s.setSlug = slug
	s.setTTL = redirectTTL
	if s.setErr != nil {
		return publishing.SetPrimaryDomainResult{}, s.setErr
	}
	return s.setResult, nil
}

func TestDomainCheckReturnsAvailability(t *testing.T) {
	router := NewRouter(
		testConfig(),
		WithPublishingService(publishing.NewService(&domainTestStore{
			slugs: map[string]bool{"taken": true},
		})),
	)

	tests := []struct {
		name      string
		path      string
		slug      string
		available bool
		reason    string
	}{
		{name: "available", path: "/api/v1/domains/check?slug=zhangsan", slug: "zhangsan", available: true},
		{name: "taken", path: "/api/v1/domains/check?slug=taken", slug: "taken", available: false, reason: publishing.SlugUnavailableTaken},
		{name: "reserved", path: "/api/v1/domains/check?slug=api", slug: "api", available: false, reason: publishing.SlugUnavailableReserved},
		{name: "invalid", path: "/api/v1/domains/check?slug=Zhangsan", slug: "Zhangsan", available: false, reason: publishing.SlugUnavailableInvalidChar},
		{name: "missing", path: "/api/v1/domains/check", slug: "", available: false, reason: publishing.SlugUnavailableRequired},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d, body %s", rec.Code, http.StatusOK, rec.Body.String())
			}
			var body checkDomainResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if body.Slug != tt.slug {
				t.Fatalf("slug = %q, want %q", body.Slug, tt.slug)
			}
			if body.Available != tt.available {
				t.Fatalf("available = %t, want %t", body.Available, tt.available)
			}
			if body.Reason != tt.reason {
				t.Fatalf("reason = %q, want %q", body.Reason, tt.reason)
			}
		})
	}
}

func TestDomainCheckPropagatesStoreError(t *testing.T) {
	router := NewRouter(
		testConfig(),
		WithPublishingService(publishing.NewService(&domainTestStore{
			err: errors.New("database failed"),
		})),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/domains/check?slug=zhangsan", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	var body ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Error.Code != ErrCodeInternal {
		t.Fatalf("error code = %q, want %q", body.Error.Code, ErrCodeInternal)
	}
}

func TestDomainUpdateSetsPrimaryDomain(t *testing.T) {
	authStore := newAuthTestStore()
	authService := auth.NewService(authStore, authTestSender{}, auth.Config{
		CodeTTL:         time.Minute,
		SessionTTL:      time.Hour,
		ExposeDebugCode: true,
	})
	redirectExpiresAt := time.Unix(1_700_000_600, 0).UTC()
	store := &domainTestStore{
		setResult: publishing.SetPrimaryDomainResult{
			Domain: publishing.Domain{
				ID:        "domain_2",
				ProfileID: "profile_user_1",
				Slug:      "zhangsan",
				IsPrimary: true,
			},
			PreviousDomain: &publishing.Domain{
				ID:                "domain_1",
				ProfileID:         "profile_user_1",
				Slug:              "oldslug",
				IsPrimary:         false,
				RedirectToSlug:    "zhangsan",
				RedirectExpiresAt: &redirectExpiresAt,
			},
		},
	}
	router := NewRouter(
		testConfig(),
		WithAuthService(authService),
		WithPublishingService(publishing.NewService(store)),
	)
	token := loginProfileTestUser(t, authService, "user@example.com")

	req := httptest.NewRequest(http.MethodPut, "/api/v1/profile/domain", bytesWithJSON(`{"slug":" zhangsan "}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var body updateDomainResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Domain.Slug != "zhangsan" || !body.Domain.IsPrimary {
		t.Fatalf("domain = %#v, want primary zhangsan", body.Domain)
	}
	if body.PreviousDomain == nil || body.PreviousDomain.RedirectToSlug != "zhangsan" {
		t.Fatalf("previous domain = %#v, want redirect to zhangsan", body.PreviousDomain)
	}
	if store.setUserID != "user_1" {
		t.Fatalf("set user id = %q, want user_1", store.setUserID)
	}
	if store.setSlug != "zhangsan" {
		t.Fatalf("set slug = %q, want zhangsan", store.setSlug)
	}
}

func TestDomainUpdateRequiresSession(t *testing.T) {
	router := NewRouter(
		testConfig(),
		WithAuthService(auth.NewService(newAuthTestStore(), authTestSender{}, auth.Config{
			CodeTTL:         time.Minute,
			SessionTTL:      time.Hour,
			ExposeDebugCode: true,
		})),
		WithPublishingService(publishing.NewService(&domainTestStore{})),
	)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/profile/domain", bytesWithJSON(`{"slug":"zhangsan"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestDomainUpdateRejectsInvalidSlug(t *testing.T) {
	authService := auth.NewService(newAuthTestStore(), authTestSender{}, auth.Config{
		CodeTTL:         time.Minute,
		SessionTTL:      time.Hour,
		ExposeDebugCode: true,
	})
	router := NewRouter(
		testConfig(),
		WithAuthService(authService),
		WithPublishingService(publishing.NewService(&domainTestStore{})),
	)
	token := loginProfileTestUser(t, authService, "user@example.com")

	req := httptest.NewRequest(http.MethodPut, "/api/v1/profile/domain", bytesWithJSON(`{"slug":"api"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestDomainUpdateRejectsTakenSlug(t *testing.T) {
	authService := auth.NewService(newAuthTestStore(), authTestSender{}, auth.Config{
		CodeTTL:         time.Minute,
		SessionTTL:      time.Hour,
		ExposeDebugCode: true,
	})
	router := NewRouter(
		testConfig(),
		WithAuthService(authService),
		WithPublishingService(publishing.NewService(&domainTestStore{
			setErr: publishing.ErrSlugTaken,
		})),
	)
	token := loginProfileTestUser(t, authService, "user@example.com")

	req := httptest.NewRequest(http.MethodPut, "/api/v1/profile/domain", bytesWithJSON(`{"slug":"taken"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusConflict)
	}
}

func bytesWithJSON(value string) *strings.Reader {
	return strings.NewReader(value)
}
