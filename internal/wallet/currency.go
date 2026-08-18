package wallet

import "fmt"

type Currency string

const (
	PLN Currency = "PLN"
)

func (c Currency) IsValid() bool {
	switch c {
	case PLN:
		return true
	default:
		return false
	}
}

func (c Currency) String() string {
	return string(c)
}

func ParseCurrency(s string) (Currency, error) {
	c := Currency(s)
	if !c.IsValid() {
		return "", fmt.Errorf("unsupported currency: %s", s)
	}
	return c, nil
}
