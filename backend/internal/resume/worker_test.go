package resume

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/url"
	"testing"
	"time"

	"github.com/YankongLi/talent-profile/backend/internal/storage"
	"github.com/hibiken/asynq"
)

type workerTestStore struct {
	resume        Resume
	markedParsing bool
	parsedText    []byte
	failedReason  string
}

func (s *workerTestStore) GetByID(_ context.Context, resumeID string) (Resume, error) {
	if s.resume.ID != resumeID {
		return Resume{}, ErrNotFound
	}
	return s.resume, nil
}

func (s *workerTestStore) MarkParsing(_ context.Context, resumeID string) error {
	if s.resume.ID != resumeID {
		return ErrNotFound
	}
	s.markedParsing = true
	s.resume.ParseStatus = StatusParsing
	return nil
}

func (s *workerTestStore) MarkParsed(_ context.Context, resumeID string, extractedText []byte) error {
	if s.resume.ID != resumeID {
		return ErrNotFound
	}
	s.parsedText = extractedText
	s.resume.ParseStatus = StatusParsed
	return nil
}

func (s *workerTestStore) MarkFailed(_ context.Context, resumeID string, reason string) error {
	if s.resume.ID != resumeID {
		return ErrNotFound
	}
	s.failedReason = reason
	s.resume.ParseStatus = StatusFailed
	return nil
}

type workerTestObjects struct {
	content []byte
	getKey  string
}

func (s *workerTestObjects) Put(_ context.Context, _ storage.PutObjectInput) (storage.ObjectInfo, error) {
	return storage.ObjectInfo{}, nil
}

func (s *workerTestObjects) Get(_ context.Context, key string) (io.ReadCloser, error) {
	s.getKey = key
	return io.NopCloser(bytes.NewReader(s.content)), nil
}

func (s *workerTestObjects) PresignedGetURL(_ context.Context, _ string, _ time.Duration) (*url.URL, error) {
	return nil, nil
}

func (s *workerTestObjects) Delete(_ context.Context, _ string) error {
	return nil
}

type workerTestExtractor struct {
	text string
	err  error
}

func (e workerTestExtractor) ExtractText(_ io.Reader, _ string) (string, error) {
	return e.text, e.err
}

func TestTextExtractionProcessorParsesResume(t *testing.T) {
	store := &workerTestStore{
		resume: Resume{
			ID:          "resume_1",
			StorageKey:  "resumes/user/random.pdf",
			MimeType:    MimePDF,
			ParseStatus: StatusUploaded,
		},
	}
	objects := &workerTestObjects{content: []byte("raw")}
	processor := NewTextExtractionProcessor(store, objects, workerTestExtractor{text: "extracted text"})

	task := asynq.NewTask(TaskTypeExtractText, []byte(`{"resume_id":"resume_1"}`))
	if err := processor.ProcessTask(context.Background(), task); err != nil {
		t.Fatalf("ProcessTask() error = %v", err)
	}
	if !store.markedParsing {
		t.Fatal("MarkParsing was not called")
	}
	if string(store.parsedText) != "extracted text" {
		t.Fatalf("parsed text = %q", string(store.parsedText))
	}
	if objects.getKey != "resumes/user/random.pdf" {
		t.Fatalf("get key = %q", objects.getKey)
	}
}

func TestTextExtractionProcessorMarksFailedAndRetries(t *testing.T) {
	store := &workerTestStore{
		resume: Resume{
			ID:          "resume_1",
			StorageKey:  "resumes/user/random.pdf",
			MimeType:    MimePDF,
			ParseStatus: StatusUploaded,
		},
	}
	processor := NewTextExtractionProcessor(store, &workerTestObjects{}, workerTestExtractor{err: ErrExtractedTextEmpty})

	task := asynq.NewTask(TaskTypeExtractText, []byte(`{"resume_id":"resume_1"}`))
	err := processor.ProcessTask(context.Background(), task)
	if !errors.Is(err, ErrExtractedTextEmpty) {
		t.Fatalf("ProcessTask() error = %v, want %v", err, ErrExtractedTextEmpty)
	}
	if store.failedReason != "no extractable text found" {
		t.Fatalf("failed reason = %q", store.failedReason)
	}
}

func TestParseExtractTextPayloadRejectsInvalidPayloadWithoutRetry(t *testing.T) {
	task := asynq.NewTask(TaskTypeExtractText, []byte(`{`))
	_, err := ParseExtractTextPayload(task)
	if !errors.Is(err, asynq.SkipRetry) {
		t.Fatalf("ParseExtractTextPayload() error = %v, want SkipRetry", err)
	}
}
