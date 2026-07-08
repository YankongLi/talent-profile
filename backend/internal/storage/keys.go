package storage

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"
)

const randomKeyBytes = 16

func RandomObjectKey(prefix, originalName string) (string, error) {
	random := make([]byte, randomKeyBytes)
	if _, err := rand.Read(random); err != nil {
		return "", fmt.Errorf("generate random object key: %w", err)
	}

	name := hex.EncodeToString(random) + safeExtension(originalName)
	if prefix := cleanPrefix(prefix); prefix != "" {
		return prefix + "/" + name, nil
	}
	return name, nil
}

func safeExtension(name string) string {
	ext := strings.ToLower(filepath.Ext(name))
	if len(ext) < 2 || len(ext) > 16 {
		return ""
	}
	for _, r := range ext[1:] {
		if !isASCIIAlnum(r) {
			return ""
		}
	}
	return ext
}

func cleanPrefix(prefix string) string {
	rawParts := strings.FieldsFunc(strings.TrimSpace(prefix), func(r rune) bool {
		return r == '/' || r == '\\'
	})

	parts := make([]string, 0, len(rawParts))
	for _, raw := range rawParts {
		part := cleanPathSegment(raw)
		if part != "" {
			parts = append(parts, part)
		}
	}
	return strings.Join(parts, "/")
}

func cleanPathSegment(segment string) string {
	var builder strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(strings.TrimSpace(segment)) {
		switch {
		case isASCIIAlnum(r), r == '_':
			builder.WriteRune(r)
			lastDash = false
		case r == '-':
			if !lastDash {
				builder.WriteByte('-')
				lastDash = true
			}
		default:
			if builder.Len() > 0 && !lastDash {
				builder.WriteByte('-')
				lastDash = true
			}
		}
	}
	return strings.Trim(builder.String(), "-_")
}

func isASCIIAlnum(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
}
