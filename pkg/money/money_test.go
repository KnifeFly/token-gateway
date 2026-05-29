package money

import "testing"

func TestAmountOperations(t *testing.T) {
	a := New("USD", 100)
	b := New("USD", 40)
	got, err := a.Sub(b)
	if err != nil {
		t.Fatalf("Sub() error = %v", err)
	}
	if got.Micros != 60 {
		t.Fatalf("micros = %d", got.Micros)
	}
	if got.Negative().Micros != -60 {
		t.Fatalf("negative = %d", got.Negative().Micros)
	}
}

func TestAmountRejectsCurrencyMismatch(t *testing.T) {
	if _, err := New("USD", 1).Add(New("CNY", 1)); err == nil {
		t.Fatal("expected currency mismatch")
	}
}
