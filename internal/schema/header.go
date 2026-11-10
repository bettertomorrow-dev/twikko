package schema

import (
	"embed"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"
	"multieditor/internal/icons"
)

// WidgetType is the kind of piece a card header can show.
type WidgetType string

const (
	WidgetIcon WidgetType = "icon"
	WidgetTag  WidgetType = "tag"
	WidgetText WidgetType = "text"
)

// Intent is the colour role of an icon or tag. Each maps to a theme token.
type Intent string

const (
	IntentNeutral Intent = "neutral"
	IntentInfo    Intent = "info"
	IntentSuccess Intent = "success"
	IntentWarning Intent = "warning"
	IntentDanger  Intent = "danger"
)

// Variant is how an icon or tag looks for one data value. An icon variant may
// name an icon; a tag variant only sets the intent.
type Variant struct {
	Icon   string `yaml:"icon,omitempty"`
	Intent Intent `yaml:"intent,omitempty"`
}

// Style is the closed set of text styles. Tailwind needs the classes spelled
// out at build time, so a style cannot be an arbitrary CSS value.
type Style struct {
	Bold   bool `yaml:"bold,omitempty"`
	Italic bool `yaml:"italic,omitempty"`
	Muted  bool `yaml:"muted,omitempty"`
}

// Widget is one piece of the card header. Source is a key, or a dotted path
// into nested mappings, of the entity.
type Widget struct {
	Type     WidgetType         `yaml:"type"`
	Source   string             `yaml:"source"`
	Variants map[string]Variant `yaml:"variants,omitempty"`
	Style    Style              `yaml:"style,omitempty"`
}

// Header is the list of widgets between the eye toggle and the card menu. In
// YAML it is either that list or a mapping naming a built-in template.
type Header struct {
	Template string
	Widgets  []Widget
}

//go:embed headers/*.yaml
var templates embed.FS

var templateName = regexp.MustCompile(`^[a-z0-9-]+$`)

// UnmarshalYAML accepts a widget list, or {template: <name>}.
func (h *Header) UnmarshalYAML(node *yaml.Node) error {
	switch node.Kind {
	case yaml.SequenceNode:
		widgets, err := decodeWidgets(node)
		if err != nil {
			return err
		}
		*h = Header{Widgets: widgets}
		return nil
	case yaml.MappingNode:
		if err := checkKeys(node, "schema.header", "template"); err != nil {
			return err
		}
		name := MapValue(node, "template")
		if name == nil || name.Kind != yaml.ScalarNode || name.Value == "" {
			return errors.New("schema.header needs a list of widgets or a template name")
		}
		widgets, err := loadTemplate(name.Value)
		if err != nil {
			return err
		}
		*h = Header{Template: strings.TrimSuffix(name.Value, ".yaml"), Widgets: widgets}
		return nil
	default:
		return errors.New("schema.header must be a list of widgets or a mapping with template")
	}
}

// TemplateNames lists the built-in header templates.
func TemplateNames() []string {
	entries, _ := templates.ReadDir("headers")
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, strings.TrimSuffix(entry.Name(), ".yaml"))
	}
	sort.Strings(names)
	return names
}

func loadTemplate(name string) ([]Widget, error) {
	base := strings.TrimSuffix(name, ".yaml")
	var data []byte
	var err error
	if templateName.MatchString(base) {
		data, err = templates.ReadFile("headers/" + base + ".yaml")
	}
	if data == nil || err != nil {
		return nil, fmt.Errorf("unknown header template %q (available: %s)", name, strings.Join(TemplateNames(), ", "))
	}
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil || len(root.Content) != 1 || root.Content[0].Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("header template %q is not a widget list", base)
	}
	widgets, err := decodeWidgets(root.Content[0])
	if err != nil {
		return nil, fmt.Errorf("header template %q: %w", base, err)
	}
	return widgets, nil
}

