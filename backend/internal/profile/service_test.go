package profile

import (
	"context"
	"errors"
	"testing"
	"time"
)

type testStore struct {
	profiles    map[string]Profile
	sections    map[string]Section
	ensureCount int
	updateInput UpdateInput
	nextSection int
}

func newTestStore() *testStore {
	return &testStore{
		profiles: make(map[string]Profile),
		sections: make(map[string]Section),
	}
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

func (s *testStore) ListSectionsByUserID(_ context.Context, userID string) ([]Section, error) {
	profile, ok := s.profiles[userID]
	if !ok {
		return nil, ErrNotFound
	}
	sections := []Section{}
	for _, section := range s.sections {
		if section.ProfileID == profile.ID {
			sections = append(sections, section)
		}
	}
	return sections, nil
}

func (s *testStore) CreateSection(_ context.Context, profileID string, input CreateSectionInput) (Section, error) {
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
	section := defaultTestSection(profileID, "section_"+string(rune('0'+s.nextSection)))
	section.SectionType = input.SectionType
	section.Content = input.Content
	section.SortOrder = sortOrder
	section.IsVisible = isVisible
	section.IsUserConfirmed = isUserConfirmed
	s.sections[section.ID] = section
	return section, nil
}

func (s *testStore) GetSectionByUserID(_ context.Context, userID string, sectionID string) (Section, error) {
	profile, ok := s.profiles[userID]
	if !ok {
		return Section{}, ErrNotFound
	}
	section, ok := s.sections[sectionID]
	if !ok || section.ProfileID != profile.ID {
		return Section{}, ErrNotFound
	}
	return section, nil
}

func (s *testStore) UpdateSectionByUserID(_ context.Context, userID string, sectionID string, input UpdateSectionInput) (Section, error) {
	section, err := s.GetSectionByUserID(context.Background(), userID, sectionID)
	if err != nil {
		return Section{}, err
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

func (s *testStore) DeleteSectionByUserID(_ context.Context, userID string, sectionID string) error {
	section, err := s.GetSectionByUserID(context.Background(), userID, sectionID)
	if err != nil {
		return err
	}
	delete(s.sections, section.ID)
	return nil
}

func (s *testStore) ReorderSectionsByUserID(_ context.Context, userID string, sectionIDs []string) ([]Section, error) {
	profile, ok := s.profiles[userID]
	if !ok {
		return nil, ErrNotFound
	}
	userSections := make(map[string]Section)
	for _, section := range s.sections {
		if section.ProfileID == profile.ID {
			userSections[section.ID] = section
		}
	}
	if len(userSections) != len(sectionIDs) {
		return nil, ErrNotFound
	}
	sections := make([]Section, 0, len(sectionIDs))
	for index, sectionID := range sectionIDs {
		section, ok := userSections[sectionID]
		if !ok {
			return nil, ErrNotFound
		}
		section.SortOrder = index
		section.UpdatedAt = section.UpdatedAt.Add(time.Second)
		s.sections[section.ID] = section
		sections = append(sections, section)
	}
	return sections, nil
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

func TestServiceSectionsEnsuresDefaultAndListsSections(t *testing.T) {
	store := newTestStore()
	service := NewService(store)

	sections, err := service.Sections(context.Background(), "user_1")
	if err != nil {
		t.Fatalf("Sections returned error: %v", err)
	}
	if len(sections) != 0 {
		t.Fatalf("sections = %#v, want empty", sections)
	}
	if store.ensureCount != 1 {
		t.Fatalf("ensure count = %d, want 1", store.ensureCount)
	}
}

func TestServiceCreateSectionNormalizesAndDefaults(t *testing.T) {
	store := newTestStore()
	service := NewService(store)
	sortOrder := 7
	confirmed := true

	section, err := service.CreateSection(context.Background(), "user_1", CreateSectionInput{
		SectionType:     " experience ",
		Content:         map[string]any{"title": "API Platform"},
		SortOrder:       &sortOrder,
		IsUserConfirmed: &confirmed,
	})
	if err != nil {
		t.Fatalf("CreateSection returned error: %v", err)
	}
	if section.SectionType != "experience" {
		t.Fatalf("section type = %q, want experience", section.SectionType)
	}
	if section.SortOrder != 7 {
		t.Fatalf("sort order = %d, want 7", section.SortOrder)
	}
	if !section.IsVisible {
		t.Fatal("section should default to visible")
	}
	if !section.IsUserConfirmed {
		t.Fatal("section should be user confirmed")
	}
	if got := section.Content["title"]; got != "API Platform" {
		t.Fatalf("content title = %#v, want API Platform", got)
	}
}

func TestServiceUpdateSectionCanHideSection(t *testing.T) {
	store := newTestStore()
	store.profiles["user_1"] = defaultTestProfile("user_1")
	section := defaultTestSection("profile_user_1", "section_1")
	store.sections[section.ID] = section
	service := NewService(store)
	visible := false

	updated, err := service.UpdateSection(context.Background(), "user_1", "section_1", UpdateSectionInput{
		IsVisible: &visible,
	})
	if err != nil {
		t.Fatalf("UpdateSection returned error: %v", err)
	}
	if updated.IsVisible {
		t.Fatal("section should be hidden")
	}
}

func TestServiceDeleteSectionChecksOwnership(t *testing.T) {
	store := newTestStore()
	store.profiles["user_1"] = defaultTestProfile("user_1")
	store.profiles["user_2"] = defaultTestProfile("user_2")
	section := defaultTestSection("profile_user_1", "section_1")
	store.sections[section.ID] = section
	service := NewService(store)

	err := service.DeleteSection(context.Background(), "user_2", "section_1")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("delete other user's section error = %v, want %v", err, ErrNotFound)
	}

	if err := service.DeleteSection(context.Background(), "user_1", "section_1"); err != nil {
		t.Fatalf("DeleteSection returned error: %v", err)
	}
	if _, ok := store.sections["section_1"]; ok {
		t.Fatal("section still exists after delete")
	}
}

func TestServiceReorderSectionsUpdatesSortOrder(t *testing.T) {
	store := newTestStore()
	store.profiles["user_1"] = defaultTestProfile("user_1")
	store.sections["section_1"] = defaultTestSection("profile_user_1", "section_1")
	store.sections["section_2"] = defaultTestSection("profile_user_1", "section_2")
	service := NewService(store)

	sections, err := service.ReorderSections(context.Background(), "user_1", []string{" section_2 ", "section_1"})
	if err != nil {
		t.Fatalf("ReorderSections returned error: %v", err)
	}
	if len(sections) != 2 {
		t.Fatalf("sections len = %d, want 2", len(sections))
	}
	if sections[0].ID != "section_2" || sections[0].SortOrder != 0 {
		t.Fatalf("first section = %#v, want section_2 sort 0", sections[0])
	}
	if sections[1].ID != "section_1" || sections[1].SortOrder != 1 {
		t.Fatalf("second section = %#v, want section_1 sort 1", sections[1])
	}
}

func TestServiceReorderSectionsRequiresCurrentUserFullSet(t *testing.T) {
	store := newTestStore()
	store.profiles["user_1"] = defaultTestProfile("user_1")
	store.profiles["user_2"] = defaultTestProfile("user_2")
	store.sections["section_1"] = defaultTestSection("profile_user_1", "section_1")
	store.sections["section_2"] = defaultTestSection("profile_user_2", "section_2")
	service := NewService(store)

	_, err := service.ReorderSections(context.Background(), "user_1", []string{"section_2"})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("reorder other user's section error = %v, want %v", err, ErrNotFound)
	}

	_, err = service.ReorderSections(context.Background(), "user_1", []string{"section_1", "section_2"})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("reorder mixed sections error = %v, want %v", err, ErrNotFound)
	}
}

func TestServiceSectionRejectsInvalidInput(t *testing.T) {
	service := NewService(newTestStore())
	_, err := service.CreateSection(context.Background(), "user_1", CreateSectionInput{SectionType: "   "})
	if !errors.Is(err, ErrInvalidSection) {
		t.Fatalf("create error = %v, want %v", err, ErrInvalidSection)
	}

	sortOrder := -1
	_, err = service.UpdateSection(context.Background(), "user_1", "section_1", UpdateSectionInput{SortOrder: &sortOrder})
	if !errors.Is(err, ErrInvalidSortOrder) {
		t.Fatalf("update error = %v, want %v", err, ErrInvalidSortOrder)
	}

	_, err = service.ReorderSections(context.Background(), "user_1", []string{"section_1", " section_1 "})
	if !errors.Is(err, ErrInvalidSectionID) {
		t.Fatalf("reorder duplicate error = %v, want %v", err, ErrInvalidSectionID)
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

func defaultTestSection(profileID string, sectionID string) Section {
	now := time.Unix(1_700_000_000, 0).UTC()
	return Section{
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
