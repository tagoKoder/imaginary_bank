// bff\internal\util\money.go
package util

import (
	"errors"
	"regexp"
)

// Format only: currency-specific precision and upper bounds are checked in Accounts/Ledger.
var moneyPattern = regexp.MustCompile(`^(0|[1-9][0-9]{0,13})(\.[0-9]{1,6})?$`)
var ErrInvalidMoney = errors.New("invalid decimal representation")

func ValidateMoney(s string) error {
	if !moneyPattern.MatchString(s) {
		return ErrInvalidMoney
	}
	return nil
}

func ValidatePositiveMoney(s string) error {
	if err := ValidateMoney(s); err != nil {
		return err
	}
	for _, c := range s {
		if c >= '1' && c <= '9' {
			return nil
		}
	}
	return ErrInvalidMoney
}
