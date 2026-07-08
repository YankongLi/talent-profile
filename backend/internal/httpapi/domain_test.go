package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/YankongLi/talent-profile/backend/internal/publishing"
)

type domainTestStore struct {
	slugs map[string]bool
	err   error
}

func (s domainTestStore) SlugExists(_ context.Context, slug string) (bool, error) {
	if s.err != nil {
		return false, s.err
	}
	return s.slugs[slug], nil
}

func TestDomainCheckReturnsAvailability(t *testing.T) {
	router := NewRouter(
		testConfig(),
		WithPublishingService(publishing.NewService(domainTestStore{
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
		WithPublishingService(publishing.NewService(domainTestStore{
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
