package money

import "fmt"

// Amount stores money in micros to avoid floating-point accounting drift.
type Amount struct {
	Currency string
	Micros   int64
}

// New creates an Amount.
func New(currency string, micros int64) Amount {
	return Amount{Currency: currency, Micros: micros}
}

// Zero returns a zero amount for currency.
func Zero(currency string) Amount {
	return Amount{Currency: currency}
}

// Add returns a+b when currencies match.
func (a Amount) Add(b Amount) (Amount, error) {
	if a.Currency != b.Currency {
		return Amount{}, fmt.Errorf("currency mismatch %q != %q", a.Currency, b.Currency)
	}
	return Amount{Currency: a.Currency, Micros: a.Micros + b.Micros}, nil
}

// Sub returns a-b when currencies match.
func (a Amount) Sub(b Amount) (Amount, error) {
	if a.Currency != b.Currency {
		return Amount{}, fmt.Errorf("currency mismatch %q != %q", a.Currency, b.Currency)
	}
	return Amount{Currency: a.Currency, Micros: a.Micros - b.Micros}, nil
}

// Negative returns the debit representation of a positive amount.
func (a Amount) Negative() Amount {
	return Amount{Currency: a.Currency, Micros: -a.Micros}
}

// Positive reports whether amount is greater than zero.
func (a Amount) Positive() bool {
	return a.Micros > 0
}
