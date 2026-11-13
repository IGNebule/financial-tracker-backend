package money

import (
	"errors"
	"fmt"
)

var ErrUnsupportedCurrency = errors.New("money: unsupported currency")

var supportedCurrencies = map[string]struct{}{
	"EUR": {},
	"USD": {},
	"GBP": {},
}

func Format(minorUnits int64, currency string) (string, error) {
	if _, ok := supportedCurrencies[currency]; !ok {
		return "", fmt.Errorf("%w: %q", ErrUnsupportedCurrency, currency)
	}

	sign := ""
	if minorUnits < 0 {
		sign = "-"
		minorUnits = -minorUnits
	}

	whole := minorUnits / 100
	fraction := minorUnits % 100
	return fmt.Sprintf("%s%d.%02d %s", sign, whole, fraction, currency), nil
}

func IsSupportedCurrency(code string) bool {
	_, ok := supportedCurrencies[code]
	return ok
}
