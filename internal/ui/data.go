package ui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/a-h/templ"

	"go.yaml.in/yaml/v3"
	"twikko/internal/icons"
	"twikko/internal/schema"
	"twikko/internal/session"
)

// Shared control styling, modelled on shadcn/ui. A read-only field renders as a
// disabled control, muted but still readable.
const (
	inputBaseClass = "rounded-md border border-input bg-transparent px-3 py-1.5 text-sm shadow-xs outline-none " +
		"focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50 " +
		"disabled:cursor-not-allowed disabled:bg-muted disabled:text-muted-foreground " +
		"disabled:[-webkit-text-fill-color:var(--muted-foreground)]"
	// A control fills its body cell, and stays narrow in the header row.
	inputBodyClass      = inputBaseClass + " w-full"
	inputHeaderClass    = inputBaseClass + " w-44 py-1"
	textareaBodyClass   = inputBodyClass + " min-h-28"
	textareaHeaderClass = inputHeaderClass + " min-h-16"
	outlineButtonClass  = "rounded-md border bg-background px-4 py-2 text-sm font-medium shadow-xs hover:bg-accent hover:text-accent-foreground"
	primaryButtonClass  = "rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground shadow-xs hover:bg-primary/90 " +
		"disabled:cursor-not-allowed disabled:opacity-50"

	// The on/off switch is a visually hidden checkbox, which keeps it a form
	// control, followed by a track whose thumb moves when it is checked.
	switchLabelClass = "inline-flex cursor-pointer items-center gap-2 has-[:disabled]:cursor-not-allowed has-[:disabled]:opacity-60"
	switchTrackClass = "relative h-5 w-9 shrink-0 rounded-full bg-input transition-colors " +
		"peer-checked:bg-primary peer-focus-visible:ring-[3px] peer-focus-visible:ring-ring/50 " +
		"after:absolute after:left-0.5 after:top-0.5 after:size-4 after:rounded-full after:bg-background " +
		"after:shadow-xs after:transition-transform peer-checked:after:translate-x-4"

	// A control with its marker for a changed value, in a body cell or the header.
	controlBodyClass   = "min-w-0 border-l-2 pl-3 "
	controlHeaderClass = "min-w-0 shrink-0 border-l-2 pl-2 "
	// A display slot of the header collapses when empty, so it leaves no gap. A
	// body slot keeps its place, so columns do not shift between cards.
	displayHeaderSlotClass = "flex min-w-0 items-center gap-1.5 empty:hidden"
	displayBodySlotClass   = "min-w-0 border-l-2 border-transparent pl-3"
)

// Display widget styling. Classes are spelled out in full, because Tailwind finds
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
	// A body text widget wraps and keeps line breaks, so a long digest stays readable.
	bodyTextWidgetClass = "min-w-0 whitespace-pre-line break-words text-sm "
)

// bodyRowClass maps a row's cell count to its grid. A row is hidden until one
// of its cells has content, so a row of blank display cells leaves no gap, and
// it shows again when an edit fills one. Rows stack to one column below md.
// The strings are spelled out in full for Tailwind.
var bodyRowClass = map[int]string{
	1: "hidden has-[>*>*]:grid gap-5 grid-cols-1",
	2: "hidden has-[>*>*]:grid gap-5 grid-cols-1 md:grid-cols-2",
	3: "hidden has-[>*>*]:grid gap-5 grid-cols-1 md:grid-cols-3",
	4: "hidden has-[>*>*]:grid gap-5 grid-cols-1 md:grid-cols-2 lg:grid-cols-4",
}

// bodyLineRowClass is the grid of a row with vlines, keyed by its widget count
// without them. Its columns come from the --cols property (see rowColumns): an
// equal share for a widget and just the line's own width for a vline. The
// vlines are hidden while the row is stacked. A row of four widgets stacks until
// lg, as it needs the room.
var bodyLineRowClass = map[int]string{
	2: "hidden has-[>*>*]:grid gap-5 grid-cols-1 [&>[role=separator]]:hidden md:grid-cols-(--cols) md:[&>[role=separator]]:block",
	3: "hidden has-[>*>*]:grid gap-5 grid-cols-1 [&>[role=separator]]:hidden md:grid-cols-(--cols) md:[&>[role=separator]]:block",
	4: "hidden has-[>*>*]:grid gap-5 grid-cols-1 [&>[role=separator]]:hidden lg:grid-cols-(--cols) lg:[&>[role=separator]]:block",
}

