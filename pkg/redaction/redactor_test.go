package redaction

import "testing"

func TestRedactPII(t *testing.T) {
	value := RedactPII("email alice@example.com phone +1 415-555-0100 bearer sk-secret-token-value")
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

func contains(value, want string) bool {
	for i := 0; i+len(want) <= len(value); i++ {
		if value[i:i+len(want)] == want {
			return true
		}
	}
	return false
}
