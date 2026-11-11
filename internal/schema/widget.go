package schema

import (
	"errors"
	"fmt"
	"strings"

	"go.yaml.in/yaml/v3"
	"multieditor/internal/icons"
)

// WidgetType is the kind of piece a card header or body can show. The display
// types draw a value. The control types edit a schema field. `control` lets the
// field's data type pick the control, and naming one of the others overrides it.
type WidgetType string

const (
	WidgetIcon WidgetType = "icon"
	WidgetTag  WidgetType = "tag"
	WidgetText WidgetType = "text"

	WidgetControl     WidgetType = "control"
	WidgetToggle      WidgetType = "toggle"
	WidgetInput       WidgetType = "input"
	WidgetTextarea    WidgetType = "textarea"
	WidgetNumber      WidgetType = "number"
	WidgetDate        WidgetType = "date"
	WidgetTime        WidgetType = "time"
	WidgetDateTime    WidgetType = "datetime"
	WidgetSelect      WidgetType = "select"
	WidgetMultiSelect WidgetType = "multiselect"
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

// Widget is one piece of the card header or body. For a display widget Source
// is an entity key, or a dotted path into nested mappings. For a control it is
// the name of a declared field.
type Widget struct {
	Type   WidgetType `yaml:"type"`
	Source string     `yaml:"source"`
	// Label replaces a control's field label, captions a display widget in the
	// body, and prefixes one in the header. A toggle shows it to its right.
	Label string `yaml:"label,omitempty"`
	// Options are the choices of a select that overrides a text field.
	Options  []string           `yaml:"options,omitempty"`
	Variants map[string]Variant `yaml:"variants,omitempty"`
	Style    Style              `yaml:"style,omitempty"`
}

// IsControl reports whether the widget edits a schema field.
func (w Widget) IsControl() bool {
	switch w.Type {
	case WidgetIcon, WidgetTag, WidgetText:
		return false
	}
	return true
}

// controlKinds lists, per field type, the controls that can edit it. The first
// is the one `control` picks. An override only changes how the same kind of
// value is drawn, so a stored value is never reinterpreted.
var controlKinds = map[FieldType][]WidgetType{
	Text:      {WidgetInput, WidgetTextarea, WidgetSelect},
	LongText:  {WidgetTextarea, WidgetInput},
	Number:    {WidgetNumber},
	Bool:      {WidgetToggle},
	Enum:      {WidgetSelect},
	MultiEnum: {WidgetMultiSelect},
	Date:      {WidgetDate},
	Time:      {WidgetTime},
	DateTime:  {WidgetDateTime},
}

func joinTypes(types []WidgetType) string {
	names := make([]string, len(types))
	for i, t := range types {
		names[i] = string(t)
	}
	return strings.Join(names, ", ")
}

// ControlKind returns the control a control widget draws for a field: the
// field's automatic control for `control`, else the type the widget names. It
// fails when that control does not fit the field's data type, or when options
// are given where they do not apply.
func ControlKind(w Widget, field Field) (WidgetType, error) {
	allowed := controlKinds[field.Type]
	kind := w.Type
	if kind == WidgetControl {
		kind = allowed[0]
	} else {
		fits := false
		for _, candidate := range allowed {
			fits = fits || candidate == kind
		}
		if !fits {
			return "", fmt.Errorf("field %q is %s, which can be edited with: %s (not %s)", field.Name, field.Type, joinTypes(allowed), kind)
		}
	}
	overridesText := kind == WidgetSelect && field.Type == Text
	switch {
	case overridesText && len(w.Options) == 0:
		return "", fmt.Errorf("a select over text field %q needs options", field.Name)
	case !overridesText && len(w.Options) != 0:
		return "", errors.New("options apply only to a select that overrides a text field (an enum field has its own options)")
	}
	return kind, nil
}

const allWidgetTypes = "icon, tag, text, control, toggle, input, textarea, number, date, time, datetime, select, or multiselect"

// validateWidget checks one widget of the header or body against the declared
// fields. It also fills in the default intent of every variant.
func validateWidget(where string, w *Widget, fields map[string]Field) error {
	switch w.Type {
	case WidgetIcon, WidgetTag, WidgetText:
		return validateDisplay(where, w)
	case WidgetControl, WidgetToggle, WidgetInput, WidgetTextarea, WidgetNumber,
		WidgetDate, WidgetTime, WidgetDateTime, WidgetSelect, WidgetMultiSelect:
		return validateControl(where, w, fields)
	case "field":
		return fmt.Errorf("%s has unsupported type %q: it was renamed to %q", where, w.Type, WidgetControl)
	default:
		return fmt.Errorf("%s has unsupported type %q (use %s)", where, w.Type, allWidgetTypes)
	}
}

func validateControl(where string, w *Widget, fields map[string]Field) error {
	if w.Source == "" {
		return fmt.Errorf("%s: source is required", where)
	}
	field, ok := fields[w.Source]
	if !ok {
		return fmt.Errorf("%s: source %q is not a field declared in schema.fields (a widget that shows other entity data needs type text, tag, or icon)", where, w.Source)
	}
	if len(w.Variants) != 0 || w.Style != (Style{}) {
		return fmt.Errorf("%s: a control cannot have variants or style", where)
	}
	if _, err := ControlKind(*w, field); err != nil {
		return fmt.Errorf("%s: %w", where, err)
	}
	return nil
}

func validateDisplay(where string, w *Widget) error {
	if err := validSource(w.Source); err != nil {
		return fmt.Errorf("%s: %w", where, err)
	}
	if len(w.Options) != 0 {
		return fmt.Errorf("%s: options apply only to a select that overrides a text field", where)
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
	return nil
}

// decodeWidget reads one widget from YAML. Unknown keys fail, so a typo such as
// "soure" cannot silently do nothing. An omitted type means `control`.
func decodeWidget(item *yaml.Node, where string) (Widget, error) {
	var w Widget
	if err := checkKeys(item, where, "type", "source", "label", "options", "variants", "style"); err != nil {
		return w, err
	}
	if err := checkWidgetKeys(item, where); err != nil {
		return w, err
	}
	if err := item.Decode(&w); err != nil {
		return w, fmt.Errorf("%s: %w", where, err)
	}
	if w.Type == "" {
		w.Type = WidgetControl
	}
	return w, nil
}

// checkWidgetKeys rejects unknown keys inside a widget's style and variants.
func checkWidgetKeys(item *yaml.Node, where string) error {
	if styleNode := MapValue(item, "style"); styleNode != nil {
		if err := checkKeys(styleNode, where+" style", "bold", "italic", "muted"); err != nil {
			return err
		}
	}
	if variants := MapValue(item, "variants"); variants != nil {
		if variants.Kind != yaml.MappingNode {
			return fmt.Errorf("%s variants must be a mapping", where)
		}
		for j := 0; j+1 < len(variants.Content); j += 2 {
			name := "variant " + variants.Content[j].Value
			if err := checkKeys(variants.Content[j+1], where+" "+name, "icon", "intent"); err != nil {
				return err
			}
		}
	}
	return nil
}

// checkKeys rejects a mapping with a key outside allowed.
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

// HeaderValue returns the scalar a display widget source points at in an entity.
// ok is false when the entity has no value there, and the error is set when the
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
