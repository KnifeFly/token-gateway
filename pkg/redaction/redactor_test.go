package redaction

import "testing"

func TestRedactPII(t *testing.T) {
	value := RedactPII("email alice@example.com phone +1 415-555-0100 bearer sk-secret-token-value api sk-1234567890abcdef")
	if value == "email alice@example.com phone +1 415-555-0100 bearer sk-secret-token-value" {
		t.Fatal("value was not redacted")
	}
	if want := "[REDACTED_EMAIL]"; !contains(value, want) {
		t.Fatalf("redacted value %q does not contain %q", value, want)
	}
	if want := "[REDACTED_PHONE]"; !contains(value, want) {
		t.Fatalf("redacted value %q does not contain %q", value, want)
	}
	if want := "[REDACTED_SECRET]"; !contains(value, want) {
		t.Fatalf("redacted value %q does not contain %q", value, want)
	}
}

func TestRedactField(t *testing.T) {
	if got := RedactField("api_key", "sk-1234567890abcdef"); got != "[REDACTED_SECRET]" {
		t.Fatalf("api key = %q", got)
	}
	if got := RedactField("prompt", "raw prompt"); got != "[REDACTED_PROMPT]" {
		t.Fatalf("prompt = %q", got)
	}
	if got := RedactField("response", "raw response"); got != "[REDACTED_RESPONSE]" {
		t.Fatalf("response = %q", got)
	}
}

func contains(value, want string) bool {
	for i := 0; i+len(want) <= len(value); i++ {
		if value[i:i+len(want)] == want {
			return true
		}
	}
	return false
}
