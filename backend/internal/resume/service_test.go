package resume

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/YankongLi/talent-profile/backend/internal/storage"
)

type resumeTestStore struct {
	createInput CreateInput
	createErr   error
	resume      Resume
	deletedID   string
}

func (s *resumeTestStore) Create(_ context.Context, input CreateInput) (Resume, error) {
	s.createInput = input
	if s.createErr != nil {
		return Resume{}, s.createErr
	}
	return Resume{
		ID:               "resume_1",
		UserID:           input.UserID,
		StorageKey:       input.StorageKey,
		OriginalFilename: input.OriginalFilename,
		MimeType:         input.MimeType,
		FileSize:         input.FileSize,
		ContentHash:      input.ContentHash,
		ParseStatus:      input.ParseStatus,
		CreatedAt:        time.Unix(1_700_000_000, 0).UTC(),
	}, nil
}

func (s *resumeTestStore) GetByUserID(_ context.Context, userID string, resumeID string) (Resume, error) {
	if s.resume.ID == "" || s.resume.ID != resumeID || s.resume.UserID != userID {
		return Resume{}, ErrNotFound
	}
	return s.resume, nil
}

func (s *resumeTestStore) SoftDeleteByUserID(_ context.Context, userID string, resumeID string, _ time.Time) error {
	if s.resume.ID == "" || s.resume.ID != resumeID || s.resume.UserID != userID {
		return ErrNotFound
	}
	s.deletedID = resumeID
	return nil
}

type resumeTestObjects struct {
	putInput   storage.PutObjectInput
	putContent []byte
	deletedKey string
}

func (s *resumeTestObjects) Put(_ context.Context, input storage.PutObjectInput) (storage.ObjectInfo, error) {
	s.putInput = input
	content, err := io.ReadAll(input.Reader)
	if err != nil {
		return storage.ObjectInfo{}, err
	}
	s.putContent = content
	return storage.ObjectInfo{Key: input.Key, Size: input.Size, ContentType: input.ContentType}, nil
}

func (s *resumeTestObjects) Get(_ context.Context, _ string) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(s.putContent)), nil
}

func (s *resumeTestObjects) PresignedGetURL(_ context.Context, _ string, _ time.Duration) (*url.URL, error) {
	return nil, nil
}

func (s *resumeTestObjects) Delete(_ context.Context, key string) error {
	s.deletedKey = key
	return nil
}

type resumeTestEnqueuer struct {
	resumeID string
	err      error
}

func (e *resumeTestEnqueuer) EnqueueTextExtraction(_ context.Context, resumeID string) error {
	e.resumeID = resumeID
	return e.err
}

func TestServiceUploadPDF(t *testing.T) {
	store := &resumeTestStore{}
	objects := &resumeTestObjects{}
	service := NewService(store, objects)

	content := []byte("%PDF-1.7\nresume body")
	resume, err := service.Upload(context.Background(), "user_1", UploadInput{
		OriginalFilename: "Jane Resume.PDF",
		ContentType:      MimePDF,
		Reader:           bytes.NewReader(content),
	})
	if err != nil {
		t.Fatalf("Upload() error = %v", err)
	}

	if resume.ParseStatus != StatusUploaded {
		t.Fatalf("ParseStatus = %q, want %q", resume.ParseStatus, StatusUploaded)
	}
	if store.createInput.MimeType != MimePDF {
		t.Fatalf("stored mime = %q, want %q", store.createInput.MimeType, MimePDF)
	}
	if store.createInput.FileSize != int64(len(content)) {
		t.Fatalf("stored size = %d, want %d", store.createInput.FileSize, len(content))
	}
	if len(store.createInput.ContentHash) != 64 {
		t.Fatalf("content hash = %q, want sha256 hex", store.createInput.ContentHash)
	}
	if !strings.HasPrefix(store.createInput.StorageKey, "resumes/user_1/") {
		t.Fatalf("storage key = %q, want user resume prefix", store.createInput.StorageKey)
	}
	if strings.Contains(strings.ToLower(store.createInput.StorageKey), "jane") {
		t.Fatalf("storage key = %q contains original filename data", store.createInput.StorageKey)
	}
	if !bytes.Equal(objects.putContent, content) {
		t.Fatal("stored object content does not match upload")
	}
}

func TestServiceUploadEnqueuesTextExtraction(t *testing.T) {
	enqueuer := &resumeTestEnqueuer{}
	service := NewServiceWithQueue(&resumeTestStore{}, &resumeTestObjects{}, enqueuer)

	resume, err := service.Upload(context.Background(), "user_1", UploadInput{
		OriginalFilename: "resume.pdf",
		ContentType:      MimePDF,
		Reader:           bytes.NewReader([]byte("%PDF-1.7\nbody")),
	})
	if err != nil {
		t.Fatalf("Upload() error = %v", err)
	}
	if enqueuer.resumeID != resume.ID {
		t.Fatalf("enqueued resume id = %q, want %q", enqueuer.resumeID, resume.ID)
	}
}

