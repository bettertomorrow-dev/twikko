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
	Time      FieldType = "time"
	DateTime  FieldType = "datetime"
)

type Field struct {
	Name     string    `yaml:"name"`
	Label    string    `yaml:"label,omitempty"`
	Type     FieldType `yaml:"type"`
	Required bool      `yaml:"required,omitempty"`
	Readonly bool      `yaml:"readonly,omitempty"`
	Options  []string  `yaml:"options,omitempty"`
	// Choices limit a text field to a list. It is set from the options of a select
	// that overrides the field, so every way of editing it obeys the same list.
	Choices []string `yaml:"-"`
}

type Schema struct {
	IDField string `yaml:"id_field"`
	// TitleField names an entity key whose value is the card title. The key
	// need not be declared in Fields. Cards fall back to the identifier when
	// it is unset or an entity lacks it.
	TitleField string `yaml:"title_field,omitempty"`
	// Header replaces the title with widgets bound to entity data. It cannot be
	// combined with TitleField.
	Header *Header `yaml:"header,omitempty"`
	// Body lays the fields out in rows and columns. Fields it does not place are
	// not shown, but stay unchanged in the output.
	Body   *Body   `yaml:"body,omitempty"`
	Fields []Field `yaml:"fields"`
}

type Document struct {
	// Title and Description head the page. Both are optional.
	Title       string
	Description string
	Schema      Schema
	Entities    []*yaml.Node
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
	for i := range s.Fields {
		if s.Fields[i].Label == "" {
			s.Fields[i].Label = s.Fields[i].Name
		}
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
		for _, shown := range s.displayWidgets() {
			if _, _, err := HeaderValue(entity, shown.widget.Source); err != nil {
				return nil, fmt.Errorf("entity %d %s: %w", i, shown.where, err)
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
	if len(entities) > 0 {
		for _, shown := range s.displayWidgets() {
			if !shown.optional && !anyEntityHas(entities, shown.widget.Source) {
				return nil, fmt.Errorf("%s: source %q matches no entity", shown.where, shown.widget.Source)
			}
		}
	}
	title, err := optionalText(doc, "title")
	if err != nil {
		return nil, err
	}
	description, err := optionalText(doc, "description")
	if err != nil {
		return nil, err
	}
	return &Document{Title: title, Description: description, Schema: s, Entities: entities}, nil
}

// optionalText reads a top-level scalar, or "" when the key is absent.
func optionalText(doc *yaml.Node, key string) (string, error) {
	node := MapValue(doc, key)
	if node == nil {
		return "", nil
	}
	if node.Kind != yaml.ScalarNode {
		return "", fmt.Errorf("%s must be text", key)
	}
	return strings.TrimSpace(node.Value), nil
}

func anyEntityHas(entities []*yaml.Node, source string) bool {
	for _, entity := range entities {
		if _, ok, _ := HeaderValue(entity, source); ok {
			return true
		}
	}
	return false
}

func ValidateSchema(s Schema) error {
	if s.IDField == "" {
		return errors.New("schema.id_field is required")
	}
	if len(s.Fields) == 0 {
		return errors.New("schema.fields is required")
	}
	if s.Header != nil && s.TitleField != "" {
		return errors.New("schema.title_field cannot be combined with schema.header")
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
		case Text, LongText, Number, Bool, Enum, MultiEnum, Date, Time, DateTime:
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
	if s.Header != nil {
		if err := ValidateHeader(s.Header, s.Fields); err != nil {
			return err
		}
	}
	if s.Body != nil {
		if err := ValidateBody(s.Body, s.Fields); err != nil {
			return err
		}
	}
	return s.placeControls()
}

// locatedWidget is a widget with where it sits, for error messages.
type locatedWidget struct {
	where  string
	widget Widget
	// optional marks a widget of a built-in template, whose sources may be
	// missing from every entity of a given input.
	optional bool
}

func (s Schema) widgets() []locatedWidget {
	var all []locatedWidget
	if s.Header != nil {
		for i, w := range s.Header.Widgets {
			all = append(all, locatedWidget{where: fmt.Sprintf("header widget %d", i), widget: w})
		}
	}
	if s.Body != nil {
		for r, row := range s.Body.Rows {
			for c, w := range row {
				all = append(all, locatedWidget{
					where:    fmt.Sprintf("body row %d cell %d", r+1, c+1),
					widget:   w,
					optional: s.Body.Template != "",
				})
			}
		}
	}
	return all
}

// displayWidgets are the widgets that read entity data rather than edit a field.
func (s Schema) displayWidgets() []locatedWidget {
	var shown []locatedWidget
	for _, w := range s.widgets() {
		if !w.widget.IsControl() && !w.widget.IsLine() {
			shown = append(shown, w)
		}
	}
	return shown
}

// placeControls checks where controls sit. A field has at most one control
// across header and body, so two controls never clash on one id and a select's
// options can act as the field's own list of choices, which it then gets. A
// body hides the fields no control places, so it may not hide a required field
// the user could neither see nor fix.
func (s Schema) placeControls() error {
	placedAt := make(map[string]string)
	index := make(map[string]int, len(s.Fields))
	for i, f := range s.Fields {
		index[f.Name] = i
	}
	for _, shown := range s.widgets() {
		w := shown.widget
		if !w.IsControl() {
			continue
		}
		if at, ok := placedAt[w.Source]; ok {
			return fmt.Errorf("%s: field %q is already placed by %s, and a field has one control", shown.where, w.Source, at)
		}
		placedAt[w.Source] = shown.where
		if w.Type == WidgetSelect && s.Fields[index[w.Source]].Type == Text {
			s.Fields[index[w.Source]].Choices = w.Options
		}
	}
	if s.Body != nil {
		for _, f := range s.Fields {
			if _, ok := placedAt[f.Name]; f.Required && !f.Readonly && !ok {
				return fmt.Errorf("required field %q is not placed in schema.body, so it would be hidden", f.Name)
			}
		}
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
		if len(field.Choices) != 0 && values[0] != "" && !contains(field.Choices, values[0]) {
			return nil, fmt.Errorf("must be one of: %s", strings.Join(field.Choices, ", "))
		}
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
		return parseTemporal(field, values[0], "must be a valid date", "2006-01-02")
	case Time:
		return parseTemporal(field, values[0], "must be a time such as 14:30", "15:04", "15:04:05")
	case DateTime:
		return parseTemporal(field, values[0], "must be a date and time such as 2026-10-20T14:30", "2006-01-02T15:04", "2006-01-02T15:04:05")
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

func parseTemporal(field Field, value, message string, layouts ...string) (any, error) {
	if value == "" {
		if field.Required {
			return nil, errors.New("is required")
		}
		return "", nil
	}
	for _, layout := range layouts {
		if _, err := time.Parse(layout, value); err == nil {
			return value, nil
		}
	}
	return nil, errors.New(message)
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
