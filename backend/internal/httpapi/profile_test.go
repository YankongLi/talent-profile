package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/YankongLi/talent-profile/backend/internal/auth"
	profiledomain "github.com/YankongLi/talent-profile/backend/internal/profile"
)

type profileTestStore struct {
	profiles    map[string]profiledomain.Profile
	ensureCount int
}

func newProfileTestStore() *profileTestStore {
	return &profileTestStore{profiles: make(map[string]profiledomain.Profile)}
}

func (s *profileTestStore) GetByUserID(_ context.Context, userID string) (profiledomain.Profile, error) {
	profile, ok := s.profiles[userID]
	if !ok {
		return profiledomain.Profile{}, profiledomain.ErrNotFound
	}
	return profile, nil
}

func (s *profileTestStore) EnsureDefaultByUserID(_ context.Context, userID string) (profiledomain.Profile, error) {
	s.ensureCount++
	if profile, ok := s.profiles[userID]; ok {
		return profile, nil
	}
	profile := defaultHTTPProfile(userID)
	s.profiles[userID] = profile
	return profile, nil
}

func (s *profileTestStore) UpdateByUserID(_ context.Context, userID string, input profiledomain.UpdateInput) (profiledomain.Profile, error) {
	profile, ok := s.profiles[userID]
	if !ok {
		return profiledomain.Profile{}, profiledomain.ErrNotFound
	}
	if input.Headline != nil {
		profile.Headline = *input.Headline
	}
	if input.Summary != nil {
		profile.Summary = *input.Summary
	}
	if input.TargetRoles != nil {
		profile.TargetRoles = append([]string(nil), (*input.TargetRoles)...)
	}
	if input.Visibility != nil {
		profile.Visibility = *input.Visibility
	}
	if input.TemplateID != nil {
		profile.TemplateID = *input.TemplateID
	}
	if input.Theme != nil {
		profile.Theme = *input.Theme
	}
	profile.UpdatedAt = profile.UpdatedAt.Add(time.Second)
	s.profiles[userID] = profile
	return profile, nil
}

func TestProfileGetRequiresSession(t *testing.T) {
	authService := newProfileTestAuthService(newAuthTestStore())
	profileService := profiledomain.NewService(newProfileTestStore())
	router := NewRouter(testConfig(), WithAuthService(authService), WithProfileService(profileService))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/profile", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	var body ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Error.Code != ErrCodeUnauthorized {
		t.Fatalf("error code = %q, want %q", body.Error.Code, ErrCodeUnauthorized)
	}
}

func TestProfileGetCreatesDefaultProfile(t *testing.T) {
	authStore := newAuthTestStore()
	authService := newProfileTestAuthService(authStore)
	profileStore := newProfileTestStore()
	router := NewRouter(testConfig(), WithAuthService(authService), WithProfileService(profiledomain.NewService(profileStore)))
	token := loginProfileTestUser(t, authService, "user@example.com")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/profile", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var body profileResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Profile.UserID != "user_1" {
		t.Fatalf("profile user id = %q, want user_1", body.Profile.UserID)
	}
	if body.Profile.Visibility != profiledomain.VisibilityDraft {
		t.Fatalf("visibility = %q, want %q", body.Profile.Visibility, profiledomain.VisibilityDraft)
	}
	if body.Profile.TemplateID != profiledomain.DefaultTemplateID() {
		t.Fatalf("template id = %q, want %q", body.Profile.TemplateID, profiledomain.DefaultTemplateID())
	}
	if body.Profile.TargetRoles == nil || len(body.Profile.TargetRoles) != 0 {
		t.Fatalf("target roles = %#v, want empty slice", body.Profile.TargetRoles)
	}
	if body.Profile.Theme == nil || len(body.Profile.Theme) != 0 {
		t.Fatalf("theme = %#v, want empty object", body.Profile.Theme)
	}
	if profileStore.ensureCount != 1 {
		t.Fatalf("ensure count = %d, want 1", profileStore.ensureCount)
	}
}