// Line widget styling. A line is one border wide, coloured by its intent. A
// vline stretches to the height of its row or of the header, and has no content
// of its own, so it never changes that height.
var lineIntentClass = map[schema.Intent]string{
	schema.IntentNeutral: "border-border",
	schema.IntentInfo:    "border-info",
	schema.IntentSuccess: "border-success",
	schema.IntentWarning: "border-warning",
	schema.IntentDanger:  "border-destructive",
}

const (
	hlineClass = "w-full border-t "
	vlineClass = "self-stretch border-l "
)

func lineClass(widget schema.Widget) string {
	intent := widget.Intent
	if intent == "" {
		intent = schema.IntentNeutral
	}
	if widget.Type == schema.WidgetHLine {
		return hlineClass + lineIntentClass[intent]
	}
	return vlineClass + lineIntentClass[intent]
}

// bodyRow describes how one body row lays out. Lines do not count as cells:
// an hline is a row of its own, and a vline takes the width of its border only.
func bodyRow(row []schema.Widget) (class, style string) {
	cells, vlines := 0, 0
	for _, widget := range row {
		switch widget.Type {
		case schema.WidgetHLine:
			return "", ""
		case schema.WidgetVLine:
			vlines++
		default:
			cells++
		}
	}
	if vlines == 0 {
		return bodyRowClass[cells], ""
	}
	tracks := make([]string, len(row))
	for i, widget := range row {
		tracks[i] = "minmax(0,1fr)"
		if widget.Type == schema.WidgetVLine {
			tracks[i] = "auto"
		}
	}
	return bodyLineRowClass[cells], "--cols:" + strings.Join(tracks, " ")
}

type FieldView struct {
	Field   schema.Field
	Value   any
	Error   string
	Changed bool
	// Compact is true for a field edited in the header, where the status line
	// shows an error only.
	Compact bool
}

type EntityView struct {
	Index        int
	ID           string
	Title        string
	Collapsed    bool
	ChangedCount int
	HasErrors    bool
	Fields       []FieldView
	// HasHeader is true when the schema configures header widgets. Widgets then
	// replaces the plain title.
	HasHeader bool
	Widgets   []WidgetView
	// HasBody is true when the schema lays the body out in rows. Rows then
	// replaces the grid.
	HasBody bool
	Rows    []RowView
	// Grid is the default body: a control for every field the header does not
	// place. It is empty when the schema has a body.
	Grid []WidgetView
}

// RowView is one row of the card body.
type RowView struct {
	Class string
	// Style sets the --cols property of a row with vlines.
	Style   string
	Widgets []WidgetView
}

// WidgetView is a header or body widget resolved against one entity's current
// values. A control carries the field it edits. A display widget carries what
// to draw, and has no value when its source is empty.
type WidgetView struct {
	// ID names a display slot, so an edit can refresh it out of band.
	ID string
	// Type is the display type, or for a control the kind it resolved to.
	Type schema.WidgetType
	// Label is a display caption or a toggle's visible text.
	Label string
	// Text is the tag or text content, and the accessible name of an icon.
	Text     string
	Icon     string
	Class    string
	HasValue bool
	Field    *FieldView
}

type PageData struct {
	Token       string
	AssetBase   string
	Title       string
	Description string
	Entities    []EntityView
	Changes     int
	HasErrors   bool
}

const defaultTitle = "Twikko"

func pageTitle(title string) string {
	if title == "" {
		return defaultTitle
	}
	return title
}

