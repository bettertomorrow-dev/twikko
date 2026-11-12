package ui

import (
	"strconv"
	"strings"
	"testing"

	"multieditor/internal/icons"
	"multieditor/internal/schema"
	"multieditor/internal/session"
)

func TestNumberDisplayPreservesPrecision(t *testing.T) {
	for _, value := range []float64{0.0000001, 1.23456789, -0.0000001, 0, 42} {
		text := String(value)
		got, err := strconv.ParseFloat(text, 64)
		if err != nil || got != value {
			t.Errorf("%v displayed as %q", value, text)
		}
	}
}

const testFields = `
  fields:
    - {name: key, type: text, readonly: true}
    - {name: summary, type: longtext}
    - {name: notes, type: text}
    - {name: team, type: text}
    - {name: points, type: number}
    - {name: blocked, type: bool}
    - {name: due, type: date, label: Due on}
    - {name: at, type: time}
    - {name: when, type: datetime}
    - {name: status, type: enum, options: [To Do, Done]}
    - {name: tags, type: multienum, options: [a, b]}
`

func pageData(t *testing.T, layout, entities string) PageData {
	t.Helper()
	doc, err := schema.Parse([]byte("schema:\n  id_field: key\n" + layout + testFields + "entities:\n" + entities))
	if err != nil {
		t.Fatal(err)
	}
	return NewPageData(session.New(doc), "tok")
}

func header(t *testing.T, widgets, entity string) []WidgetView {
	t.Helper()
	return pageData(t, "  header:\n"+widgets+"\n  body:\n    - - {source: summary}\n    - - {source: notes}\n    - - {source: team}\n    - - {source: points}\n    - - {source: blocked}\n    - - {source: due}\n    - - {source: at}\n    - - {source: when}\n    - - {source: status}\n    - - {source: tags}\n", "  - "+entity+"\n").Entities[0].Widgets
}

func TestResolveIconUsesMatchingVariant(t *testing.T) {
	views := header(t, `    - {type: icon, source: type, variants: {Bug: {icon: bug, intent: danger}}}`, `{key: A-1, type: Bug}`)
	if len(views) != 1 || views[0].Icon != "bug" || views[0].Text != "Bug" || !views[0].HasValue {
		t.Fatalf("unexpected views: %#v", views)
	}
	if !strings.Contains(views[0].Class, "text-destructive") {
		t.Errorf("danger icon must use the destructive colour: %q", views[0].Class)
	}
}

func TestResolveIconFallsBackWhenNoVariantMatches(t *testing.T) {
	views := header(t, `    - {type: icon, source: type, variants: {Bug: {icon: bug}}}`, `{key: A-1, type: Mystery}`)
	if views[0].Icon != icons.Fallback || !strings.Contains(views[0].Class, "text-muted-foreground") {
		t.Fatalf("an unmatched value must draw the neutral fallback icon: %#v", views)
	}
}

func TestResolveIconVariantWithIntentOnlyKeepsFallbackIcon(t *testing.T) {
	views := header(t, `    - {type: icon, source: type, variants: {Bug: {intent: warning}}}`, `{key: A-1, type: Bug}`)
	if views[0].Icon != icons.Fallback || !strings.Contains(views[0].Class, "text-warning-foreground") {
		t.Fatalf("unexpected views: %#v", views)
	}
}

// A header display widget with no value stays in the list as an empty slot,
// which collapses in the page and can fill in again after an edit.
func TestResolveMarksMissingAndEmptyValues(t *testing.T) {
	data := pageData(t, "  header:\n    - {type: tag, source: key}\n    - {type: tag, source: extra}\n", "  - {key: A-1, extra: x}\n  - {key: A-2}\n  - {key: A-3, extra: ''}\n")
	for i, want := range []bool{true, false, false} {
		widgets := data.Entities[i].Widgets
		if len(widgets) != 2 || !widgets[0].HasValue || widgets[1].HasValue != want {
			t.Errorf("entity %d: %#v", i, widgets)
		}
	}
	if data.Entities[0].Widgets[1].ID != "widget-0-h-1" {
		t.Errorf("slot id: %q", data.Entities[0].Widgets[1].ID)
	}
}

func TestResolveTagIntentAndTextStyle(t *testing.T) {
	views := header(t, `    - {type: tag, source: status, variants: {Done: {intent: success}}}
    - {type: tag, source: owner}
    - {type: text, source: title, style: {bold: true, italic: true, muted: true}}
    - {type: text, source: key}`, `{key: A-1, status: Done, owner: Dana, title: Fix login}`)
	if len(views) != 4 {
		t.Fatalf("unexpected views: %#v", views)
	}
	if !strings.Contains(views[0].Class, "bg-success/10") || !strings.Contains(views[1].Class, "bg-secondary") {
		t.Errorf("tag intents: %q, %q", views[0].Class, views[1].Class)
	}
	for _, want := range []string{"font-semibold", "italic", "text-muted-foreground", "truncate"} {
		if !strings.Contains(views[2].Class, want) {
			t.Errorf("styled text lacks %q: %q", want, views[2].Class)
		}
	}
	if strings.Contains(views[3].Class, "font-semibold") || strings.Contains(views[3].Class, "italic") {
		t.Errorf("unstyled text must stay plain: %q", views[3].Class)
	}
}

