package schema

import (
	"fmt"
	"time"

	"go.yaml.in/yaml/v3"
)

// RowHighlight applies an intent to a table row when When matches.
type RowHighlight struct {
	Intent Intent
	When   Condition
}

// Condition is a checked tree. A leaf compares one field or a measurement.
type Condition struct {
	All     []Condition
	Any     []Condition
	Field   string
	Op      string
	Value   any
	Measure ConditionMeasure
}

type ConditionMeasure struct {
	Length         bool
	DistanceFrom   string
	ReferenceField string
	Unit           string
}

// Relative reports whether evaluating this tree depends on the current time.
func (c Condition) Relative() bool {
	if c.Measure.DistanceFrom == "now" || c.Measure.DistanceFrom == "today" {
		return true
	}
	for _, children := range [][]Condition{c.All, c.Any} {
		for _, child := range children {
			if child.Relative() {
				return true
			}
		}
	}
	return false
}

func decodeTableHighlights(n *yaml.Node, table *Table, fields []Field) error {
	if zone := MapValue(n, "timezone"); zone != nil {
		text, err := conditionText(zone, "view.table.timezone")
		if err != nil {
			return err
		}
		location, err := time.LoadLocation(text)
		if err != nil || text == "" {
			return fmt.Errorf("view.table.timezone must name a valid time zone")
		}
		table.Timezone = text
		table.Location = location
	}
	rules := MapValue(n, "row_highlights")
	if rules == nil {
		return nil
	}
	if rules.Kind != yaml.SequenceNode || len(rules.Content) > 64 {
		return fmt.Errorf("view.table.row_highlights must be a list of at most 64 rules")
	}
	declared := fieldMap(fields)
	count := 0
	for i, rule := range rules.Content {
		path := fmt.Sprintf("view.table.row_highlights[%d]", i)
		if err := checkKeys(rule, path, "intent", "when"); err != nil {
			return err
		}
		intent, err := conditionText(MapValue(rule, "intent"), path+".intent")
		if err != nil {
			return err
		}
		switch Intent(intent) {
		case IntentNeutral, IntentInfo, IntentSuccess, IntentWarning, IntentDanger:
		default:
			return fmt.Errorf("%s.intent must be neutral, info, success, warning or danger", path)
		}
		c, err := decodeCondition(MapValue(rule, "when"), path+".when", declared, 0, &count)
		if err != nil {
			return err
		}
		table.RowHighlights = append(table.RowHighlights, RowHighlight{Intent: Intent(intent), When: c})
	}
	return nil
}

