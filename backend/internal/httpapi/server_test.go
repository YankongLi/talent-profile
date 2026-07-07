package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/YankongLi/talent-profile/backend/internal/config"
	"github.com/gin-gonic/gin"
)

func TestHealthRoutes(t *testing.T) {
	router := NewRouter(testConfig())

	for _, path := range []string{"/healthz", "/readyz", "/api/v1/health"} {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
			}

			var body HealthResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if body.Status != "ok" {
				t.Fatalf("status body = %q, want ok", body.Status)
			}
			if body.App != "talentpage-api-test" {
				t.Fatalf("app body = %q", body.App)
			}
		})
	}
}

func TestSecurityHeaders(t *testing.T) {
	router := NewRouter(testConfig())
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("X-Content-Type-Options = %q, want nosniff", got)
	}
	if got := rec.Header().Get("X-Frame-Options"); got != "DENY" {
		t.Fatalf("X-Frame-Options = %q, want DENY", got)
	}
	if got := rec.Header().Get("Referrer-Policy"); got != "strict-origin-when-cross-origin" {
		t.Fatalf("Referrer-Policy = %q, want strict-origin-when-cross-origin", got)
	}
}

func TestV1RouteContractsReturnNotImplemented(t *testing.T) {
	router := NewRouter(testConfig())

	tests := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/v1/auth/email-code"},
		{http.MethodPost, "/api/v1/auth/verify"},
		{http.MethodPost, "/api/v1/auth/logout"},
		{http.MethodDelete, "/api/v1/account"},
		{http.MethodPost, "/api/v1/resumes"},
		{http.MethodGet, "/api/v1/resumes/resume_123/status"},
		{http.MethodDelete, "/api/v1/resumes/resume_123"},
		{http.MethodPost, "/api/v1/resumes/resume_123/generate-profile"},
		{http.MethodGet, "/api/v1/profile"},
		{http.MethodPatch, "/api/v1/profile"},
		{http.MethodPost, "/api/v1/profile/sections"},
		{http.MethodPatch, "/api/v1/profile/sections/section_123"},
		{http.MethodDelete, "/api/v1/profile/sections/section_123"},
		{http.MethodPost, "/api/v1/profile/sections/reorder"},
		{http.MethodPost, "/api/v1/profile/sections/section_123/rewrite"},
		{http.MethodGet, "/api/v1/domains/check?slug=zhangsan"},
		{http.MethodPut, "/api/v1/profile/domain"},
		{http.MethodPost, "/api/v1/profile/publish"},
		{http.MethodPost, "/api/v1/profile/unpublish"},
		{http.MethodGet, "/api/v1/profile/preview"},
		{http.MethodGet, "/api/v1/profile/analytics?range=7d"},
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			if rec.Code != http.StatusNotImplemented {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotImplemented)
			}

			var body ErrorResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if body.Error.Code != ErrCodeNotImplemented {
				t.Fatalf("error code = %q, want %q", body.Error.Code, ErrCodeNotImplemented)
			}
			if body.Error.RequestID == "" {
				t.Fatal("request id is empty")
			}
		})
	}
}

func TestNoRouteUsesJSONError(t *testing.T) {
	router := NewRouter(testConfig())
	req := httptest.NewRequest(http.MethodGet, "/missing", nil)
	req.Header.Set("X-Request-ID", "req_123")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}

	var body ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Error.Code != ErrCodeNotFound {
		t.Fatalf("error code = %q, want %q", body.Error.Code, ErrCodeNotFound)
	}
	if body.Error.RequestID != "req_123" {
		t.Fatalf("request id = %q, want req_123", body.Error.RequestID)
	}
}

func TestNoMethodUsesJSONError(t *testing.T) {
	router := NewRouter(testConfig())
	req := httptest.NewRequest(http.MethodPost, "/healthz", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}

	var body ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Error.Code != ErrCodeMethodDenied {
		t.Fatalf("error code = %q, want %q", body.Error.Code, ErrCodeMethodDenied)
	}
}

func TestPanicUsesJSONError(t *testing.T) {
	router := NewRouter(testConfig())
	router.GET("/panic", func(_ *gin.Context) {
		panic("boom")
	})

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
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
	if body.Error.RequestID == "" {
		t.Fatal("request id is empty")
	}
}

func testConfig() config.Config {
	return config.Config{
		AppName: "talentpage-api-test",
		Env:     config.EnvTest,
		HTTP: config.HTTPConfig{
			Addr:            "127.0.0.1:0",
			ReadTimeout:     time.Second,
			WriteTimeout:    time.Second,
			ShutdownTimeout: time.Second,
		},
		Auth: config.AuthConfig{
			CodeTTL:    time.Minute,
			SessionTTL: time.Hour,
			CookieName: "talentpage_session",
		},
	}
}