func TestHeaderTitleIsFirstTextWidgetWithAValueOrID(t *testing.T) {
	views := []WidgetView{
		{Type: schema.WidgetTag, Text: "A-1", HasValue: true},
		{Type: schema.WidgetText},
		{Type: schema.WidgetText, Text: "Fix login", HasValue: true},
	}
	if got := headerTitle(views, "A-1"); got != "Fix login" {
		t.Errorf("got %q", got)
	}
	if got := headerTitle(views[:2], "A-1"); got != "A-1" {
		t.Errorf("without a text value the title is the identifier, got %q", got)
	}
}

const bodyEntities = "  - {key: A-1, summary: Fix, due: 2026-10-20, status: To Do}\n  - {key: A-2, summary: Add, status: Done}\n"

// bodyOf lays out the given rows and then places every other editable field in
// a trailing row, which a required-field check or the limits might want.
func bodyOf(t *testing.T, rows string) PageData {
	t.Helper()
	return pageData(t, "  body:\n"+rows, bodyEntities)
}

func TestBodyRowsFollowTheLayout(t *testing.T) {
	data := bodyOf(t, `    - - {type: text, source: summary, label: What}
    - - {source: due, label: Due date}
      - {source: status}
      - {type: tag, source: status}
`)
	entity := data.Entities[0]
	if !entity.HasBody || len(entity.Rows) != 2 || len(entity.Grid) != 0 {
		t.Fatalf("unexpected rows: %#v", entity.Rows)
	}
	first := entity.Rows[0].Widgets[0]
	if first.Field != nil || !first.HasValue || first.Text != "Fix" || first.Label != "What" {
		t.Errorf("display widget: %#v", first)
	}
	if got := entity.Rows[1].Widgets[0].Field; got == nil || got.Field.Label != "Due date" {
		t.Errorf("a widget label must replace the field label: %#v", got)
	}
	if got := entity.Rows[1].Widgets[1].Field; got == nil || got.Field.Label != "status" {
		t.Errorf("an unlabelled control keeps the field label: %#v", got)
	}
	if entity.Rows[0].Widgets[0].ID != "widget-0-0-0" || data.Entities[1].Rows[0].Widgets[0].ID != "widget-1-0-0" {
		t.Errorf("slot ids must be unique per card")
	}
}

func TestBodyRowClassMatchesColumnCount(t *testing.T) {
	data := bodyOf(t, `    - - {source: status}
    - - {source: due}
      - {type: text, source: summary}
    - - {type: text, source: key}
      - {type: text, source: summary}
      - {type: text, source: due}
    - - {type: text, source: key}
      - {type: text, source: summary}
      - {type: text, source: due}
      - {type: text, source: status}
`)
	for i, want := range []string{"grid-cols-1", "md:grid-cols-2", "md:grid-cols-3", "lg:grid-cols-4"} {
		if got := data.Entities[0].Rows[i].Class; !strings.Contains(got, want) {
			t.Errorf("row %d class %q lacks %q", i, got, want)
		}
	}
}

func TestBodyBlankDisplayWidgetKeepsItsSlot(t *testing.T) {
	data := bodyOf(t, `    - - {type: text, source: summary}
      - {type: text, source: due, label: Due}
`)
	row := data.Entities[1].Rows[0]
	if len(row.Widgets) != 2 || !row.Widgets[0].HasValue || row.Widgets[1].HasValue {
		t.Fatalf("the widget without a value must stay as an empty slot: %#v", row.Widgets)
	}
}

func TestBodyTextWrapsWhileHeaderTextTruncates(t *testing.T) {
	data := bodyOf(t, "    - - {type: text, source: summary}\n")
	got := data.Entities[0].Rows[0].Widgets[0].Class
	if !strings.Contains(got, "whitespace-pre-line") || strings.Contains(got, "truncate") {
		t.Errorf("a body text widget must wrap: %q", got)
	}
}

func TestAutoControlFollowsTheFieldType(t *testing.T) {
	data := pageData(t, "", bodyEntities)
	want := map[string]schema.WidgetType{
		"key": schema.WidgetInput, "summary": schema.WidgetTextarea, "notes": schema.WidgetInput,
		"points": schema.WidgetNumber, "blocked": schema.WidgetToggle, "due": schema.WidgetDate,
		"at": schema.WidgetTime, "when": schema.WidgetDateTime, "status": schema.WidgetSelect,
		"tags": schema.WidgetMultiSelect,
	}
	grid := data.Entities[0].Grid
	if len(grid) != len(data.Entities[0].Fields) {
		t.Fatalf("the default grid must hold every field: %d of %d", len(grid), len(data.Entities[0].Fields))
	}
	for _, widget := range grid {
		if got := want[widget.Field.Field.Name]; widget.Type != got && widget.Field.Field.Name != "team" {
			t.Errorf("%s: got %q, want %q", widget.Field.Field.Name, widget.Type, got)
		}
	}
}