func decodeWidgets(node *yaml.Node) ([]Widget, error) {
	widgets := make([]Widget, len(node.Content))
	for i, item := range node.Content {
		where := fmt.Sprintf("header widget %d", i)
		if item.Kind != yaml.MappingNode {
			return nil, fmt.Errorf("%s must be a mapping", where)
		}
		if err := checkKeys(item, where, "type", "source", "variants", "style"); err != nil {
			return nil, err
		}
		if styleNode := MapValue(item, "style"); styleNode != nil {
			if err := checkKeys(styleNode, where+" style", "bold", "italic", "muted"); err != nil {
				return nil, err
			}
		}
		if variants := MapValue(item, "variants"); variants != nil {
			if variants.Kind != yaml.MappingNode {
				return nil, fmt.Errorf("%s variants must be a mapping", where)
			}
			for j := 0; j+1 < len(variants.Content); j += 2 {
				name := "variant " + variants.Content[j].Value
				if err := checkKeys(variants.Content[j+1], where+" "+name, "icon", "intent"); err != nil {
					return nil, err
				}
			}
		}
		if err := item.Decode(&widgets[i]); err != nil {
			return nil, fmt.Errorf("%s: %w", where, err)
		}
	}
	return widgets, nil
}

// checkKeys rejects a mapping with a key outside allowed, so a typo such as
// "soure" fails at load instead of silently doing nothing.
func checkKeys(node *yaml.Node, where string, allowed ...string) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("%s must be a mapping", where)
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		key := node.Content[i].Value
		known := false
		for _, name := range allowed {
			if key == name {
				known = true
			}
		}
		if !known {
			return fmt.Errorf("%s has unknown key %q (allowed: %s)", where, key, strings.Join(allowed, ", "))
		}
	}
	return nil
}

// ValidateHeader checks the parts of a header that do not depend on entities.
func ValidateHeader(h *Header) error {
	if len(h.Widgets) == 0 {
		return errors.New("schema.header has no widgets")
	}
	for i := range h.Widgets {
		w := &h.Widgets[i]
		where := fmt.Sprintf("header widget %d", i)
		switch w.Type {
		case WidgetIcon, WidgetTag, WidgetText:
		default:
			return fmt.Errorf("%s has unsupported type %q (use icon, tag, or text)", where, w.Type)
		}
		if err := validSource(w.Source); err != nil {
			return fmt.Errorf("%s: %w", where, err)
		}
		if w.Type == WidgetText && len(w.Variants) != 0 {
			return fmt.Errorf("%s: text widgets cannot have variants", where)
		}
		if w.Type != WidgetText && w.Style != (Style{}) {
			return fmt.Errorf("%s: style applies only to text widgets", where)
		}
		for value, variant := range w.Variants {
			if value == "" {
				return fmt.Errorf("%s: a variant needs a non-empty value", where)
			}
			switch variant.Intent {
			case "":
				variant.Intent = IntentNeutral
			case IntentNeutral, IntentInfo, IntentSuccess, IntentWarning, IntentDanger:
			default:
				return fmt.Errorf("%s variant %q has unsupported intent %q (use neutral, info, success, warning, or danger)", where, value, variant.Intent)
			}
			if variant.Icon != "" {
				if w.Type != WidgetIcon {
					return fmt.Errorf("%s variant %q: only icon widgets can name an icon", where, value)
				}
				if !icons.Has(variant.Icon) {
					return fmt.Errorf("%s variant %q: unknown icon %q (available: %s)", where, value, variant.Icon, strings.Join(icons.Names(), ", "))
				}
			}
			w.Variants[value] = variant
		}
	}
	return nil
}

func validSource(source string) error {
	if source == "" {
		return errors.New("source is required")
	}
	for _, part := range strings.Split(source, ".") {
		if part == "" {
			return fmt.Errorf("source %q has an empty path segment", source)
		}
	}
	return nil
}

// HeaderValue returns the scalar a widget source points at in an entity. ok is
// false when the entity has no value there, and the error is set when the
// value is not a scalar.
func HeaderValue(entity *yaml.Node, source string) (value string, ok bool, err error) {
	node := MapValue(entity, source)
	if node == nil && strings.Contains(source, ".") {
		node = entity
		for _, part := range strings.Split(source, ".") {
			node = MapValue(deref(node), part)
			if node == nil {
				break
			}
		}
	}
	node = deref(node)
	switch {
	case node == nil:
		return "", false, nil
	case node.Kind != yaml.ScalarNode:
		return "", false, fmt.Errorf("source %q must point at a scalar", source)
	case node.Tag == "!!null" || node.Value == "":
		return "", false, nil
	}
	return node.Value, true, nil
}

func deref(node *yaml.Node) *yaml.Node {
	for node != nil && node.Kind == yaml.AliasNode {
		node = node.Alias
	}
	return node
}
