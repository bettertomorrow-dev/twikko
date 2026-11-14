package session

import (
	"cmp"
	"fmt"
	"math/big"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"twikko/internal/schema"
)

// TableColumns uses layout slots as column identities, including repeated displays.
func (s *Store) TableColumns() []schema.Widget {
	if s.view != nil && len(s.view.Table.Columns) > 0 {
		return append([]schema.Widget(nil), s.view.Table.Columns...)
	}
	columns := make([]schema.Widget, len(s.schema.Fields))
	for i, field := range s.schema.Fields {
		columns[i] = schema.Widget{Type: schema.WidgetControl, Source: field.Name}
	}
	return columns
}

// SortedIndices returns a display order without changing records or their identities.
func (s *Store) SortedIndices(column, direction string) ([]int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.mode != "table" {
		return nil, fmt.Errorf("sorting requires table mode")
	}
	if direction != "asc" && direction != "desc" {
		return nil, fmt.Errorf("direction must be asc or desc")
	}
	order := make([]int, len(s.current))
	for i := range order {
		order[i] = i
	}
	if column == "number" {
		if direction == "desc" {
			slices.Reverse(order)
		}
		return order, nil
	}
	columns := s.TableColumns()
	index, err := strconv.Atoi(column)
	if err != nil || index < 0 || index >= len(columns) {
		return nil, fmt.Errorf("unknown column")
	}
	field, ok := s.field(columns[index].Source)
	if !ok {
		return nil, fmt.Errorf("column has no declared field type")
	}
	values := make([]any, len(order))
	for i, entity := range s.current {
		if s.errors[i][field.Name] != "" {
			continue
		}
		value, err := schema.ParseNode(field, schema.MapValue(entity, field.Name))
		if err == nil && value != nil {
			values[i] = sortValue(field, value)
		}
	}
	sort.SliceStable(order, func(i, j int) bool {
		a, b := values[order[i]], values[order[j]]
		// Missing and invalid values stay last in either direction.
		if a == nil || b == nil {
			return a != nil && b == nil
		}
		comparison := compareSortValues(a, b)
		if direction == "desc" {
			return comparison > 0
		}
		return comparison < 0
	})
	return order, nil
}

func sortValue(field schema.Field, value any) any {
	switch field.Type {
	case schema.Number:
		n, _ := new(big.Rat).SetString(string(value.(schema.Decimal)))
		return n
	case schema.Enum:
		return slices.Index(field.Options, value.(string))
	case schema.MultiEnum:
		selected := value.([]string)
		indices := make([]int, 0, len(selected))
		for i, option := range field.Options {
			if slices.Contains(selected, option) {
				indices = append(indices, i)
			}
		}
		return indices
	case schema.Date, schema.Time, schema.DateTime:
		for _, layout := range []string{"2006-01-02", "15:04", "15:04:05", "2006-01-02T15:04", "2006-01-02T15:04:05"} {
			if parsed, err := time.Parse(layout, value.(string)); err == nil {
				return parsed
			}
		}
		return nil
	default:
		return value
	}
}

func compareSortValues(a, b any) int {
	switch v := a.(type) {
	case *big.Rat:
		return v.Cmp(b.(*big.Rat))
	case time.Time:
		return v.Compare(b.(time.Time))
	case int:
		return cmp.Compare(v, b.(int))
	case []int:
		return slices.Compare(v, b.([]int))
	case bool:
		if v == b.(bool) {
			return 0
		}
		if v {
			return 1
		}
		return -1
	default:
		return strings.Compare(a.(string), b.(string))
	}
}
