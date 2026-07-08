package resume

import (
	"strings"
	"testing"
)

func TestBuildParseResultRedactsSensitiveFields(t *testing.T) {
	result := BuildParseResult("Contact: Jane.Doe@example.com, phone 138 0013 8000.")

	if result.RedactedText != "Contact: J***@example.com, phone *******8000." {
		t.Fatalf("RedactedText = %q", result.RedactedText)
	}
	if string(result.ExtractedText) != "Contact: Jane.Doe@example.com, phone 138 0013 8000." {
		t.Fatalf("ExtractedText = %q", string(result.ExtractedText))
	}
	if len(result.SensitiveFields) != 2 {
		t.Fatalf("SensitiveFields len = %d, want 2", len(result.SensitiveFields))
	}
	if result.SensitiveFields[0].Type != SensitiveTypeEmail {
		t.Fatalf("first sensitive type = %q", result.SensitiveFields[0].Type)
	}
	if result.SensitiveFields[0].ValueHash == "" || strings.Contains(result.SensitiveFields[0].ValueHash, "Jane") {
		t.Fatalf("email hash = %q", result.SensitiveFields[0].ValueHash)
	}
	if result.SensitiveFields[1].Type != SensitiveTypePhone {
		t.Fatalf("second sensitive type = %q", result.SensitiveFields[1].Type)
	}
	if result.SensitiveFields[1].MaskedValue != "*******8000" {
		t.Fatalf("phone masked value = %q", result.SensitiveFields[1].MaskedValue)
	}
}

func TestBuildParseResultDoesNotMatchPhoneInsideLongDigits(t *testing.T) {
	result := BuildParseResult("invoice id 9913800138000123")
	if len(result.SensitiveFields) != 0 {
		t.Fatalf("SensitiveFields len = %d, want 0", len(result.SensitiveFields))
	}
	if result.RedactedText != "invoice id 9913800138000123" {
		t.Fatalf("RedactedText = %q", result.RedactedText)
	}
}
