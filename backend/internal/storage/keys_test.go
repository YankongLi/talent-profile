package storage

import (
	"path"
	"regexp"
	"strings"
	"testing"
)

func TestRandomObjectKeyUsesRandomNameAndSafeExtension(t *testing.T) {
	key, err := RandomObjectKey("resumes/User 123", "Jane Resume.PDF")
	if err != nil {
		t.Fatalf("RandomObjectKey() error = %v", err)
	}

	matched, err := regexp.MatchString(`^resumes/user-123/[0-9a-f]{32}\.pdf$`, key)
	if err != nil {
		t.Fatalf("regexp error = %v", err)
	}
	if !matched {
		t.Fatalf("key = %q, want safe prefix, random hex name, and .pdf extension", key)
	}
	base := strings.ToLower(path.Base(key))
	if strings.Contains(base, "jane") || strings.Contains(base, "resume") {
		t.Fatalf("key = %q contains original filename data", key)
	}
}

func TestRandomObjectKeyDropsUnsafeExtension(t *testing.T) {
	key, err := RandomObjectKey("../Raw Uploads//", "resume.bad-ext!")
	if err != nil {
		t.Fatalf("RandomObjectKey() error = %v", err)
	}

	matched, err := regexp.MatchString(`^raw-uploads/[0-9a-f]{32}$`, key)
	if err != nil {
		t.Fatalf("regexp error = %v", err)
	}
	if !matched {
		t.Fatalf("key = %q, want unsafe extension removed", key)
	}
}

func TestRandomObjectKeyIsUnique(t *testing.T) {
	first, err := RandomObjectKey("resumes", "resume.pdf")
	if err != nil {
		t.Fatalf("RandomObjectKey() first error = %v", err)
	}
	second, err := RandomObjectKey("resumes", "resume.pdf")
	if err != nil {
		t.Fatalf("RandomObjectKey() second error = %v", err)
	}
	if first == second {
		t.Fatalf("two random keys are equal: %q", first)
	}
}