func decodeCondition(n *yaml.Node, path string, fields map[string]Field, depth int, count *int) (Condition, error) {
	var c Condition
	*count++
	if depth > 8 || *count > 256 {
		return c, fmt.Errorf("%s exceeds condition limits (depth 8, total 256 nodes)", path)
	}
	if n == nil {
		return c, fmt.Errorf("%s is required", path)
	}
	if err := checkKeys(n, path, "all", "any", "field", "op", "value", "measure"); err != nil {
		return c, err
	}
	for _, key := range []string{"all", "any"} {
		if group := MapValue(n, key); group != nil {
			if len(n.Content) != 2 || group.Kind != yaml.SequenceNode || len(group.Content) == 0 {
				return c, fmt.Errorf("%s.%s must be a nonempty list and the only condition key", path, key)
			}
			children := make([]Condition, len(group.Content))
			for i, child := range group.Content {
				var err error
				children[i], err = decodeCondition(child, fmt.Sprintf("%s.%s[%d]", path, key, i), fields, depth+1, count)
				if err != nil {
					return c, err
				}
			}
			if key == "all" {
				c.All = children
			} else {
				c.Any = children
			}
			return c, nil
		}
	}
	var err error
	c.Field, err = conditionText(MapValue(n, "field"), path+".field")
	if err != nil {
		return c, err
	}
	f, ok := fields[c.Field]
	if !ok {
		return c, fmt.Errorf("%s.field names undeclared field %q", path, c.Field)
	}
	c.Op, err = conditionText(MapValue(n, "op"), path+".op")
	if err != nil {
		return c, err
	}
	typ := f.Type
	if measure := MapValue(n, "measure"); measure != nil {
		if measure.Kind == yaml.ScalarNode && measure.Tag == "!!str" && measure.Value == "length" {
			if typ != Text && typ != LongText {
				return c, fmt.Errorf("%s.measure length requires a text field", path)
			}
			c.Measure.Length = true
		} else {
			if err := checkKeys(measure, path+".measure", "distance_from", "unit"); err != nil {
				return c, err
			}
			if typ != Date && typ != Time && typ != DateTime {
				return c, fmt.Errorf("%s.measure distance requires a temporal field", path)
			}
			ref := MapValue(measure, "distance_from")
			if ref != nil && ref.Kind == yaml.MappingNode {
				if err := checkKeys(ref, path+".measure.distance_from", "field"); err != nil {
					return c, err
				}
				c.Measure.ReferenceField, err = conditionText(MapValue(ref, "field"), path+".measure.distance_from.field")
				if err != nil {
					return c, err
				}
				if other, ok := fields[c.Measure.ReferenceField]; !ok || other.Type != typ {
					return c, fmt.Errorf("%s.measure.distance_from.field requires the same temporal type", path)
				}
			} else {
				c.Measure.DistanceFrom, err = conditionText(ref, path+".measure.distance_from")
				if err != nil {
					return c, err
				}
				if (typ == Date && c.Measure.DistanceFrom != "today") || (typ != Date && c.Measure.DistanceFrom != "now") {
					return c, fmt.Errorf("%s.measure.distance_from requires today for date or now for time/datetime", path)
				}
			}
			c.Measure.Unit, err = conditionText(MapValue(measure, "unit"), path+".measure.unit")
			if err != nil {
				return c, err
			}
			if c.Measure.Unit != "days" && c.Measure.Unit != "hours" && c.Measure.Unit != "minutes" {
				return c, fmt.Errorf("%s.measure.unit must be days, hours or minutes", path)
			}
			if typ == Date && c.Measure.Unit != "days" {
				return c, fmt.Errorf("%s.measure.unit must be days for date", path)
			}
		}
		typ = Number
	}
	value := MapValue(n, "value")
	switch typ {
	case Number:
		switch c.Op {
		case "even", "odd":
			if c.Measure.DistanceFrom != "" || c.Measure.ReferenceField != "" {
				return c, fmt.Errorf("%s.op parity requires a number or string length", path)
			}
			if value != nil {
				return c, fmt.Errorf("%s.value is not allowed for %s", path, c.Op)
			}
			return c, nil
		case "gt", "lt", "eq", "ne":
			c.Value, err = NodeDecimal(value)
			if err != nil {
				return c, fmt.Errorf("%s.value: %w", path, err)
			}
		default:
			return c, fmt.Errorf("%s.op: unsupported numeric operator %q", path, c.Op)
		}
	case Text, LongText:
		switch c.Op {
		case "eq", "ne", "contains", "not_contains":
			c.Value, err = conditionText(value, path+".value")
		default:
			return c, fmt.Errorf("%s.op: unsupported text operator %q", path, c.Op)
		}
	case Bool:
		if c.Op != "eq" && c.Op != "ne" {
			return c, fmt.Errorf("%s.op: boolean operator must be eq or ne", path)
		} else if value == nil || value.Kind != yaml.ScalarNode || value.Tag != "!!bool" {
			return c, fmt.Errorf("%s.value must be a boolean", path)
		} else {
			err = value.Decode(&c.Value)
		}
	case Enum, MultiEnum:
		if typ == Enum && (c.Op == "eq" || c.Op == "ne") {
			var text string
			text, err = conditionText(value, path+".value")
			if err == nil && !contains(f.Options, text) {
				err = fmt.Errorf("value must name a declared option")
			}
			c.Value = text
		} else if c.Op == "contains_all" || c.Op == "contains_any" {
			if value == nil || value.Kind != yaml.SequenceNode || len(value.Content) == 0 {
				err = fmt.Errorf("value must be a nonempty option list")
			} else {
				options := make([]string, len(value.Content))
				for i, option := range value.Content {
					options[i], err = conditionText(option, fmt.Sprintf("%s.value[%d]", path, i))
					if err != nil || !contains(f.Options, options[i]) {
						err = fmt.Errorf("value[%d] must name a declared option", i)
						break
					}
				}
				c.Value = options
			}
		} else {
			err = fmt.Errorf("unsupported enum operator %q", c.Op)
		}
	default:
		err = fmt.Errorf("temporal conditions require a distance measure")
	}
	if err != nil {
		return c, fmt.Errorf("%s: %w", path, err)
	}
	return c, nil
}

func conditionText(n *yaml.Node, path string) (string, error) {
	if n == nil || n.Kind != yaml.ScalarNode || n.Tag != "!!str" {
		return "", fmt.Errorf("%s must be text", path)
	}
	return n.Value, nil
}