func TestOverrideAndLabelOfControls(t *testing.T) {
	data := bodyOf(t, `    - - {type: textarea, source: notes}
      - {type: select, source: team, options: [A, B]}
    - - {type: toggle, source: blocked, label: Blocked}
`)
	row := data.Entities[0].Rows
	if row[0].Widgets[0].Type != schema.WidgetTextarea || row[0].Widgets[1].Type != schema.WidgetSelect {
		t.Errorf("overrides must win over the auto control: %#v", row[0].Widgets)
	}
	if got := choices(row[0].Widgets[1].Field.Field); len(got) != 2 || got[0] != "A" {
		t.Errorf("a select over a text field offers the widget options: %v", got)
	}
	toggle := row[1].Widgets[0]
	if toggle.Type != schema.WidgetToggle || toggle.Label != "Blocked" || toggle.Field.Field.Label != "blocked" {
		t.Errorf("a toggle keeps its label as visible text and the field label as its name: %#v", toggle)
	}
}

func TestHeaderControlsAreOmittedFromTheDefaultGrid(t *testing.T) {
	data := pageData(t, "  header:\n    - {type: tag, source: key}\n    - {source: status}\n", bodyEntities)
	entity := data.Entities[0]
	if len(entity.Widgets) != 2 || entity.Widgets[1].Field == nil || entity.Widgets[1].Type != schema.WidgetSelect {
		t.Fatalf("header: %#v", entity.Widgets)
	}
	if !entity.Widgets[1].Field.Compact {
		t.Error("a header control uses the compact status line")
	}
	for _, widget := range entity.Grid {
		if widget.Field.Field.Name == "status" {
			t.Error("the status control must not appear twice")
		}
	}
	if len(entity.Grid) != len(entity.Fields)-1 {
		t.Errorf("grid has %d controls for %d fields", len(entity.Grid), len(entity.Fields))
	}
}

func TestEntityWithoutLayoutHasOnlyTheGrid(t *testing.T) {
	entity := pageData(t, "", bodyEntities).Entities[0]
	if entity.HasBody || entity.HasHeader || entity.Rows != nil || entity.Widgets != nil {
		t.Errorf("unexpected layout: %#v", entity)
	}
}

const lineBody = `  body:
    - - {source: summary}
      - {type: vline, intent: danger}
      - {source: notes}
    - - {type: hline}
    - - {source: team}
      - {type: vline}
      - {source: points}
      - {type: vline}
      - {source: blocked}
      - {type: vline}
      - {source: due}
    - - {source: at}
      - {source: when}
    - - {source: status}
    - - {source: tags}
`

func TestBodyLinesTakeOnlyTheirOwnTrack(t *testing.T) {
	rows := pageData(t, lineBody, "  - {key: A-1}\n").Entities[0].Rows
	if len(rows) != 6 {
		t.Fatalf("want 6 rows, got %d", len(rows))
	}
	if rows[0].Style != "--cols:minmax(0,1fr) auto minmax(0,1fr)" || !strings.Contains(rows[0].Class, "md:grid-cols-(--cols)") {
		t.Errorf("a vline gets an auto track between two equal ones: %q %q", rows[0].Style, rows[0].Class)
	}
	if rows[1].Class != "" || len(rows[1].Widgets) != 1 || rows[1].Widgets[0].Type != schema.WidgetHLine {
		t.Errorf("an hline is a row of its own with no grid: %#v", rows[1])
	}
	if !strings.Contains(rows[2].Class, "lg:grid-cols-(--cols)") || strings.Contains(rows[2].Class, "md:grid-cols") {
		t.Errorf("four widgets with vlines stack until lg: %q", rows[2].Class)
	}
	if rows[3].Style != "" || rows[3].Class != bodyRowClass[2] {
		t.Errorf("a row without vlines keeps its equal columns: %q %q", rows[3].Class, rows[3].Style)
	}
}

func TestLineWidgetClasses(t *testing.T) {
	rows := pageData(t, lineBody, "  - {key: A-1}\n").Entities[0].Rows
	vline, hline := rows[0].Widgets[1], rows[1].Widgets[0]
	if !vline.IsLine() || vline.Field != nil || !strings.Contains(vline.Class, "self-stretch border-l border-destructive") {
		t.Errorf("a vline stretches to its row and uses its intent colour: %#v", vline)
	}
	if !strings.Contains(hline.Class, "border-t border-border") || hline.Orientation() != "horizontal" || vline.Orientation() != "vertical" {
		t.Errorf("an hline is a neutral top border: %#v", hline)
	}
}

func TestHeaderLineIsNotAControl(t *testing.T) {
	data := pageData(t, "  header:\n    - {source: status}\n    - {type: vline}\n    - {type: text, source: summary}\n  body:\n    - - {source: summary}\n", "  - {key: A-1, summary: Fix}\n")
	widgets := data.Entities[0].Widgets
	if len(widgets) != 3 || !widgets[1].IsLine() || widgets[1].Field != nil || widgets[2].HasValue != true {
		t.Fatalf("unexpected header widgets: %#v", widgets)
	}
}
