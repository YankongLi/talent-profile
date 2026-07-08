package resume

import (
	"context"
	"errors"
	"strings"

	"github.com/YankongLi/talent-profile/backend/internal/storage"
	"github.com/hibiken/asynq"
)

const maxParseErrorLength = 500

type WorkerStore interface {
	GetByID(ctx context.Context, resumeID string) (Resume, error)
	MarkParsing(ctx context.Context, resumeID string) error
	MarkParsed(ctx context.Context, resumeID string, extractedText []byte) error
	MarkFailed(ctx context.Context, resumeID string, reason string) error
}

type TextExtractionProcessor struct {
	store     WorkerStore
	objects   storage.Client
	extractor TextExtractor
}

func NewTextExtractionProcessor(store WorkerStore, objects storage.Client, extractor TextExtractor) *TextExtractionProcessor {
	if extractor == nil {
		extractor = DefaultTextExtractor{}
	}
	return &TextExtractionProcessor{store: store, objects: objects, extractor: extractor}
}

func (p *TextExtractionProcessor) Register(mux *asynq.ServeMux) {
	mux.HandleFunc(TaskTypeExtractText, p.ProcessTask)
}

func (p *TextExtractionProcessor) ProcessTask(ctx context.Context, task *asynq.Task) error {
	payload, err := ParseExtractTextPayload(task)
	if err != nil {
		return err
	}

	resume, err := p.store.GetByID(ctx, payload.ResumeID)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if resume.ParseStatus == StatusParsed {
		return nil
	}

	if err := p.store.MarkParsing(ctx, resume.ID); err != nil {
		return err
	}

	object, err := p.objects.Get(ctx, resume.StorageKey)
	if err != nil {
		_ = p.store.MarkFailed(ctx, resume.ID, publicParseError(err))
		return err
	}
	defer object.Close()

	text, err := p.extractor.ExtractText(object, resume.MimeType)
	if err != nil {
		_ = p.store.MarkFailed(ctx, resume.ID, publicParseError(err))
		return err
	}

	if err := p.store.MarkParsed(ctx, resume.ID, []byte(text)); err != nil {
		return err
	}
	return nil
}

func publicParseError(err error) string {
	message := "text extraction failed"
	switch {
	case errors.Is(err, ErrUnsupportedExtractionType):
		message = "unsupported file type"
	case errors.Is(err, ErrInvalidFileHeader):
		message = "invalid file structure"
	case errors.Is(err, ErrExtractedTextEmpty):
		message = "no extractable text found"
	case errors.Is(err, ErrExtractedTextTooLarge), errors.Is(err, ErrFileTooLarge):
		message = "extracted text is too large"
	case err != nil:
		message = err.Error()
	}
	message = strings.TrimSpace(message)
	if len(message) > maxParseErrorLength {
		message = message[:maxParseErrorLength]
	}
	return message
}
