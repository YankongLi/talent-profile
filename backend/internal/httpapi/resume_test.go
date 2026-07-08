package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"net/url"
	"strings"
	"testing"
	"time"

	resumedomain "github.com/YankongLi/talent-profile/backend/internal/resume"
	"github.com/YankongLi/talent-profile/backend/internal/storage"
)

type resumeHTTPTestStore struct {
	createInput resumedomain.CreateInput
	resumes     map[string]resumedomain.Resume
	deletedID   string
}

func (s *resumeHTTPTestStore) Create(_ context.Context, input resumedomain.CreateInput) (resumedomain.Resume, error) {
	s.createInput = input
	resume := resumedomain.Resume{
		ID:               "resume_1",
		UserID:           input.UserID,
		StorageKey:       input.StorageKey,
		OriginalFilename: input.OriginalFilename,
		MimeType:         input.MimeType,
		FileSize:         input.FileSize,
		ContentHash:      input.ContentHash,
		ParseStatus:      input.ParseStatus,
		CreatedAt:        time.Unix(1_700_000_000, 0).UTC(),
	}
	if s.resumes == nil {
		s.resumes = make(map[string]resumedomain.Resume)
	}
	s.resumes[resume.ID] = resume
	return resume, nil
}

func (s *resumeHTTPTestStore) GetByUserID(_ context.Context, userID string, resumeID string) (resumedomain.Resume, error) {
	resume, ok := s.resumes[resumeID]
	if !ok || resume.UserID != userID {
		return resumedomain.Resume{}, resumedomain.ErrNotFound
	}
	return resume, nil
}

func (s *resumeHTTPTestStore) SoftDeleteByUserID(_ context.Context, userID string, resumeID string, _ time.Time) error {
	resume, ok := s.resumes[resumeID]
	if !ok || resume.UserID != userID {
		return resumedomain.ErrNotFound
	}
	s.deletedID = resumeID
	delete(s.resumes, resumeID)
	return nil
}

type resumeHTTPTestObjects struct {
	putInput storage.PutObjectInput
	deleted  string
}

func (s *resumeHTTPTestObjects) Put(_ context.Context, input storage.PutObjectInput) (storage.ObjectInfo, error) {
	if _, err := io.Copy(io.Discard, input.Reader); err != nil {
		return storage.ObjectInfo{}, err
	}
	s.putInput = input
	return storage.ObjectInfo{Key: input.Key, Size: input.Size, ContentType: input.ContentType}, nil
}

func (s *resumeHTTPTestObjects) PresignedGetURL(_ context.Context, _ string, _ time.Duration) (*url.URL, error) {
	return nil, nil
}

func (s *resumeHTTPTestObjects) Delete(_ context.Context, key string) error {
	s.deleted = key
	return nil
}

func TestResumeUpload(t *testing.T) {
	authStore := newAuthTestStore()
	authService := newProfileTestAuthService(authStore)
	token := loginProfileTestUser(t, authService, "user@example.com")

	resumeStore := &resumeHTTPTestStore{}
	objects := &resumeHTTPTestObjects{}
	router := NewRouter(
		testConfig(),
		WithAuthService(authService),
		WithResumeService(resumedomain.NewService(resumeStore, objects)),
	)

	body, contentType := multipartResumeBody(t, "resume.pdf", resumedomain.MimePDF, []byte("%PDF-1.7\nbody"))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/resumes", body)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "storage_key") {
		t.Fatalf("response leaked storage key: %s", rec.Body.String())
	}

	var response resumeResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Resume.ID != "resume_1" {
		t.Fatalf("resume id = %q, want resume_1", response.Resume.ID)
	}
	if response.Resume.ParseStatus != resumedomain.StatusUploaded {
		t.Fatalf("parse status = %q, want %q", response.Resume.ParseStatus, resumedomain.StatusUploaded)
	}
	if resumeStore.createInput.UserID == "" {
		t.Fatal("stored user id is empty")
	}
	if resumeStore.createInput.MimeType != resumedomain.MimePDF {
		t.Fatalf("stored mime type = %q", resumeStore.createInput.MimeType)
	}
	if objects.putInput.Key == "" {
		t.Fatal("object was not uploaded")
	}
}