func TestProfilePatchUpdatesCurrentUserProfile(t *testing.T) {
	authStore := newAuthTestStore()
	authService := newProfileTestAuthService(authStore)
	profileStore := newProfileTestStore()
	router := NewRouter(testConfig(), WithAuthService(authService), WithProfileService(profiledomain.NewService(profileStore)))
	token := loginProfileTestUser(t, authService, "user@example.com")

	reqBody := bytes.NewBufferString(`{
		"headline":"  Senior Backend Engineer  ",
		"summary":"  I build product platforms.  ",
		"target_roles":[" Platform Lead ","Go Engineer"],
		"visibility":"unlisted",
		"template_id":" modern ",
		"theme":{"accent":"emerald"}
	}`)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/profile", reqBody)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var body profileResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Profile.UserID != "user_1" {
		t.Fatalf("profile user id = %q, want user_1", body.Profile.UserID)
	}
	if body.Profile.Headline != "Senior Backend Engineer" {
		t.Fatalf("headline = %q, want Senior Backend Engineer", body.Profile.Headline)
	}
	if body.Profile.Summary != "I build product platforms." {
		t.Fatalf("summary = %q, want normalized summary", body.Profile.Summary)
	}
	if got := body.Profile.TargetRoles; len(got) != 2 || got[0] != "Platform Lead" || got[1] != "Go Engineer" {
		t.Fatalf("target roles = %#v, want normalized roles", got)
	}
	if body.Profile.Visibility != profiledomain.VisibilityUnlisted {
		t.Fatalf("visibility = %q, want %q", body.Profile.Visibility, profiledomain.VisibilityUnlisted)
	}
	if body.Profile.TemplateID != "modern" {
		t.Fatalf("template id = %q, want modern", body.Profile.TemplateID)
	}
	if got := body.Profile.Theme["accent"]; got != "emerald" {
		t.Fatalf("theme accent = %#v, want emerald", got)
	}
}

func TestProfilePatchRejectsInvalidBody(t *testing.T) {
	authStore := newAuthTestStore()
	authService := newProfileTestAuthService(authStore)
	profileStore := newProfileTestStore()
	router := NewRouter(testConfig(), WithAuthService(authService), WithProfileService(profiledomain.NewService(profileStore)))
	token := loginProfileTestUser(t, authService, "user@example.com")

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/profile", bytes.NewBufferString(`{"visibility":"private"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	var body ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Error.Code != ErrCodeBadRequest {
		t.Fatalf("error code = %q, want %q", body.Error.Code, ErrCodeBadRequest)
	}
}

func newProfileTestAuthService(store *authTestStore) *auth.Service {
	return auth.NewService(store, authTestSender{}, auth.Config{
		CodeTTL:         time.Minute,
		SessionTTL:      time.Hour,
		ExposeDebugCode: true,
	})
}

func loginProfileTestUser(t *testing.T, service *auth.Service, email string) string {
	t.Helper()
	code, err := service.RequestEmailCode(context.Background(), email)
	if err != nil {
		t.Fatalf("request email code: %v", err)
	}
	result, err := service.VerifyEmailCode(context.Background(), code.Email, code.DebugCode)
	if err != nil {
		t.Fatalf("verify email code: %v", err)
	}
	if result.Token == "" {
		t.Fatal("token is empty")
	}
	return result.Token
}

func defaultHTTPProfile(userID string) profiledomain.Profile {
	now := time.Unix(1_700_000_000, 0).UTC()
	return profiledomain.Profile{
		ID:          "profile_" + userID,
		UserID:      userID,
		TargetRoles: []string{},
		Visibility:  profiledomain.VisibilityDraft,
		TemplateID:  profiledomain.DefaultTemplateID(),
		Theme:       map[string]any{},
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}
