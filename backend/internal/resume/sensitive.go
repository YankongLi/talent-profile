package resume

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"sort"
	"strings"
)

const (
	SensitiveTypeEmail = "email"
	SensitiveTypePhone = "phone"
)

var (
	emailPattern = regexp.MustCompile(`(?i)\b[A-Z0-9._%+\-]+@[A-Z0-9.\-]+\.[A-Z]{2,}\b`)
	phonePattern = regexp.MustCompile(`(?:\+?86[-.\s]?)?1[3-9]\d[-.\s]?\d{4}[-.\s]?\d{4}`)
)

type SensitiveField struct {
	Type        string `json:"type"`
	MaskedValue string `json:"masked_value"`
	ValueHash   string `json:"value_hash"`
	Start       int    `json:"start"`
	End         int    `json:"end"`
}

type ParseResult struct {
	ExtractedText   []byte
	RedactedText    string
	SensitiveFields []SensitiveField
}

type sensitiveMatch struct {
	start int
	end   int
	kind  string
	value string
}

func BuildParseResult(extractedText string) ParseResult {
	matches := detectSensitiveMatches(extractedText)
	fields := make([]SensitiveField, 0, len(matches))
	redacted := extractedText

	for index := len(matches) - 1; index >= 0; index-- {
		match := matches[index]
		masked := maskSensitiveValue(match.kind, match.value)
		redacted = redacted[:match.start] + masked + redacted[match.end:]
		fields = append(fields, SensitiveField{
			Type:        match.kind,
			MaskedValue: masked,
			ValueHash:   hashSensitiveValue(match.kind, match.value),
			Start:       match.start,
			End:         match.end,
		})
	}

	for i, j := 0, len(fields)-1; i < j; i, j = i+1, j-1 {
		fields[i], fields[j] = fields[j], fields[i]
	}

	return ParseResult{
		ExtractedText:   []byte(extractedText),
		RedactedText:    redacted,
		SensitiveFields: fields,
	}
}

func detectSensitiveMatches(text string) []sensitiveMatch {
	matches := []sensitiveMatch{}
	for _, loc := range emailPattern.FindAllStringIndex(text, -1) {
		matches = append(matches, sensitiveMatch{
			start: loc[0],
			end:   loc[1],
			kind:  SensitiveTypeEmail,
			value: text[loc[0]:loc[1]],
		})
	}
	for _, loc := range phonePattern.FindAllStringIndex(text, -1) {
		if !validPhoneBoundary(text, loc[0], loc[1]) {
			continue
		}
		matches = append(matches, sensitiveMatch{
			start: loc[0],
			end:   loc[1],
			kind:  SensitiveTypePhone,
			value: text[loc[0]:loc[1]],
		})
	}

	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].start == matches[j].start {
			return matches[i].end > matches[j].end
		}
		return matches[i].start < matches[j].start
	})

	filtered := matches[:0]
	lastEnd := -1
	for _, match := range matches {
		if match.start < lastEnd {
			continue
		}
		filtered = append(filtered, match)
		lastEnd = match.end
	}
	return filtered
}

func validPhoneBoundary(text string, start int, end int) bool {
	if start > 0 && isDigit(text[start-1]) {
		return false
	}
	if end < len(text) && isDigit(text[end]) {
		return false
	}
	return true
}

func maskSensitiveValue(kind string, value string) string {
	switch kind {
	case SensitiveTypeEmail:
		parts := strings.SplitN(value, "@", 2)
		if len(parts) != 2 {
			return "***"
		}
		local := parts[0]
		if local == "" {
			return "***@" + parts[1]
		}
		return local[:1] + "***@" + parts[1]
	case SensitiveTypePhone:
		digits := onlyDigits(value)
		if len(digits) <= 4 {
			return "****"
		}
		return "*******" + digits[len(digits)-4:]
	default:
		return "***"
	}
}

func hashSensitiveValue(kind string, value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if kind == SensitiveTypePhone {
		normalized = onlyDigits(value)
	}
	sum := sha256.Sum256([]byte(kind + "\x00" + normalized))
	return hex.EncodeToString(sum[:])
}

func onlyDigits(value string) string {
	var builder strings.Builder
	for _, r := range value {
		if r >= '0' && r <= '9' {
			builder.WriteRune(r)
		}
	}
	return builder.String()
}

func isDigit(b byte) bool {
	return b >= '0' && b <= '9'
}