func TestResumeUploadRequiresAuth(t *testing.T) {
	router := NewRouter(
		testConfig(),
		WithAuthService(newProfileTestAuthService(newAuthTestStore())),
		WithResumeService(resumedomain.NewService(&resumeHTTPTestStore{}, &resumeHTTPTestObjects{})),
	)

	body, contentType := multipartResumeBody(t, "resume.pdf", resumedomain.MimePDF, []byte("%PDF-1.7\nbody"))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/resumes", body)
	req.Header.Set("Content-Type", contentType)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestResumeStatus(t *testing.T) {
	authStore := newAuthTestStore()
	authService := newProfileTestAuthService(authStore)
	token := loginProfileTestUser(t, authService, "user@example.com")
	userID := authStore.users["user@example.com"].ID

	resumeStore := &resumeHTTPTestStore{
		resumes: map[string]resumedomain.Resume{
			"resume_1": {
				ID:               "resume_1",
				UserID:           userID,
				OriginalFilename: "resume.pdf",
				MimeType:         resumedomain.MimePDF,
				FileSize:         10,
				ParseStatus:      resumedomain.StatusFailed,
				ParseError:       "unsupported embedded image",
			},
		},
	}
	router := NewRouter(
		testConfig(),
		WithAuthService(authService),
		WithResumeService(resumedomain.NewService(resumeStore, &resumeHTTPTestObjects{})),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/resumes/resume_1/status", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var response resumeStatusResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.ParseStatus != resumedomain.StatusFailed {
		t.Fatalf("parse status = %q", response.ParseStatus)
	}
	if response.ParseError == "" {
		t.Fatal("parse error is empty")
	}
	if strings.Contains(rec.Body.String(), "storage_key") {
		t.Fatalf("response leaked storage key: %s", rec.Body.String())
	}
}

func TestResumeDelete(t *testing.T) {
	authStore := newAuthTestStore()
	authService := newProfileTestAuthService(authStore)
	token := loginProfileTestUser(t, authService, "user@example.com")
	userID := authStore.users["user@example.com"].ID

	resumeStore := &resumeHTTPTestStore{
		resumes: map[string]resumedomain.Resume{
			"resume_1": {
				ID:         "resume_1",
				UserID:     userID,
				StorageKey: "resumes/" + userID + "/random.pdf",
			},
		},
	}
	objects := &resumeHTTPTestObjects{}
	router := NewRouter(
		testConfig(),
		WithAuthService(authService),
		WithResumeService(resumedomain.NewService(resumeStore, objects)),
	)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/resumes/resume_1", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if resumeStore.deletedID != "resume_1" {
		t.Fatalf("deleted id = %q", resumeStore.deletedID)
	}
	if objects.deleted != "resumes/"+userID+"/random.pdf" {
		t.Fatalf("deleted object = %q", objects.deleted)
	}
}

func TestResumeUploadRejectsInvalidFile(t *testing.T) {
	authStore := newAuthTestStore()
	authService := newProfileTestAuthService(authStore)
	token := loginProfileTestUser(t, authService, "user@example.com")
	router := NewRouter(
		testConfig(),
		WithAuthService(authService),
		WithResumeService(resumedomain.NewService(&resumeHTTPTestStore{}, &resumeHTTPTestObjects{})),
	)

	body, contentType := multipartResumeBody(t, "resume.pdf", resumedomain.MimePDF, []byte("not a pdf"))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/resumes", body)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	var response ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Error.Code != ErrCodeBadRequest {
		t.Fatalf("error code = %q, want %q", response.Error.Code, ErrCodeBadRequest)
	}
}

func TestResumeUploadNotImplementedWithoutService(t *testing.T) {
	router := NewRouter(testConfig())

	body, contentType := multipartResumeBody(t, "resume.pdf", resumedomain.MimePDF, []byte("%PDF-1.7\nbody"))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/resumes", body)
	req.Header.Set("Content-Type", contentType)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotImplemented)
	}
}

func multipartResumeBody(t *testing.T, filename, mimeType string, content []byte) (*bytes.Buffer, string) {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", `form-data; name="file"; filename="`+filename+`"`)
	header.Set("Content-Type", mimeType)
	part, err := writer.CreatePart(header)
	if err != nil {
		t.Fatalf("create multipart part: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("write multipart part: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}
	return &body, writer.FormDataContentType()
}
