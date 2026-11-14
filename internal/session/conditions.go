package session

import (
	"math/big"
	"strings"
	"time"
	"unicode/utf8"

	"go.yaml.in/yaml/v3"
	"twikko/internal/schema"
)

// RowHighlight returns the first matching rule's intent, or an empty intent.
func (s *Store) RowHighlight(index int, now time.Time) schema.Intent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.view == nil || len(s.view.Table.RowHighlights) == 0 || index < 0 || index >= len(s.current) {
		return ""
	}
	location := time.Local
	if s.view.Table.Location != nil {
		location = s.view.Table.Location
	}
	for _, rule := range s.view.Table.RowHighlights {
		if s.matchesCondition(index, rule.When, now.In(location).Truncate(time.Minute), location) {
			return rule.Intent
		}
	}
	return ""
}

func (s *Store) HasRelativeHighlights() bool {
	if s.view == nil {
		return false
	}
	for _, rule := range s.view.Table.RowHighlights {
		if rule.When.Relative() {
			return true
		}
	}
	return false
}

func (s *Store) matchesCondition(index int, c schema.Condition, now time.Time, location *time.Location) bool {
	if len(c.All) > 0 {
		for _, child := range c.All {
			if !s.matchesCondition(index, child, now, location) {
				return false
			}
		}
		return true
	}
	if len(c.Any) > 0 {
		for _, child := range c.Any {
			if s.matchesCondition(index, child, now, location) {
				return true
			}
		}
		return false
	}
	f, ok := s.field(c.Field)
	if !ok {
		return false
	}
	value, ok := s.conditionValue(index, f)
	if !ok {
		return false
	}
	if c.Measure.Length {
		text, ok := value.(string)
		if !ok {
			return false
		}
		return compareConditionNumber(big.NewRat(int64(utf8.RuneCountInString(text)), 1), c)
	}
	if c.Measure.DistanceFrom != "" || c.Measure.ReferenceField != "" {
		text, ok := value.(string)
		if !ok {
			return false
		}
		a, ok := conditionTemporal(f.Type, text, location)
		if !ok {
			return false
		}
		var b int64
		if c.Measure.ReferenceField != "" {
			ref, exists := s.field(c.Measure.ReferenceField)
			if !exists {
				return false
			}
			raw, exists := s.conditionValue(index, ref)
			if !exists {
				return false
			}
			text, exists := raw.(string)
			if !exists {
				return false
			}
			b, ok = conditionTemporal(f.Type, text, location)
			if !ok {
				return false
			}
		} else {
			switch f.Type {
			case schema.Date:
				b, ok = conditionTemporal(f.Type, now.Format("2006-01-02"), location)
			case schema.Time:
				b = int64(now.Hour()*3600 + now.Minute()*60)
			default:
				b = now.Unix()
			}
		}
		unit := int64(60)
		switch c.Measure.Unit {
		case "hours":
			unit = 3600
		case "days":
			unit = 86400
		}
		return compareConditionNumber(big.NewRat(a-b, unit), c)
	}
	switch v := value.(type) {
	case schema.Decimal:
		number, valid := new(big.Rat).SetString(string(v))
		return valid && compareConditionNumber(number, c)
	case bool:
		expected, valid := c.Value.(bool)
		return valid && ((c.Op == "eq" && v == expected) || (c.Op == "ne" && v != expected))
	case string:
		if c.Op == "contains_all" || c.Op == "contains_any" {
			return compareConditionSet([]string{v}, c)
		}
		expected, valid := c.Value.(string)
		if !valid {
			return false
		}
		switch c.Op {
		case "eq":
			return v == expected
		case "ne":
			return v != expected
		case "contains":
			return strings.Contains(v, expected)
		case "not_contains":
			return !strings.Contains(v, expected)
		}
	case []string:
		return compareConditionSet(v, c)
	}
	return false
}

// An invalid draft must not fall back to the last valid value in current.
func (s *Store) conditionValue(index int, field schema.Field) (any, bool) {
	if s.errors[index][field.Name] != "" {
		return nil, false
	}
	if _, exists := s.drafts[index][field.Name]; exists {
		return nil, false
	}
	n := schema.MapValue(s.current[index], field.Name)
	for n != nil && n.Kind == yaml.AliasNode {
		n = n.Alias
	}
	if n == nil || n.Tag == "!!null" {
		return nil, false
	}
	value, err := schema.ParseNode(field, n)
	return value, err == nil && value != nil
}

func compareConditionNumber(value *big.Rat, c schema.Condition) bool {
	if c.Op == "even" || c.Op == "odd" {
		if !value.IsInt() {
			return false
		}
		even := value.Num().Bit(0) == 0
		return (c.Op == "even" && even) || (c.Op == "odd" && !even)
	}
	d, ok := c.Value.(schema.Decimal)
	if !ok {
		return false
	}
	expected, ok := new(big.Rat).SetString(string(d))
	if !ok {
		return false
	}
	cmp := value.Cmp(expected)
	switch c.Op {
	case "gt":
		return cmp > 0
	case "lt":
		return cmp < 0
	case "eq":
		return cmp == 0
	case "ne":
		return cmp != 0
	}
	return false
}

func compareConditionSet(value []string, c schema.Condition) bool {
	expected, ok := c.Value.([]string)
	if !ok || len(expected) == 0 {
		return false
	}
	for _, option := range expected {
		found := false
		for _, item := range value {
			if item == option {
				found = true
				break
			}
		}
		if c.Op == "contains_any" && found {
			return true
		}
		if c.Op == "contains_all" && !found {
			return false
		}
	}
	return c.Op == "contains_all"
}

// Dates use UTC midnights for calendar-day arithmetic. Time values are seconds
// since midnight. Local datetime values must identify exactly one instant.
func conditionTemporal(typ schema.FieldType, value string, location *time.Location) (int64, bool) {
	var layouts []string
	switch typ {
	case schema.Date:
		layouts = []string{"2006-01-02"}
	case schema.Time:
		layouts = []string{"15:04", "15:04:05"}
	case schema.DateTime:
		layouts = []string{"2006-01-02T15:04", "2006-01-02T15:04:05"}
	default:
		return 0, false
	}
	for _, layout := range layouts {
		naive, err := time.Parse(layout, value)
		if err != nil {
			continue
		}
		if typ == schema.Date {
			return naive.Unix(), true
		}
		if typ == schema.Time {
			return int64(naive.Hour()*3600 + naive.Minute()*60 + naive.Second()), true
		}
		// Collect offsets on both sides of nearby transitions, then count the
		// instants which format to the requested local wall clock.
		offsets := map[int]bool{}
		for hours := -48; hours <= 48; hours += 6 {
			_, offset := naive.Add(time.Duration(hours) * time.Hour).In(location).Zone()
			offsets[offset] = true
		}
		matches := 0
		var instant int64
		for offset := range offsets {
			candidate := naive.Add(-time.Duration(offset) * time.Second)
			if candidate.In(location).Format(layout) == value {
				matches++
				instant = candidate.Unix()
			}
		}
		return instant, matches == 1
	}
	return 0, false
}
