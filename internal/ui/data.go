package ui

import (
	"fmt"
	"strings"

	"github.com/a-h/templ"

	"go.yaml.in/yaml/v3"
	"multieditor/internal/icons"
	"multieditor/internal/schema"
	"multieditor/internal/session"
)

// Shared control styling, modelled on shadcn/ui. A read-only field renders as a
// disabled control, muted but still readable.
const (
	inputClass = "w-full rounded-md border border-input bg-transparent px-3 py-1.5 text-sm shadow-xs outline-none " +
		"focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50 " +
		"disabled:cursor-not-allowed disabled:bg-muted disabled:text-muted-foreground " +
		"disabled:[-webkit-text-fill-color:var(--muted-foreground)]"
	outlineButtonClass = "rounded-md border bg-background px-4 py-2 text-sm font-medium shadow-xs hover:bg-accent hover:text-accent-foreground"
	primaryButtonClass = "rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground shadow-xs hover:bg-primary/90 " +
		"disabled:cursor-not-allowed disabled:opacity-50"
)

// Header widget styling. Classes are spelled out in full, because Tailwind finds
// them by scanning source files at build time and cannot expand a name built
// from a YAML value. A theme token backs every intent.
var (
	iconIntentClass = map[schema.Intent]string{
		schema.IntentNeutral: "text-muted-foreground",
		schema.IntentInfo:    "text-info-foreground",
		schema.IntentSuccess: "text-success-foreground",
		schema.IntentWarning: "text-warning-foreground",
		schema.IntentDanger:  "text-destructive",
	}
	tagIntentClass = map[schema.Intent]string{
		schema.IntentNeutral: "bg-secondary text-secondary-foreground",
		schema.IntentInfo:    "bg-info/10 text-info-foreground",
		schema.IntentSuccess: "bg-success/10 text-success-foreground",
		schema.IntentWarning: "bg-warning/15 text-warning-foreground",
		schema.IntentDanger:  "bg-destructive/10 text-destructive",
	}
)

const (
	iconWidgetClass = "inline-flex shrink-0 items-center "
	tagWidgetClass  = "inline-flex shrink-0 items-center rounded-md px-2 py-0.5 text-xs font-medium "
	textWidgetClass = "min-w-0 truncate text-sm "
)

type FieldView struct {
	Field   schema.Field
	Value   any
	Error   string
	Changed bool
}

type EntityView struct {
	Index        int
	ID           string
	Title        string
	Collapsed    bool
	ChangedCount int
	HasErrors    bool
	Fields       []FieldView
	// HasHeader is true when the schema configures header widgets. Widgets
	// then replaces the plain title.
	HasHeader bool
	Widgets   []WidgetView
}

// WidgetView is a header widget resolved against one entity's current values.
type WidgetView struct {
	Type schema.WidgetType
	// Text is the tag or text content, and the accessible name of an icon.
	Text  string
	Icon  string
	Class string
}

type PageData struct {
	Token     string
	AssetBase string
	Entities  []EntityView
	Changes   int
	HasErrors bool
}

func NewPageData(store *session.Store, token string) PageData {
	s := store.Schema()
	data := PageData{
		Token:     token,
		AssetBase: "/s/" + token + "/assets/",
		Changes:   store.ChangedCount(),
		HasErrors: store.HasErrors(),
		Entities:  make([]EntityView, store.EntityCount()),
	}
	for index := range data.Entities {
		entity := store.Entity(index)
		id := nodeText(schema.MapValue(entity, s.IDField))
		title := id
		if s.TitleField != "" {
			if custom := nodeText(schema.MapValue(entity, s.TitleField)); custom != "" {
				title = custom
			}
		}
		data.Entities[index] = EntityView{
			Index:     index,
			ID:        id,
			Title:     title,
			Collapsed: store.Collapsed(index),
			Fields:    make([]FieldView, len(s.Fields)),
		}
		if s.Header != nil {
			view := &data.Entities[index]
			view.HasHeader = true
			view.Widgets = ResolveWidgets(s.Header, entity)
			view.Title = headerTitle(view.Widgets, id)
		}
		for fieldIndex, field := range s.Fields {
			data.Entities[index].Fields[fieldIndex] = FieldView{
				Field:   field,
				Value:   store.FieldValue(index, field),
				Error:   store.Error(index, field.Name),
				Changed: store.IsChanged(index, field.Name),
			}
			view := data.Entities[index].Fields[fieldIndex]
			if view.Changed {
				data.Entities[index].ChangedCount++
			}
			if view.Error != "" {
				data.Entities[index].HasErrors = true
			}
		}
	}
	return data
}

// ResolveWidgets reads each widget's source from the entity and returns what to
// draw. A widget whose source has no value is left out, so it leaves no gap. An
// icon or tag takes its look from the variant that matches the value exactly;
// an icon with no matching variant gets the fallback icon.
func ResolveWidgets(header *schema.Header, entity *yaml.Node) []WidgetView {
	views := make([]WidgetView, 0, len(header.Widgets))
	for _, widget := range header.Widgets {
		value, ok, err := schema.HeaderValue(entity, widget.Source)
		if err != nil || !ok {
			continue
		}
		variant, matched := widget.Variants[value]
		intent := variant.Intent
		if intent == "" {
			intent = schema.IntentNeutral
		}
		view := WidgetView{Type: widget.Type, Text: value}
		switch widget.Type {
		case schema.WidgetIcon:
			view.Icon = icons.Fallback
			if matched && variant.Icon != "" {
				view.Icon = variant.Icon
			}
			view.Class = iconWidgetClass + iconIntentClass[intent]
		case schema.WidgetTag:
			view.Class = tagWidgetClass + tagIntentClass[intent]
		default:
			view.Class = textWidgetClass + textStyleClass(widget.Style)
		}
		views = append(views, view)
	}
	return views
}

func textStyleClass(style schema.Style) string {
	classes := "font-normal"
	if style.Bold {
		classes = "font-semibold"
	}
	if style.Italic {
		classes += " italic"
	}
	if style.Muted {
		classes += " text-muted-foreground"
	}
	return classes
}

// headerTitle is the plain-text name of a card for screen readers and button
// labels: the first text widget, or the identifier when there is none.
func headerTitle(widgets []WidgetView, id string) string {
	for _, widget := range widgets {
		if widget.Type == schema.WidgetText {
			return widget.Text
		}
	}
	return id
}

func (data PageData) Entity(index int) EntityView {
	return data.Entities[index]
}

func Contains(values any, value string) bool {
	items, ok := values.([]string)
	if !ok {
		return false
	}
	for _, item := range items {
		if item == value {
			return true
		}
	}
	return false
}

func String(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case float64:
		return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%f", typed), "0"), ".")
	case bool:
		if typed {
			return "true"
		}
		return "false"
	default:
		return ""
	}
}

func inputType(fieldType schema.FieldType) string {
	switch fieldType {
	case schema.Number:
		return "number"
	case schema.Date:
		return "date"
	default:
		return "text"
	}
}

func nodeText(node *yaml.Node) string {
	if node == nil {
		return ""
	}
	return node.Value
}

func collapseLabel(entity EntityView) string {
	if entity.Collapsed {
		return "Expand " + entity.Title
	}
	return "Collapse " + entity.Title
}

func oobAttrs(oob bool) templ.Attributes {
	if oob {
		return templ.Attributes{"hx-swap-oob": "true"}
	}
	return nil
}
