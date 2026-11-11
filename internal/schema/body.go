package schema

import (
	"errors"
	"fmt"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Limits of a body layout. Four columns is the most that stays readable in a
// card, and ten rows keeps a card scannable.
const (
	MaxBodyRows    = 10
	MaxBodyColumns = 4
)

// Body is the card body layout: rows of widgets. In YAML it is either that list
// of lists, or a mapping naming a built-in template. A schema field that no
// cell places is not shown.
type Body struct {
	Template string
	Rows     [][]Widget
}

// UnmarshalYAML accepts a list of rows, or {template: <name>}.
func (b *Body) UnmarshalYAML(node *yaml.Node) error {
	switch node.Kind {
	case yaml.SequenceNode:
		rows, err := decodeRows(node)
		if err != nil {
			return err
		}
		*b = Body{Rows: rows}
		return nil
	case yaml.MappingNode:
		if err := checkKeys(node, "schema.body", "template"); err != nil {
			return err
		}
		name := MapValue(node, "template")
		if name == nil || name.Kind != yaml.ScalarNode || name.Value == "" {
			return errors.New("schema.body needs a list of rows or a template name")
		}
		list, err := readTemplate("body", "bodies", name.Value)
		if err != nil {
			return err
		}
		rows, err := decodeRows(list)
		if err != nil {
			return fmt.Errorf("body template %q: %w", strings.TrimSuffix(name.Value, ".yaml"), err)
		}
		*b = Body{Template: strings.TrimSuffix(name.Value, ".yaml"), Rows: rows}
		return nil
	default:
		return errors.New("schema.body must be a list of rows or a mapping with template")
	}
}

// BodyTemplateNames lists the built-in body templates.
func BodyTemplateNames() []string { return templateNames("bodies") }

func decodeRows(node *yaml.Node) ([][]Widget, error) {
	rows := make([][]Widget, len(node.Content))
	for r, row := range node.Content {
		if row.Kind != yaml.SequenceNode {
			return nil, fmt.Errorf("body row %d must be a list of cells", r+1)
		}
		rows[r] = make([]Widget, len(row.Content))
		for c, item := range row.Content {
			widget, err := decodeWidget(item, fmt.Sprintf("body row %d cell %d", r+1, c+1))
			if err != nil {
				return nil, err
			}
			rows[r][c] = widget
		}
	}
	return rows, nil
}

// ValidateBody checks a body against the declared fields.
func ValidateBody(b *Body, fields []Field) error {
	if len(b.Rows) == 0 {
		return errors.New("schema.body has no rows")
	}
	if len(b.Rows) > MaxBodyRows {
		return fmt.Errorf("schema.body has %d rows (at most %d)", len(b.Rows), MaxBodyRows)
	}
	declared := fieldMap(fields)
	for r := range b.Rows {
		row := b.Rows[r]
		if len(row) == 0 {
			return fmt.Errorf("body row %d is empty", r+1)
		}
		if len(row) > MaxBodyColumns {
			return fmt.Errorf("body row %d has %d cells (at most %d)", r+1, len(row), MaxBodyColumns)
		}
		for c := range row {
			if err := validateWidget(fmt.Sprintf("body row %d cell %d", r+1, c+1), &row[c], declared); err != nil {
				return err
			}
		}
	}
	return nil
}
