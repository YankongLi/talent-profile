package profile

import (
	"context"
	"errors"
	"testing"
	"time"
)

type testStore struct {
	profiles    map[string]Profile
	ensureCount int
	updateInput UpdateInput
}

func newTestStore() *testStore {
	return &testStore{profiles: make(map[string]Profile)}
}

func (s *testStore) GetByUserID(_ context.Context, userID string) (Profile, error) {
	profile, ok := s.profiles[userID]
	if !ok {
		return Profile{}, ErrNotFound
	}
	return profile, nil
}

func (s *testStore) EnsureDefaultByUserID(_ context.Context, userID string) (Profile, error) {
	s.ensureCount++
	profile := defaultTestProfile(userID)
	if existing, ok := s.profiles[userID]; ok {
		return existing, nil
	}
	s.profiles[userID] = profile
	return profile, nil
}

func (s *testStore) UpdateByUserID(_ context.Context, userID string, input UpdateInput) (Profile, error) {
	profile, ok := s.profiles[userID]
	if !ok {
		return Profile{}, ErrNotFound
	}
	s.updateInput = input
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

func TestServiceGetEnsuresDefaultProfile(t *testing.T) {
	store := newTestStore()
	service := NewService(store)

	profile, err := service.Get(context.Background(), "user_1")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if profile.UserID != "user_1" {
		t.Fatalf("profile user id = %q, want user_1", profile.UserID)
	}
	if profile.Visibility != VisibilityDraft {
		t.Fatalf("visibility = %q, want %q", profile.Visibility, VisibilityDraft)
	}
	if profile.TemplateID != DefaultTemplateID() {
		t.Fatalf("template id = %q, want %q", profile.TemplateID, DefaultTemplateID())
	}
	if len(profile.TargetRoles) != 0 {
		t.Fatalf("target roles = %#v, want empty", profile.TargetRoles)
	}
	if store.ensureCount != 1 {
		t.Fatalf("ensure count = %d, want 1", store.ensureCount)
	}
}

func TestServiceUpdateEnsuresDefaultAndNormalizesInput(t *testing.T) {
	store := newTestStore()
	service := NewService(store)
	headline := "  Backend Engineer  "
	summary := "  Builds APIs.  "
	targetRoles := []string{" Platform Engineer ", "Go Developer"}
	visibility := VisibilityUnlisted
	templateID := " modern "
	theme := map[string]any{"accent": "blue"}

	profile, err := service.Update(context.Background(), "user_1", UpdateInput{
		Headline:    &headline,
		Summary:     &summary,
		TargetRoles: &targetRoles,
		Visibility:  &visibility,
		TemplateID:  &templateID,
		Theme:       &theme,
	})
	if err != nil {
		t.Fatalf("Update returned error: %v", err)
	}
	if profile.Headline != "Backend Engineer" {
		t.Fatalf("headline = %q, want Backend Engineer", profile.Headline)
	}
	if profile.Summary != "Builds APIs." {
		t.Fatalf("summary = %q, want Builds APIs.", profile.Summary)
	}
	if got := profile.TargetRoles; len(got) != 2 || got[0] != "Platform Engineer" || got[1] != "Go Developer" {
		t.Fatalf("target roles = %#v, want normalized roles", got)
	}
	if profile.Visibility != VisibilityUnlisted {
		t.Fatalf("visibility = %q, want %q", profile.Visibility, VisibilityUnlisted)
	}
	if profile.TemplateID != "modern" {
		t.Fatalf("template id = %q, want modern", profile.TemplateID)
	}
	if store.ensureCount != 1 {
		t.Fatalf("ensure count = %d, want 1", store.ensureCount)
	}
	if store.updateInput.Headline == nil || *store.updateInput.Headline != "Backend Engineer" {
		t.Fatalf("stored headline input = %#v, want normalized", store.updateInput.Headline)
	}
}

func TestServiceUpdateWithoutChangesReturnsCurrentProfile(t *testing.T) {
	store := newTestStore()
	store.profiles["user_1"] = defaultTestProfile("user_1")
	service := NewService(store)

	profile, err := service.Update(context.Background(), "user_1", UpdateInput{})
	if err != nil {
		t.Fatalf("Update returned error: %v", err)
	}
	if profile.UserID != "user_1" {
		t.Fatalf("profile user id = %q, want user_1", profile.UserID)
	}
	if store.ensureCount != 0 {
		t.Fatalf("ensure count = %d, want 0", store.ensureCount)
	}
}

func TestServiceUpdateRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name string
		in   UpdateInput
		want error
	}{
		{
			name: "duplicate roles",
			in: func() UpdateInput {
				roles := []string{"Engineer", " engineer "}
				return UpdateInput{TargetRoles: &roles}
			}(),
			want: ErrInvalidTargetRole,
		},
		{
			name: "invalid visibility",
			in: func() UpdateInput {
				visibility := "private"
				return UpdateInput{Visibility: &visibility}
			}(),
			want: ErrInvalidVisibility,
		},
		{
			name: "blank template id",
			in: func() UpdateInput {
				templateID := "   "
				return UpdateInput{TemplateID: &templateID}
			}(),
			want: ErrInvalidTemplateID,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewService(newTestStore())

			_, err := service.Update(context.Background(), "user_1", tt.in)

			if !errors.Is(err, tt.want) {
				t.Fatalf("error = %v, want %v", err, tt.want)
			}
		})
	}
}

func defaultTestProfile(userID string) Profile {
	now := time.Unix(1_700_000_000, 0).UTC()
	return Profile{
		ID:          "profile_" + userID,
		UserID:      userID,
		TargetRoles: []string{},
		Visibility:  VisibilityDraft,
		TemplateID:  DefaultTemplateID(),
		Theme:       map[string]any{},
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}
