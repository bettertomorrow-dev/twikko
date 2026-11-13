package schema

import (
	"strings"
	"testing"
)

func TestDecimalPrecisionAndBounds(t *testing.T) {
	a, err := ParseDecimal("9007199254740993.0000000000000001")
	if err != nil {
		t.Fatal(err)
	}
	b, err := ParseDecimal("9007199254740993.00000000000000010")
	if err != nil || !a.Equal(b) {
		t.Fatalf("equivalent decimals: %v", err)
	}
	c, err := ParseDecimal("9007199254740993")
	if err != nil || a.Equal(c) {
		t.Fatal("lost decimal precision")
	}
	for _, s := range []string{"1/3", "NaN", "1e10001", "1e-10001", strings.Repeat("1", 1025)} {
		if _, err := ParseDecimal(s); err == nil {
			t.Errorf("accepted %q", s)
		}
	}
}
