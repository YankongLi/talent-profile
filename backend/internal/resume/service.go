package resume

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/YankongLi/talent-profile/backend/internal/storage"
)

const (
	MaxUploadBytes = 10 * 1024 * 1024

	MimePDF  = "application/pdf"
	MimeDOCX = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"

	StatusUploaded = "uploaded"
	StatusParsing  = "parsing"
	StatusParsed   = "parsed"
	StatusFailed   = "failed"
)

var (
	ErrFileRequired        = errors.New("resume file is required")
	ErrFileTooLarge        = errors.New("resume file is too large")
	ErrUnsupportedFileType = errors.New("unsupported resume file type")
	ErrInvalidFileHeader   = errors.New("invalid resume file header")
	ErrInvalidUserID       = errors.New("invalid user id")
	ErrInvalidResumeID     = errors.New("invalid resume id")
	ErrNotFound            = errors.New("resume not found")
)

type Store interface {
	Create(ctx context.Context, input CreateInput) (Resume, error)
	GetByUserID(ctx context.Context, userID string, resumeID string) (Resume, error)
	SoftDeleteByUserID(ctx context.Context, userID string, resumeID string, deletedAt time.Time) error
}

type Service struct {
	store   Store
	objects storage.Client
}

type Resume struct {
	ID               string    `json:"id"`
	UserID           string    `json:"user_id"`
	StorageKey       string    `json:"-"`
	OriginalFilename string    `json:"original_filename"`
	MimeType         string    `json:"mime_type"`
	FileSize         int64     `json:"file_size"`
	ContentHash      string    `json:"-"`
	ParseStatus      string    `json:"parse_status"`
	ParseError       string    `json:"parse_error,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
}

type UploadInput struct {
	OriginalFilename string
	ContentType      string
	Reader           io.Reader
}

type CreateInput struct {
	UserID           string
	StorageKey       string
	OriginalFilename string
	MimeType         string
	FileSize         int64
	ContentHash      string
	ParseStatus      string
}

func NewService(store Store, objects storage.Client) *Service {
	return &Service{store: store, objects: objects}
}

func (s *Service) Status(ctx context.Context, userID string, resumeID string) (Resume, error) {
	if strings.TrimSpace(userID) == "" {
		return Resume{}, ErrInvalidUserID
	}
	if strings.TrimSpace(resumeID) == "" {
		return Resume{}, ErrInvalidResumeID
	}
	return s.store.GetByUserID(ctx, userID, resumeID)
}

func (s *Service) Delete(ctx context.Context, userID string, resumeID string) error {
	resume, err := s.Status(ctx, userID, resumeID)
	if err != nil {
		return err
	}

	if err := s.objects.Delete(ctx, resume.StorageKey); err != nil {
		return fmt.Errorf("delete resume object: %w", err)
	}
	return s.store.SoftDeleteByUserID(ctx, userID, resumeID, time.Now().UTC())
}

func (s *Service) Upload(ctx context.Context, userID string, input UploadInput) (Resume, error) {
	if strings.TrimSpace(userID) == "" {
		return Resume{}, ErrInvalidUserID
	}
	if input.Reader == nil {
		return Resume{}, ErrFileRequired
	}

	content, err := readLimited(input.Reader, MaxUploadBytes)
	if err != nil {
		return Resume{}, err
	}
	if len(content) == 0 {
		return Resume{}, ErrFileRequired
	}

	mimeType, err := validateFile(input.OriginalFilename, input.ContentType, content)
	if err != nil {
		return Resume{}, err
	}

	objectKey, err := storage.RandomObjectKey("resumes/"+userID, input.OriginalFilename)
	if err != nil {
		return Resume{}, err
	}

	if _, err := s.objects.Put(ctx, storage.PutObjectInput{
		Key:         objectKey,
		Reader:      bytes.NewReader(content),
		Size:        int64(len(content)),
		ContentType: mimeType,
		Metadata: map[string]string{
			"original-filename": filepath.Base(input.OriginalFilename),
		},
	}); err != nil {
		return Resume{}, fmt.Errorf("store resume object: %w", err)
	}

	contentHash := sha256.Sum256(content)
	resume, err := s.store.Create(ctx, CreateInput{
		UserID:           userID,
		StorageKey:       objectKey,
		OriginalFilename: filepath.Base(input.OriginalFilename),
		MimeType:         mimeType,
		FileSize:         int64(len(content)),
		ContentHash:      hex.EncodeToString(contentHash[:]),
		ParseStatus:      StatusUploaded,
	})
	if err != nil {
		_ = s.objects.Delete(ctx, objectKey)
		return Resume{}, err
	}
	return resume, nil
}

func readLimited(reader io.Reader, maxBytes int64) ([]byte, error) {
	var buf bytes.Buffer
	written, err := buf.ReadFrom(io.LimitReader(reader, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if written > maxBytes {
		return nil, ErrFileTooLarge
	}
	return buf.Bytes(), nil
}

func validateFile(filename, contentType string, content []byte) (string, error) {
	ext := strings.ToLower(filepath.Ext(filename))
	normalizedContentType := normalizeContentType(contentType)
	sniffedContentType := http.DetectContentType(firstBytes(content, 512))

	switch ext {
	case ".pdf":
		if normalizedContentType != MimePDF || sniffedContentType != MimePDF {
			return "", ErrUnsupportedFileType
		}
		if !bytes.HasPrefix(content, []byte("%PDF-")) {
			return "", ErrInvalidFileHeader
		}
		return MimePDF, nil
	case ".docx":
		if normalizedContentType != MimeDOCX {
			return "", ErrUnsupportedFileType
		}
		if !hasZipHeader(content) || !looksLikeDOCX(content) {
			return "", ErrInvalidFileHeader
		}
		return MimeDOCX, nil
	default:
		return "", ErrUnsupportedFileType
	}
}

func normalizeContentType(value string) string {
	mediaType, _, err := mime.ParseMediaType(strings.TrimSpace(value))
	if err != nil {
		return strings.ToLower(strings.TrimSpace(value))
	}
	return strings.ToLower(mediaType)
}

func firstBytes(content []byte, max int) []byte {
	if len(content) <= max {
		return content
	}
	return content[:max]
}

func hasZipHeader(content []byte) bool {
	return bytes.HasPrefix(content, []byte{'P', 'K', 0x03, 0x04}) ||
		bytes.HasPrefix(content, []byte{'P', 'K', 0x05, 0x06}) ||
		bytes.HasPrefix(content, []byte{'P', 'K', 0x07, 0x08})
}

func looksLikeDOCX(content []byte) bool {
	reader, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		return false
	}

	hasContentTypes := false
	hasDocument := false
	for _, file := range reader.File {
		switch file.Name {
		case "[Content_Types].xml":
			hasContentTypes = true
		case "word/document.xml":
			hasDocument = true
		}
	}
	return hasContentTypes && hasDocument
}
