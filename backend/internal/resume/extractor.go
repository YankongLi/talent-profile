package resume

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode"

	"github.com/ledongthuc/pdf"
)

const maxExtractedTextBytes = 1024 * 1024

var (
	ErrUnsupportedExtractionType = errors.New("unsupported extraction file type")
	ErrExtractedTextEmpty        = errors.New("extracted text is empty")
	ErrExtractedTextTooLarge     = errors.New("extracted text is too large")
)

type TextExtractor interface {
	ExtractText(ctxReader io.Reader, mimeType string) (string, error)
}

type DefaultTextExtractor struct{}

func (DefaultTextExtractor) ExtractText(reader io.Reader, mimeType string) (string, error) {
	content, err := readLimited(reader, MaxUploadBytes)
	if err != nil {
		return "", err
	}

	var text string
	switch mimeType {
	case MimePDF:
		text, err = extractPDFText(content)
	case MimeDOCX:
		text, err = extractDOCXText(content)
	default:
		return "", ErrUnsupportedExtractionType
	}
	if err != nil {
		return "", err
	}

	text = normalizeExtractedText(text)
	if text == "" {
		return "", ErrExtractedTextEmpty
	}
	if len(text) > maxExtractedTextBytes {
		return "", ErrExtractedTextTooLarge
	}
	return text, nil
}

func extractPDFText(content []byte) (string, error) {
	reader, err := pdf.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		return "", fmt.Errorf("read pdf: %w", err)
	}
	plainReader, err := reader.GetPlainText()
	if err != nil {
		return "", fmt.Errorf("extract pdf text: %w", err)
	}
	data, err := io.ReadAll(io.LimitReader(plainReader, maxExtractedTextBytes+1))
	if err != nil {
		return "", err
	}
	if len(data) > maxExtractedTextBytes {
		return "", ErrExtractedTextTooLarge
	}
	return string(data), nil
}

func extractDOCXText(content []byte) (string, error) {
	reader, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		return "", fmt.Errorf("read docx: %w", err)
	}

	var builder strings.Builder
	for _, file := range reader.File {
		if file.Name != "word/document.xml" {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			return "", err
		}
		err = appendWordXMLText(&builder, rc)
		closeErr := rc.Close()
		if err != nil {
			return "", err
		}
		if closeErr != nil {
			return "", closeErr
		}
		return builder.String(), nil
	}
	return "", ErrInvalidFileHeader
}

func appendWordXMLText(builder *strings.Builder, reader io.Reader) error {
	decoder := xml.NewDecoder(reader)
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}

		switch value := token.(type) {
		case xml.CharData:
			text := strings.TrimSpace(string(value))
			if text != "" {
				if builder.Len() > 0 {
					builder.WriteByte(' ')
				}
				builder.WriteString(text)
			}
		case xml.StartElement:
			if value.Name.Local == "p" && builder.Len() > 0 {
				builder.WriteByte('\n')
			}
		}
	}
}

func normalizeExtractedText(value string) string {
	var builder strings.Builder
	lastSpace := false
	lastNewline := false
	for _, r := range strings.TrimSpace(value) {
		switch {
		case r == '\n' || r == '\r':
			if !lastNewline && builder.Len() > 0 {
				builder.WriteByte('\n')
			}
			lastSpace = false
			lastNewline = true
		case unicode.IsSpace(r):
			if !lastSpace && !lastNewline && builder.Len() > 0 {
				builder.WriteByte(' ')
			}
			lastSpace = true
		default:
			builder.WriteRune(r)
			lastSpace = false
			lastNewline = false
		}
	}
	return strings.TrimSpace(builder.String())
}
