package apperr

import (
	"errors"
	"net/http"
	"testing"
)

func TestSafeMessage(t *testing.T) {
	err := Internal("database password leaked", WithUnsafeMessage())
	if got := err.SafeMessage(); got != "internal error" {
		t.Fatalf("SafeMessage() = %q", got)
	}
}

func TestAs(t *testing.T) {
	cause := errors.New("boom")
	err := ConfigUnavailable("db unavailable", WithCause(cause), WithTemporary())

	got, ok := As(err)
	if !ok {
		t.Fatal("As() did not identify app error")
	}
	if got.HTTPStatus != http.StatusServiceUnavailable {
		t.Fatalf("HTTPStatus = %d", got.HTTPStatus)
	}
	if !got.Temporary {
		t.Fatal("Temporary = false")
	}
	if !errors.Is(got, cause) {
		t.Fatal("cause was not preserved")
	}
}
