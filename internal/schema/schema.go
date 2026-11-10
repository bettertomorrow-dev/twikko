package schema

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"
)

type FieldType string

const (
	Text      FieldType = "text"
	LongText  FieldType = "longtext"
	Number    FieldType = "number"
	Bool      FieldType = "bool"
	Enum      FieldType = "enum"
	MultiEnum FieldType = "multienum"
	Date      FieldType = "date"
)

type Field struct {
	Name     string    `yaml:"name"`
	Label    string    `yaml:"label,omitempty"`
	Type     FieldType `yaml:"type"`
	Required bool      `yaml:"required,omitempty"`
	Readonly bool      `yaml:"readonly,omitempty"`
	Options  []string  `yaml:"options,omitempty"`
}

type Schema struct {
	IDField string `yaml:"id_field"`
	// TitleField names an entity key whose value is the card title. The key
	// need not be declared in Fields. Cards fall back to the identifier when
	// it is unset or an entity lacks it.
	TitleField string  `yaml:"title_field,omitempty"`
	Fields     []Field `yaml:"fields"`
}

type Document struct {
	Schema   Schema
	Entities []*yaml.Node
}

func Load(path string) (*Document, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read input: %w", err)
	}
	return Parse(data)
}

func Parse(data []byte) (*Document, error) {
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("parse YAML: %w", err)
	}
	if len(root.Content) != 1 || root.Content[0].Kind != yaml.MappingNode {
		return nil, errors.New("input must be a YAML mapping")
	}
	doc := root.Content[0]
	schemaNode := MapValue(doc, "schema")
	entitiesNode := MapValue(doc, "entities")
	if schemaNode == nil || entitiesNode == nil {
		return nil, errors.New("input requires schema and entities")
	}
	var s Schema
	if err := schemaNode.Decode(&s); err != nil {
		return nil, fmt.Errorf("decode schema: %w", err)
	}
	if err := ValidateSchema(s); err != nil {
		return nil, err
	}
	if entitiesNode.Kind != yaml.SequenceNode {
		return nil, errors.New("entities must be a sequence")
	}
	entities := make([]*yaml.Node, len(entitiesNode.Content))
	for i, entity := range entitiesNode.Content {
		if entity.Kind != yaml.MappingNode {
			return nil, fmt.Errorf("entity %d must be a mapping", i)
		}
		if MapValue(entity, s.IDField) == nil {
			return nil, fmt.Errorf("entity %d is missing identifier %q", i, s.IDField)
		}
		if s.TitleField != "" {
			if title := MapValue(entity, s.TitleField); title != nil && title.Kind != yaml.ScalarNode {
				return nil, fmt.Errorf("entity %d title field %q must be a scalar", i, s.TitleField)
			}
		}
		for _, field := range s.Fields {
			if value := MapValue(entity, field.Name); value != nil {
				if _, err := ParseNode(field, value); err != nil {
					return nil, fmt.Errorf("entity %d field %q: %w", i, field.Name, err)
				}
			}
		}
		entities[i] = Clone(entity)
	}
	return &Document{Schema: s, Entities: entities}, nil
}

func ValidateSchema(s Schema) error {
	if s.IDField == "" {
		return errors.New("schema.id_field is required")
	}
	if len(s.Fields) == 0 {
		return errors.New("schema.fields is required")
	}
	seen := make(map[string]bool)
	idFound := false
	for _, f := range s.Fields {
		if f.Name == "" {
			return errors.New("every schema field requires a name")
		}
		if seen[f.Name] {
			return fmt.Errorf("schema field %q appears more than once", f.Name)
		}
		seen[f.Name] = true
		if f.Name == s.IDField {
			idFound = true
		}
		switch f.Type {
		case Text, LongText, Number, Bool, Enum, MultiEnum, Date:
		default:
			return fmt.Errorf("field %q has unsupported type %q", f.Name, f.Type)
		}
		if (f.Type == Enum || f.Type == MultiEnum) && len(f.Options) == 0 {
			return fmt.Errorf("field %q requires options", f.Name)
		}
		if f.Type != Enum && f.Type != MultiEnum && len(f.Options) != 0 {
			return fmt.Errorf("field %q cannot have options", f.Name)
		}
	}
	if !idFound {
		return fmt.Errorf("schema.id_field %q is not declared in schema.fields", s.IDField)
	}
	return nil
}

func ParseFormValue(field Field, values []string, present bool) (any, error) {
	if field.Type == Bool && !present {
		return false, nil
	}
	if len(values) == 0 {
		if field.Required {
			return nil, errors.New("is required")
		}
		if field.Type == MultiEnum {
			return []string{}, nil
		}
		return "", nil
	}
	switch field.Type {
	case Text, LongText:
		return values[0], nil
	case Number:
		if values[0] == "" {
			if field.Required {
				return nil, errors.New("is required")
			}
			return "", nil
		}
		n, err := strconv.ParseFloat(values[0], 64)
		if err != nil {
			return nil, errors.New("must be a number")
		}
		return n, nil
	case Bool:
		return values[0] == "on" || values[0] == "true", nil
	case Date:
		if values[0] == "" {
			if field.Required {
				return nil, errors.New("is required")
			}
			return "", nil
		}
		if _, err := time.Parse("2006-01-02", values[0]); err != nil {
			return nil, errors.New("must be a valid date")
		}
		return values[0], nil
	case Enum:
		if !contains(field.Options, values[0]) {
			return nil, fmt.Errorf("must be one of: %s", strings.Join(field.Options, ", "))
		}
		return values[0], nil
	case MultiEnum:
		for _, value := range values {
			if !contains(field.Options, value) {
				return nil, fmt.Errorf("contains invalid option %q", value)
			}
		}
		return append([]string(nil), values...), nil
	default:
		return nil, fmt.Errorf("unsupported field type %q", field.Type)
	}
}

func ParseNode(field Field, node *yaml.Node) (any, error) {
	switch field.Type {
	case MultiEnum:
		if node.Kind != yaml.SequenceNode {
			return nil, errors.New("must be a sequence")
		}
		values := make([]string, len(node.Content))
		for i, value := range node.Content {
			values[i] = value.Value
		}
		return ParseFormValue(field, values, true)
	case Bool:
		if node.Tag != "!!bool" {
			return nil, errors.New("must be a boolean")
		}
		return ParseFormValue(field, []string{node.Value}, true)
	default:
		return ParseFormValue(field, []string{node.Value}, true)
	}
}

func MapValue(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}

func SetMapValue(node *yaml.Node, key string, value *yaml.Node) {
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			node.Content[i+1] = value
			return
		}
	}
	node.Content = append(node.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, value)
}

func Clone(node *yaml.Node) *yaml.Node {
	if node == nil {
		return nil
	}
	copy := *node
	copy.Content = make([]*yaml.Node, len(node.Content))
	for i, child := range node.Content {
		copy.Content[i] = Clone(child)
	}
	copy.Alias = Clone(node.Alias)
	return &copy
}

func contains(values []string, value string) bool {
	for _, option := range values {
		if option == value {
			return true
		}
	}
	return false
}