func NewPageData(store *session.Store, token string) PageData {
	s := store.Schema()
	data := PageData{
		Token:       token,
		Title:       pageTitle(store.Title()),
		Description: store.Description(),
		AssetBase:   "/s/" + token + "/assets/",
		Changes:     store.ChangedCount(),
		HasErrors:   store.HasErrors(),
		Entities:    make([]EntityView, store.EntityCount()),
	}
	inHeader := headerControls(s)
	for index := range data.Entities {
		entity := store.Entity(index)
		id := nodeText(schema.MapValue(entity, s.IDField))
		title := id
		if s.TitleField != "" {
			if custom := nodeText(schema.MapValue(entity, s.TitleField)); custom != "" {
				title = custom
			}
		}
		view := &data.Entities[index]
		*view = EntityView{
			Index:     index,
			ID:        id,
			Title:     title,
			Collapsed: store.Collapsed(index),
			Fields:    make([]FieldView, len(s.Fields)),
		}
		byName := make(map[string]*FieldView, len(s.Fields))
		for fieldIndex, field := range s.Fields {
			view.Fields[fieldIndex] = FieldView{
				Field:   field,
				Value:   store.FieldValue(index, field),
				Error:   store.Error(index, field.Name),
				Changed: store.IsChanged(index, field.Name),
				Compact: inHeader[field.Name],
			}
			fv := &view.Fields[fieldIndex]
			byName[field.Name] = fv
			if fv.Changed {
				view.ChangedCount++
			}
			if fv.Error != "" {
				view.HasErrors = true
			}
		}
		if s.Header != nil {
			view.HasHeader = true
			view.Widgets = resolveWidgets(s.Header.Widgets, entity, byName, func(i int) string {
				return fmt.Sprintf("widget-%d-h-%d", index, i)
			}, textWidgetClass)
			view.Title = headerTitle(view.Widgets, id)
		}
		switch {
		case s.Body != nil:
			view.HasBody = true
			view.Rows = make([]RowView, len(s.Body.Rows))
			for r, row := range s.Body.Rows {
				class, style := bodyRow(row)
				view.Rows[r] = RowView{
					Class: class,
					Style: style,
					Widgets: resolveWidgets(row, entity, byName, func(c int) string {
						return fmt.Sprintf("widget-%d-%d-%d", index, r, c)
					}, bodyTextWidgetClass),
				}
			}
		default:
			for i := range view.Fields {
				if !inHeader[view.Fields[i].Field.Name] {
					view.Grid = append(view.Grid, autoControl(&view.Fields[i]))
				}
			}
		}
	}
	return data
}

// headerControls lists the fields a header control edits. The default body
// leaves them out, so no field has two controls.
func headerControls(s schema.Schema) map[string]bool {
	placed := map[string]bool{}
	if s.Header != nil {
		for _, w := range s.Header.Widgets {
			if w.IsControl() {
				placed[w.Source] = true
			}
		}
	}
	return placed
}

// autoControl is the control the field's data type calls for.
func autoControl(field *FieldView) WidgetView {
	kind, _ := schema.ControlKind(schema.Widget{Type: schema.WidgetControl}, field.Field)
	return WidgetView{Type: kind, Field: field}
}

// resolveWidgets resolves a list of widgets for one entity. id names the display
// slot at a position. textClass is the base class of a text widget, which
// differs between the header (truncates) and the body (wraps).
func resolveWidgets(widgets []schema.Widget, entity *yaml.Node, fields map[string]*FieldView, id func(int) string, textClass string) []WidgetView {
	views := make([]WidgetView, len(widgets))
	for i, widget := range widgets {
		views[i] = resolveWidget(widget, entity, fields, textClass)
		views[i].ID = id(i)
	}
	return views
}

func resolveWidget(widget schema.Widget, entity *yaml.Node, fields map[string]*FieldView, textClass string) WidgetView {
	if widget.IsLine() {
		return WidgetView{Type: widget.Type, Class: lineClass(widget)}
	}
	if widget.IsControl() {
		return resolveControl(widget, fields[widget.Source])
	}
	view := WidgetView{Type: widget.Type, Label: widget.Label}
	value, ok, err := schema.HeaderValue(entity, widget.Source)
	if err != nil || !ok {
		return view
	}
	variant, matched := widget.Variants[value]
	intent := variant.Intent
	if intent == "" {
		intent = schema.IntentNeutral
	}
	view.Text, view.HasValue = value, true
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
		view.Class = textClass + textStyleClass(widget.Style)
	}
	return view
}

// resolveControl points a control widget at the field it edits. A label
// replaces the field label above a control. A toggle keeps it as visible text
// to its right, and the field label stays as its accessible name.
func resolveControl(widget schema.Widget, field *FieldView) WidgetView {
	kind, _ := schema.ControlKind(widget, field.Field)
	view := WidgetView{Type: kind, Field: field}
	if kind == schema.WidgetToggle {
		view.Label = widget.Label
	} else if widget.Label != "" {
		labelled := *field
		labelled.Field.Label = widget.Label
		view.Field = &labelled
	}
	return view
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
// labels: the first text widget that has a value, or the identifier.
func headerTitle(widgets []WidgetView, id string) string {
	for _, widget := range widgets {
		if widget.Type == schema.WidgetText && widget.HasValue {
			return widget.Text
		}
	}
	return id
}

// IsLine reports whether the widget is a separator line.
func (w WidgetView) IsLine() bool {
	return w.Type == schema.WidgetHLine || w.Type == schema.WidgetVLine
}

// Orientation is the ARIA orientation of a line.
func (w WidgetView) Orientation() string {
	if w.Type == schema.WidgetVLine {
		return "vertical"
	}
	return "horizontal"
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
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case bool:
		if typed {
			return "true"
		}
		return "false"
	default:
		return ""
	}
}

