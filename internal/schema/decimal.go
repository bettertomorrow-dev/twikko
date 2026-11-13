package schema

import (
	"errors"
	"go.yaml.in/yaml/v3"
	"math/big"
	"regexp"
	"strconv"
	"strings"
)

// Decimal retains the original decimal spelling while allowing exact comparison.
type Decimal string

var decimalPattern = regexp.MustCompile(`^[+-]?(?:[0-9]+(?:\.[0-9]*)?|\.[0-9]+)(?:[eE][+-]?[0-9]+)?$`)

// ParseDecimal validates a bounded decimal without converting through float64.
func ParseDecimal(s string) (Decimal, error) {
	if len(s) > 1024 || !decimalPattern.MatchString(s) {
		return "", errors.New("must be a bounded decimal number")
	}
	if i := strings.IndexAny(s, "eE"); i >= 0 {
		exp, err := strconv.ParseInt(s[i+1:], 10, 32)
		if err != nil || exp < -10000 || exp > 10000 {
			return "", errors.New("decimal exponent exceeds limit")
		}
	}
	if _, ok := new(big.Rat).SetString(s); !ok {
		return "", errors.New("must be a decimal number")
	}
	return Decimal(s), nil
}

// Equal compares decimal values exactly.
func (d Decimal) Equal(other Decimal) bool {
	a, ok := new(big.Rat).SetString(string(d))
	if !ok {
		return false
	}
	b, ok := new(big.Rat).SetString(string(other))
	return ok && a.Cmp(b) == 0
}

// Node writes decimal integers without YAML's legacy octal interpretation.
func (d Decimal) Node() *yaml.Node {
	text := string(d)
	tag := "!!float"
	if !strings.ContainsAny(text, ".eE") {
		n, _ := new(big.Int).SetString(text, 10)
		text = n.String()
		tag = "!!int"
	}
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: tag, Value: text}
}
func (d Decimal) MarshalYAML() (any, error) { return d.Node(), nil }

// NodeDecimal reads YAML integer notation using YAML semantics, including
// legacy octal, before displaying or comparing a decimal editor value.
func NodeDecimal(node *yaml.Node) (Decimal, error) {
	node = deref(node)
	if node == nil || node.Kind != yaml.ScalarNode || (node.Tag != "!!int" && node.Tag != "!!float") {
		return "", errors.New("must be a number")
	}
	if node.Tag == "!!int" {
		if len(node.Value) > 1024 {
			return "", errors.New("number exceeds limit")
		}
		if n, ok := new(big.Int).SetString(node.Value, 0); ok {
			return ParseDecimal(n.String())
		}
	}
	return ParseDecimal(node.Value)
}
