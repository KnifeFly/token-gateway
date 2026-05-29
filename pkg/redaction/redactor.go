package redaction

import "regexp"

var (
	emailPattern       = regexp.MustCompile(`(?i)\b[A-Z0-9._%+\-]+@[A-Z0-9.\-]+\.[A-Z]{2,}\b`)
	phonePattern       = regexp.MustCompile(`\b(?:\+?\d[\d \-().]{7,}\d)\b`)
	bearerTokenPattern = regexp.MustCompile(`(?i)\b(bearer|api[_-]?key)\s+([A-Za-z0-9._~+\-/=]{12,})`)
)

// RedactPII replaces common PII and credential-like tokens with safe markers.
func RedactPII(value string) string {
	value = emailPattern.ReplaceAllString(value, "[REDACTED_EMAIL]")
	value = phonePattern.ReplaceAllString(value, "[REDACTED_PHONE]")
	value = bearerTokenPattern.ReplaceAllString(value, "$1 [REDACTED_SECRET]")
	return value
}

// RedactPIIBytes redacts PII from a byte slice and returns a fresh slice.
func RedactPIIBytes(value []byte) []byte {
	if len(value) == 0 {
		return nil
	}
	return []byte(RedactPII(string(value)))
}