// inputType is the HTML input type of a single-line control.
func inputType(kind schema.WidgetType) string {
	switch kind {
	case schema.WidgetNumber:
		return "number"
	case schema.WidgetDate:
		return "date"
	case schema.WidgetTime:
		return "time"
	case schema.WidgetDateTime:
		return "datetime-local"
	default:
		return "text"
	}
}

// A picker input leaves room on the left for the button that opens its popover.
const pickerInputBaseClass = inputBaseClass + " pl-9 placeholder:text-muted-foreground aria-invalid:border-destructive"

func pickerInputClass(kind schema.WidgetType, header bool) string {
	switch {
	case !header:
		return pickerInputBaseClass + " w-full"
	case kind == schema.WidgetDateTime:
		return pickerInputBaseClass + " w-56 py-1"
	default:
		return pickerInputBaseClass + " w-44 py-1"
	}
}

func pickerIcon(kind schema.WidgetType) string {
	if kind == schema.WidgetTime {
		return "clock"
	}
	return "calendar"
}

func pickerButtonLabel(kind schema.WidgetType) string {
	if kind == schema.WidgetTime {
		return "Open time picker"
	}
	return "Open calendar"
}

// pickerPlaceholder is the mask of a picker input. static/app.js types digits
// into the same mask.
func pickerPlaceholder(kind schema.WidgetType) string {
	switch kind {
	case schema.WidgetTime:
		return "HH:MM"
	case schema.WidgetDateTime:
		return "YYYY-MM-DD HH:MM"
	default:
		return "YYYY-MM-DD"
	}
}

// pickerText is the text in a picker input: the value in the order of its mask.
// static/app.js writes a value it picks the same way, so the input does not
// change when it first renders. A value that does not parse stays as it is,
// next to its validation error.
func pickerText(widget WidgetView) string {
	value := String(widget.Field.Value)
	var layouts []string
	var out string
	switch widget.Type {
	case schema.WidgetTime:
		layouts, out = []string{"15:04", "15:04:05"}, "15:04"
	case schema.WidgetDateTime:
		layouts, out = []string{"2006-01-02T15:04", "2006-01-02T15:04:05"}, "2006-01-02 15:04"
	default:
		layouts, out = []string{"2006-01-02"}, "2006-01-02"
	}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed.Format(out)
		}
	}
	return value
}

// choices are the options a select offers: an enum's own, or the list a select
// gave a text field.
func choices(field schema.Field) []string {
	if field.Type == schema.Enum {
		return field.Options
	}
	return field.Choices
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

func fieldID(entityIndex int, name string) string {
	return fmt.Sprintf("field-%d-%s", entityIndex, name)
}

func inputClass(header bool) string {
	if header {
		return inputHeaderClass
	}
	return inputBodyClass
}

func textareaClass(header bool) string {
	if header {
		return textareaHeaderClass
	}
	return textareaBodyClass
}

func controlWrapperClass(header, changed bool) string {
	class := controlBodyClass
	if header {
		class = controlHeaderClass
	}
	if changed {
		return class + "border-changed"
	}
	return class + "border-transparent"
}

func displaySlotClass(header bool) string {
	if header {
		return displayHeaderSlotClass
	}
	return displayBodySlotClass
}

func statusClass(field FieldView) string {
	if field.Compact {
		return "text-xs"
	}
	return "mt-1 min-h-5 text-xs"
}

// controlAttrs wires a control to its edit endpoint. A control in the header
// has no label element, so its label becomes its accessible name and tooltip.
func controlAttrs(data PageData, entity EntityView, widget WidgetView, header bool, trigger string) templ.Attributes {
	name := widget.Field.Field.Name
	attrs := templ.Attributes{
		"hx-post":    fmt.Sprintf("/s/%s/entities/%d/fields/%s", data.Token, entity.Index, name),
		"hx-trigger": trigger,
		"hx-target":  fmt.Sprintf("#status-%d-%s", entity.Index, name),
		"hx-swap":    "outerHTML",
	}
	if header {
		attrs["aria-label"] = widget.Field.Field.Label
		attrs["title"] = widget.Field.Field.Label
	}
	return attrs
}
