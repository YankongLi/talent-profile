package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/YankongLi/talent-profile/backend/internal/auth"
	profiledomain "github.com/YankongLi/talent-profile/backend/internal/profile"
)

type profileTestStore struct {
	profiles    map[string]profiledomain.Profile
	sections    map[string]profiledomain.Section
	ensureCount int
	nextSection int
}

func newProfileTestStore() *profileTestStore {
	return &profileTestStore{
		profiles: make(map[string]profiledomain.Profile),
		sections: make(map[string]profiledomain.Section),
	}
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

func (s *profileTestStore) ListSectionsByUserID(_ context.Context, userID string) ([]profiledomain.Section, error) {
	profile, ok := s.profiles[userID]
	if !ok {
		return nil, profiledomain.ErrNotFound
	}
	sections := []profiledomain.Section{}
	for _, section := range s.sections {
		if section.ProfileID == profile.ID {
			sections = append(sections, section)
		}
	}
	return sections, nil
}

func (s *profileTestStore) CreateSection(_ context.Context, profileID string, input profiledomain.CreateSectionInput) (profiledomain.Section, error) {
	s.nextSection++
	sortOrder := 0
	if input.SortOrder != nil {
		sortOrder = *input.SortOrder
	}
	isVisible := true
	if input.IsVisible != nil {
		isVisible = *input.IsVisible
	}
	isUserConfirmed := false
	if input.IsUserConfirmed != nil {
		isUserConfirmed = *input.IsUserConfirmed
	}
	section := defaultHTTPSection(profileID, fmt.Sprintf("section_%d", s.nextSection))
	section.SectionType = input.SectionType
	section.Content = input.Content
	section.SortOrder = sortOrder
	section.IsVisible = isVisible
	section.IsUserConfirmed = isUserConfirmed
	s.sections[section.ID] = section
	return section, nil
}

func (s *profileTestStore) GetSectionByUserID(_ context.Context, userID string, sectionID string) (profiledomain.Section, error) {
	profile, ok := s.profiles[userID]
	if !ok {
		return profiledomain.Section{}, profiledomain.ErrNotFound
	}
	section, ok := s.sections[sectionID]
	if !ok || section.ProfileID != profile.ID {
		return profiledomain.Section{}, profiledomain.ErrNotFound
	}
	return section, nil
}

func (s *profileTestStore) UpdateSectionByUserID(_ context.Context, userID string, sectionID string, input profiledomain.UpdateSectionInput) (profiledomain.Section, error) {
	section, err := s.GetSectionByUserID(context.Background(), userID, sectionID)
	if err != nil {
		return profiledomain.Section{}, err
	}
	if input.SectionType != nil {
		section.SectionType = *input.SectionType
	}
	if input.Content != nil {
		section.Content = *input.Content
	}
	if input.SortOrder != nil {
		section.SortOrder = *input.SortOrder
	}
	if input.IsVisible != nil {
		section.IsVisible = *input.IsVisible
	}
	if input.IsUserConfirmed != nil {
		section.IsUserConfirmed = *input.IsUserConfirmed
	}
	section.UpdatedAt = section.UpdatedAt.Add(time.Second)
	s.sections[section.ID] = section
	return section, nil
}

func (s *profileTestStore) DeleteSectionByUserID(_ context.Context, userID string, sectionID string) error {
	section, err := s.GetSectionByUserID(context.Background(), userID, sectionID)
	if err != nil {
		return err
	}
	delete(s.sections, section.ID)
	return nil
}

func (s *profileTestStore) ReorderSectionsByUserID(_ context.Context, userID string, sectionIDs []string) ([]profiledomain.Section, error) {
	profile, ok := s.profiles[userID]
	if !ok {
		return nil, profiledomain.ErrNotFound
	}
	userSections := make(map[string]profiledomain.Section)
	for _, section := range s.sections {
		if section.ProfileID == profile.ID {
			userSections[section.ID] = section
		}
	}
	if len(userSections) != len(sectionIDs) {
		return nil, profiledomain.ErrNotFound
	}
	sections := make([]profiledomain.Section, 0, len(sectionIDs))
	for index, sectionID := range sectionIDs {
		section, ok := userSections[sectionID]
		if !ok {
			return nil, profiledomain.ErrNotFound
		}
		section.SortOrder = index
		section.UpdatedAt = section.UpdatedAt.Add(time.Second)
		s.sections[section.ID] = section
		sections = append(sections, section)
	}
	return sections, nil
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
	if body.Sections == nil || len(body.Sections) != 0 {
		t.Fatalf("sections = %#v, want empty slice", body.Sections)
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

func TestProfileSectionRoutesCreateHideAndDeleteSection(t *testing.T) {
	authStore := newAuthTestStore()
	authService := newProfileTestAuthService(authStore)
	profileStore := newProfileTestStore()
	router := NewRouter(testConfig(), WithAuthService(authService), WithProfileService(profiledomain.NewService(profileStore)))
	token := loginProfileTestUser(t, authService, "user@example.com")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/profile/sections", bytes.NewBufferString(`{
		"section_type":" experience ",
		"content":{"title":"API Platform"},
		"sort_order":2,
		"is_user_confirmed":true
	}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want %d, body %s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	var createBody sectionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &createBody); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if createBody.Section.SectionType != "experience" {
		t.Fatalf("section type = %q, want experience", createBody.Section.SectionType)
	}
	if !createBody.Section.IsVisible {
		t.Fatal("section should default visible")
	}
	if !createBody.Section.IsUserConfirmed {
		t.Fatal("section should be confirmed")
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/profile", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("get status = %d, want %d, body %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var profileBody profileResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &profileBody); err != nil {
		t.Fatalf("decode get response: %v", err)
	}
	if len(profileBody.Sections) != 1 || profileBody.Sections[0].ID != createBody.Section.ID {
		t.Fatalf("sections = %#v, want created section", profileBody.Sections)
	}

	req = httptest.NewRequest(http.MethodPatch, "/api/v1/profile/sections/"+createBody.Section.ID, bytes.NewBufferString(`{"is_visible":false}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("patch status = %d, want %d, body %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var patchBody sectionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &patchBody); err != nil {
		t.Fatalf("decode patch response: %v", err)
	}
	if patchBody.Section.IsVisible {
		t.Fatal("section should be hidden")
	}

	req = httptest.NewRequest(http.MethodDelete, "/api/v1/profile/sections/"+createBody.Section.ID, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("delete status = %d, want %d, body %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if _, ok := profileStore.sections[createBody.Section.ID]; ok {
		t.Fatal("section still exists after delete")
	}
}

func TestProfileSectionRoutesReorderSections(t *testing.T) {
	authStore := newAuthTestStore()
	authService := newProfileTestAuthService(authStore)
	profileStore := newProfileTestStore()
	router := NewRouter(testConfig(), WithAuthService(authService), WithProfileService(profiledomain.NewService(profileStore)))
	token := loginProfileTestUser(t, authService, "user@example.com")

	createSection := func(sectionType string, sortOrder int) string {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/profile/sections", bytes.NewBufferString(fmt.Sprintf(`{
			"section_type":%q,
			"content":{"title":%q},
			"sort_order":%d
		}`, sectionType, sectionType, sortOrder)))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("create status = %d, want %d, body %s", rec.Code, http.StatusCreated, rec.Body.String())
		}
		var body sectionResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode create response: %v", err)
		}
		return body.Section.ID
	}

	firstID := createSection("experience", 0)
	secondID := createSection("project", 1)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/profile/sections/reorder", bytes.NewBufferString(fmt.Sprintf(`{
		"section_ids":[%q,%q]
	}`, secondID, firstID)))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("reorder status = %d, want %d, body %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var body reorderSectionsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode reorder response: %v", err)
	}
	if len(body.Sections) != 2 {
		t.Fatalf("sections len = %d, want 2", len(body.Sections))
	}
	if body.Sections[0].ID != secondID || body.Sections[0].SortOrder != 0 {
		t.Fatalf("first reordered section = %#v, want second section sort 0", body.Sections[0])
	}
	if body.Sections[1].ID != firstID || body.Sections[1].SortOrder != 1 {
		t.Fatalf("second reordered section = %#v, want first section sort 1", body.Sections[1])
	}
}

func TestProfileSectionRoutesRejectInvalidReorder(t *testing.T) {
	authStore := newAuthTestStore()
	authService := newProfileTestAuthService(authStore)
	profileStore := newProfileTestStore()
	router := NewRouter(testConfig(), WithAuthService(authService), WithProfileService(profiledomain.NewService(profileStore)))
	token := loginProfileTestUser(t, authService, "user@example.com")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/profile/sections/reorder", bytes.NewBufferString(`{"section_ids":["section_1","section_1"]}`))
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

func TestProfileSectionRoutesRejectInvalidSection(t *testing.T) {
	authStore := newAuthTestStore()
	authService := newProfileTestAuthService(authStore)
	profileStore := newProfileTestStore()
	router := NewRouter(testConfig(), WithAuthService(authService), WithProfileService(profiledomain.NewService(profileStore)))
	token := loginProfileTestUser(t, authService, "user@example.com")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/profile/sections", bytes.NewBufferString(`{"section_type":"   "}`))
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

func defaultHTTPSection(profileID string, sectionID string) profiledomain.Section {
	now := time.Unix(1_700_000_000, 0).UTC()
	return profiledomain.Section{
		ID:              sectionID,
		ProfileID:       profileID,
		SectionType:     "experience",
		Content:         map[string]any{},
		IsVisible:       true,
		IsUserConfirmed: false,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
}
