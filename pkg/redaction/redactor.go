package redaction

import "regexp"

var (
	emailPattern       = regexp.MustCompile(`(?i)\b[A-Z0-9._%+\-]+@[A-Z0-9.\-]+\.[A-Z]{2,}\b`)
	phonePattern       = regexp.MustCompile(`\b(?:\+?\d[\d \-().]{7,}\d)\b`)
	bearerTokenPattern = regexp.MustCompile(`(?i)\b(bearer|api[_-]?key)\s+([A-Za-z0-9._~+\-/=]{12,})`)
	apiKeyPattern      = regexp.MustCompile(`\b(?:sk|tg|pk|rk)-?[A-Za-z0-9_]{12,}\b`)
)

// Redact replaces common PII, API keys, provider keys, and token-like secrets.
func Redact(value string) string {
	value = emailPattern.ReplaceAllString(value, "[REDACTED_EMAIL]")
	value = phonePattern.ReplaceAllString(value, "[REDACTED_PHONE]")
	value = bearerTokenPattern.ReplaceAllString(value, "$1 [REDACTED_SECRET]")
	value = apiKeyPattern.ReplaceAllString(value, "[REDACTED_SECRET]")
	return value
}

// RedactPII replaces common PII and credential-like tokens with safe markers.
func RedactPII(value string) string {
	return Redact(value)
}

// RedactPIIBytes redacts PII from a byte slice and returns a fresh slice.
func RedactPIIBytes(value []byte) []byte {
	if len(value) == 0 {
		return nil
	}
	return []byte(Redact(string(value)))
}

// RedactField redacts sensitive structured fields by key.
func RedactField(key string, value string) string {
	switch key {
	case "api_key", "provider_key", "provider_api_key", "authorization", "token", "secret", "credential":
		return "[REDACTED_SECRET]"
	case "prompt":
		return "[REDACTED_PROMPT]"
	case "response":
		return "[REDACTED_RESPONSE]"
	default:
		return Redact(value)
	}
}
