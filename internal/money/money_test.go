package money

import (
	"errors"
	"strings"
	"testing"
)

func TestFormatPositiveAmount(t *testing.T) {
	got, err := Format(1234, "EUR")
	if err != nil {
		t.Fatalf("Format(1234, \"EUR\") returned unexpected error: %v", err)
	}
	if want := "12.34 EUR"; got != want {
		t.Fatalf("Format(1234, \"EUR\") = %q, want %q", got, want)
	}
}

func TestFormatNegativeAmount(t *testing.T) {
	got, err := Format(-1234, "EUR")
	if err != nil {
		t.Fatalf("Format(-1234, \"EUR\") returned unexpected error: %v", err)
	}
	if want := "-12.34 EUR"; got != want {
		t.Fatalf("Format(-1234, \"EUR\") = %q, want %q", got, want)
	}
}

func TestFormatNegativeSmallAmount(t *testing.T) {
	got, err := Format(-5, "EUR")
	if err != nil {
		t.Fatalf("Format(-5, \"EUR\") returned unexpected error: %v", err)
	}
	if want := "-0.05 EUR"; got != want {
		t.Fatalf("Format(-5, \"EUR\") = %q, want %q", got, want)
	}
}

func TestFormatZero(t *testing.T) {
	got, err := Format(0, "EUR")
	if err != nil {
		t.Fatalf("Format(0, \"EUR\") returned unexpected error: %v", err)
	}
	if want := "0.00 EUR"; got != want {
		t.Fatalf("Format(0, \"EUR\") = %q, want %q", got, want)
	}
}

func TestFormatUnsupportedCurrency(t *testing.T) {
	got, err := Format(1234, "XYZ")

	if err == nil {
		t.Fatal("Format(1234, \"XYZ\") returned nil error, want ErrUnsupportedCurrency")
	}
	if !errors.Is(err, ErrUnsupportedCurrency) {
		t.Fatalf("Format(1234, \"XYZ\") error = %v, want it to wrap ErrUnsupportedCurrency", err)
	}
	if !strings.Contains(err.Error(), "XYZ") {
		t.Fatalf("Format(1234, \"XYZ\") error = %q, want it to mention the offending code", err.Error())
	}
	if got != "" {
		t.Fatalf("Format(1234, \"XYZ\") value = %q, want empty string when error is non-nil", got)
	}
}
