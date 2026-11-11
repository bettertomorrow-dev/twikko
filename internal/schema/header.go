package schema

import (
	"embed"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Header is the list of widgets between the eye toggle and the card menu. In
// YAML it is either that list or a mapping naming a built-in template.
type Header struct {
	Template string
	Widgets  []Widget
}

//go:embed headers/*.yaml bodies/*.yaml
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
func TemplateNames() []string { return templateNames("headers") }

func templateNames(dir string) []string {
	entries, _ := templates.ReadDir(dir)
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, strings.TrimSuffix(entry.Name(), ".yaml"))
	}
	sort.Strings(names)
	return names
}

// readTemplate returns the YAML list inside a built-in template file. kind
// ("header" or "body") only words the errors. The name is checked against a
// strict pattern, so it cannot reach outside dir.
func readTemplate(kind, dir, name string) (*yaml.Node, error) {
	base := strings.TrimSuffix(name, ".yaml")
	var data []byte
	var err error
	if templateName.MatchString(base) {
		data, err = templates.ReadFile(dir + "/" + base + ".yaml")
	}
	if data == nil || err != nil {
		return nil, fmt.Errorf("unknown %s template %q (available: %s)", kind, name, strings.Join(templateNames(dir), ", "))
	}
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil || len(root.Content) != 1 || root.Content[0].Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("%s template %q is not a list", kind, base)
	}
	return root.Content[0], nil
}

func loadTemplate(name string) ([]Widget, error) {
	list, err := readTemplate("header", "headers", name)
	if err != nil {
		return nil, err
	}
	widgets, err := decodeWidgets(list)
	if err != nil {
		return nil, fmt.Errorf("header template %q: %w", strings.TrimSuffix(name, ".yaml"), err)
	}
	return widgets, nil
}

func decodeWidgets(node *yaml.Node) ([]Widget, error) {
	widgets := make([]Widget, len(node.Content))
	for i, item := range node.Content {
		w, err := decodeWidget(item, fmt.Sprintf("header widget %d", i))
		if err != nil {
			return nil, err
		}
		widgets[i] = w
	}
	return widgets, nil
}

// ValidateHeader checks a header against the declared fields.
func ValidateHeader(h *Header, fields []Field) error {
	if len(h.Widgets) == 0 {
		return errors.New("schema.header has no widgets")
	}
	declared := fieldMap(fields)
	for i := range h.Widgets {
		where := fmt.Sprintf("header widget %d", i)
		if h.Widgets[i].Type == WidgetHLine {
			return fmt.Errorf("%s: a header is one row, so it takes vline, not hline", where)
		}
		if err := validateWidget(where, &h.Widgets[i], declared); err != nil {
			return err
		}
	}
	return checkVLines("header widget", h.Widgets)
}

func fieldMap(fields []Field) map[string]Field {
	declared := make(map[string]Field, len(fields))
	for _, field := range fields {
		declared[field.Name] = field
	}
	return declared
}
