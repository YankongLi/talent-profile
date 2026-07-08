package resume

import (
	"bytes"
	"testing"
)

func TestDefaultTextExtractorExtractsDOCX(t *testing.T) {
	text, err := (DefaultTextExtractor{}).ExtractText(bytes.NewReader(testDOCX(t)), MimeDOCX)
	if err != nil {
		t.Fatalf("ExtractText() error = %v", err)
	}
	if text != "Hello world" {
		t.Fatalf("text = %q, want Hello world", text)
	}
}

func TestDefaultTextExtractorRejectsUnsupportedMime(t *testing.T) {
	_, err := (DefaultTextExtractor{}).ExtractText(bytes.NewReader([]byte("hello")), "text/plain")
	if err != ErrUnsupportedExtractionType {
		t.Fatalf("ExtractText() error = %v, want %v", err, ErrUnsupportedExtractionType)
	}
}
