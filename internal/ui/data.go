package ui

import (
	"fmt"
	"strings"

	"github.com/a-h/templ"

	"go.yaml.in/yaml/v3"
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

func saveBarAttrs(oob bool) templ.Attributes {
	if oob {
		return templ.Attributes{"hx-swap-oob": "true"}
	}
	return nil
}