func TestServiceUploadDOCX(t *testing.T) {
	service := NewService(&resumeTestStore{}, &resumeTestObjects{})

	content := testDOCX(t)
	resume, err := service.Upload(context.Background(), "user_1", UploadInput{
		OriginalFilename: "resume.docx",
		ContentType:      MimeDOCX,
		Reader:           bytes.NewReader(content),
	})
	if err != nil {
		t.Fatalf("Upload() error = %v", err)
	}
	if resume.MimeType != MimeDOCX {
		t.Fatalf("MimeType = %q, want %q", resume.MimeType, MimeDOCX)
	}
}

func TestServiceUploadRejectsInvalidFiles(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		mimeType string
		content  []byte
		wantErr  error
	}{
		{
			name:     "unsupported extension",
			filename: "resume.txt",
			mimeType: "text/plain",
			content:  []byte("hello"),
			wantErr:  ErrUnsupportedFileType,
		},
		{
			name:     "pdf wrong mime",
			filename: "resume.pdf",
			mimeType: "application/octet-stream",
			content:  []byte("%PDF-1.7\n"),
			wantErr:  ErrUnsupportedFileType,
		},
		{
			name:     "pdf wrong header",
			filename: "resume.pdf",
			mimeType: MimePDF,
			content:  []byte("not a pdf"),
			wantErr:  ErrUnsupportedFileType,
		},
		{
			name:     "docx wrong header",
			filename: "resume.docx",
			mimeType: MimeDOCX,
			content:  []byte("not a zip"),
			wantErr:  ErrInvalidFileHeader,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewService(&resumeTestStore{}, &resumeTestObjects{})
			_, err := service.Upload(context.Background(), "user_1", UploadInput{
				OriginalFilename: tt.filename,
				ContentType:      tt.mimeType,
				Reader:           bytes.NewReader(tt.content),
			})
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Upload() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestServiceUploadRejectsOversizedFile(t *testing.T) {
	service := NewService(&resumeTestStore{}, &resumeTestObjects{})

	_, err := service.Upload(context.Background(), "user_1", UploadInput{
		OriginalFilename: "resume.pdf",
		ContentType:      MimePDF,
		Reader:           bytes.NewReader(make([]byte, MaxUploadBytes+1)),
	})
	if !errors.Is(err, ErrFileTooLarge) {
		t.Fatalf("Upload() error = %v, want %v", err, ErrFileTooLarge)
	}
}

func TestServiceUploadDeletesObjectWhenCreateFails(t *testing.T) {
	store := &resumeTestStore{createErr: errors.New("insert failed")}
	objects := &resumeTestObjects{}
	service := NewService(store, objects)

	_, err := service.Upload(context.Background(), "user_1", UploadInput{
		OriginalFilename: "resume.pdf",
		ContentType:      MimePDF,
		Reader:           bytes.NewReader([]byte("%PDF-1.7\n")),
	})
	if err == nil {
		t.Fatal("Upload() error = nil, want error")
	}
	if objects.deletedKey == "" {
		t.Fatal("Delete() was not called")
	}
	if objects.deletedKey != store.createInput.StorageKey {
		t.Fatalf("deleted key = %q, want %q", objects.deletedKey, store.createInput.StorageKey)
	}
}

func TestServiceStatus(t *testing.T) {
	store := &resumeTestStore{
		resume: Resume{
			ID:               "resume_1",
			UserID:           "user_1",
			OriginalFilename: "resume.pdf",
			MimeType:         MimePDF,
			FileSize:         12,
			ParseStatus:      StatusFailed,
			ParseError:       "parse failed",
		},
	}
	service := NewService(store, &resumeTestObjects{})

	resume, err := service.Status(context.Background(), "user_1", "resume_1")
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if resume.ParseStatus != StatusFailed {
		t.Fatalf("ParseStatus = %q, want %q", resume.ParseStatus, StatusFailed)
	}
	if resume.ParseError != "parse failed" {
		t.Fatalf("ParseError = %q", resume.ParseError)
	}
}

func TestServiceDeleteRemovesObjectThenSoftDeletes(t *testing.T) {
	store := &resumeTestStore{
		resume: Resume{
			ID:         "resume_1",
			UserID:     "user_1",
			StorageKey: "resumes/user_1/random.pdf",
		},
	}
	objects := &resumeTestObjects{}
	service := NewService(store, objects)

	if err := service.Delete(context.Background(), "user_1", "resume_1"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if objects.deletedKey != "resumes/user_1/random.pdf" {
		t.Fatalf("deleted key = %q", objects.deletedKey)
	}
	if store.deletedID != "resume_1" {
		t.Fatalf("deleted id = %q", store.deletedID)
	}
}

func testDOCX(t *testing.T) []byte {
	t.Helper()

	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)
	files := map[string]string{
		"[Content_Types].xml": `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"></Types>`,
		"word/document.xml":   `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:r><w:t>Hello world</w:t></w:r></w:p></w:body></w:document>`,
	}
	for name, content := range files {
		file, err := writer.Create(name)
		if err != nil {
			t.Fatalf("create docx entry: %v", err)
		}
		if _, err := file.Write([]byte(content)); err != nil {
			t.Fatalf("write docx entry: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close docx: %v", err)
	}
	return buf.Bytes()
}
