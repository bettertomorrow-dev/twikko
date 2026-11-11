package schema

import (
	"strings"
	"testing"
)

func TestControlKindAutoAndOverrides(t *testing.T) {
	fieldOf := func(t FieldType) Field { return Field{Name: "f", Type: t} }
	for name, tc := range map[string]struct {
		widget Widget
		field  Field
		want   WidgetType
	}{
		"text auto":            {Widget{Type: WidgetControl}, fieldOf(Text), WidgetInput},
		"longtext auto":        {Widget{Type: WidgetControl}, fieldOf(LongText), WidgetTextarea},
		"number auto":          {Widget{Type: WidgetControl}, fieldOf(Number), WidgetNumber},
		"bool auto":            {Widget{Type: WidgetControl}, fieldOf(Bool), WidgetToggle},
		"enum auto":            {Widget{Type: WidgetControl}, fieldOf(Enum), WidgetSelect},
		"multienum auto":       {Widget{Type: WidgetControl}, fieldOf(MultiEnum), WidgetMultiSelect},
		"date auto":            {Widget{Type: WidgetControl}, fieldOf(Date), WidgetDate},
		"time auto":            {Widget{Type: WidgetControl}, fieldOf(Time), WidgetTime},
		"datetime auto":        {Widget{Type: WidgetControl}, fieldOf(DateTime), WidgetDateTime},
		"text as textarea":     {Widget{Type: WidgetTextarea}, fieldOf(Text), WidgetTextarea},
		"longtext as input":    {Widget{Type: WidgetInput}, fieldOf(LongText), WidgetInput},
		"text as select":       {Widget{Type: WidgetSelect, Options: []string{"a"}}, fieldOf(Text), WidgetSelect},
		"enum as select":       {Widget{Type: WidgetSelect}, fieldOf(Enum), WidgetSelect},
		"explicit matches own": {Widget{Type: WidgetToggle}, fieldOf(Bool), WidgetToggle},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := ControlKind(tc.widget, tc.field)
			if err != nil || got != tc.want {
				t.Fatalf("got %q, %v, want %q", got, err, tc.want)
			}
		})
	}
}

func TestControlKindRejections(t *testing.T) {
	for name, tc := range map[string]struct {
		widget Widget
		field  Field
		want   string
	}{
		"number as textarea":        {Widget{Type: WidgetTextarea}, Field{Name: "n", Type: Number}, "can be edited with: number (not textarea)"},
		"bool as input":             {Widget{Type: WidgetInput}, Field{Name: "b", Type: Bool}, "can be edited with: toggle"},
		"date as time":              {Widget{Type: WidgetTime}, Field{Name: "d", Type: Date}, "can be edited with: date"},
		"text select needs options": {Widget{Type: WidgetSelect}, Field{Name: "t", Type: Text}, "needs options"},
		"options on a text input":   {Widget{Type: WidgetInput, Options: []string{"a"}}, Field{Name: "t", Type: Text}, "options apply only to a select"},
		"options on an enum select": {Widget{Type: WidgetSelect, Options: []string{"a"}}, Field{Name: "e", Type: Enum, Options: []string{"a"}}, "options apply only to a select"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := ControlKind(tc.widget, tc.field)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want error containing %q, got %v", tc.want, err)
			}
		})
	}
}

const controlSchema = `
  fields:
    - {name: key, type: text, readonly: true}
    - {name: notes, type: text}
    - {name: team, type: text}
    - {name: n, type: number}
    - {name: blocked, type: bool}
    - {name: at, type: time}
    - {name: when, type: datetime}
    - {name: status, type: enum, options: [To Do, Done]}
`

func parseControls(layout, entities string) (*Document, error) {
	return Parse([]byte("schema:\n  id_field: key\n" + layout + controlSchema + "entities:\n" + entities))
}

const controlEntities = "  - {key: A-1, notes: hi, team: Backend, n: 3, blocked: false, at: '14:30', when: 2026-10-20T14:30, status: To Do}\n"

func TestHeaderTakesControls(t *testing.T) {
	doc, err := parseControls(`  header:
    - {type: tag, source: key}
    - {source: status}
    - {type: toggle, source: blocked, label: Blocked}
`, controlEntities)
	if err != nil {
		t.Fatal(err)
	}
	if got := doc.Schema.Header.Widgets[1].Type; got != WidgetControl {
		t.Errorf("an omitted type must mean control in the header too: %q", got)
	}
}

func TestSelectOptionsBecomeTheFieldsChoices(t *testing.T) {
	doc, err := parseControls("  body:\n    - - {type: select, source: team, options: [Backend, Frontend]}\n    - - {source: notes}\n    - - {source: n}\n    - - {source: blocked}\n    - - {source: at}\n    - - {source: when}\n    - - {source: status}\n", controlEntities)
	if err != nil {
		t.Fatal(err)
	}
	var team Field
	for _, field := range doc.Schema.Fields {
		if field.Name == "team" {
			team = field
		}
	}
	if _, err := ParseFormValue(team, []string{"Ops"}, true); err == nil || !strings.Contains(err.Error(), "must be one of: Backend, Frontend") {
		t.Errorf("a value outside the options must be refused: %v", err)
	}
	if got, err := ParseFormValue(team, []string{"Frontend"}, true); err != nil || got != "Frontend" {
		t.Errorf("an offered value must be accepted: %v %v", got, err)
	}
	if got, err := ParseFormValue(team, nil, false); err != nil || got != "" {
		t.Errorf("empty must stay allowed when not required: %v %v", got, err)
	}
}

func TestEntityValueOutsideSelectOptionsFailsTheLoad(t *testing.T) {
	_, err := parseControls("  header:\n    - {type: select, source: team, options: [Frontend]}\n", controlEntities)
	if err == nil || !strings.Contains(err.Error(), `field "team": must be one of: Frontend`) {
		t.Fatalf("got %v", err)
	}
}

func TestControlPlacementRules(t *testing.T) {
	for name, tc := range map[string]struct {
		layout string
		want   string
	}{
		"same field in header and body":  {"  header:\n    - {source: status}\n  body:\n    - - {source: status}\n    - - {source: notes}\n", `body row 1 cell 1: field "status" is already placed by header widget 0`},
		"same field twice in the header": {"  header:\n    - {source: status}\n    - {source: status}\n", "already placed"},
		"control on an undeclared field": {"  header:\n    - {source: nope}\n", "not a field declared"},
		"mismatched control":             {"  header:\n    - {type: textarea, source: n}\n", "can be edited with: number"},
		"control with style":             {"  header:\n    - {source: notes, style: {bold: true}}\n", "a control cannot have variants or style"},
		"options on a display widget":    {"  header:\n    - {type: text, source: key, options: [a]}\n", "options apply only"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := parseControls(tc.layout, controlEntities)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want error containing %q, got %v", tc.want, err)
			}
		})
	}
}

// A header control counts as placing its field, so a body may leave it out.
func TestHeaderControlSatisfiesTheRequiredFieldRule(t *testing.T) {
	_, err := Parse([]byte(`
schema:
  id_field: key
  header: [{source: summary}]
  body:
    - - {type: text, source: key}
  fields:
    - {name: key, type: text, readonly: true}
    - {name: summary, type: text, required: true}
entities: [{key: A-1, summary: x}]
`))
	if err != nil {
		t.Fatal(err)
	}
}

func TestTimeAndDateTimeValues(t *testing.T) {
	timeField := Field{Name: "at", Type: Time}
	dateTimeField := Field{Name: "when", Type: DateTime, Required: true}
	for _, tc := range []struct {
		field Field
		value string
		ok    bool
	}{
		{timeField, "14:30", true},
		{timeField, "14:30:15", true},
		{timeField, "", true},
		{timeField, "25:99", false},
		{timeField, "2:30pm", false},
		{dateTimeField, "2026-10-20T14:30", true},
		{dateTimeField, "2026-10-20T14:30:15", true},
		{dateTimeField, "yesterday", false},
		{dateTimeField, "2026-10-20T14:30:00Z", false},
		{dateTimeField, "", false},
	} {
		_, err := ParseFormValue(tc.field, []string{tc.value}, true)
		if (err == nil) != tc.ok {
			t.Errorf("%s %q: ok=%v err=%v", tc.field.Type, tc.value, tc.ok, err)
		}
	}
}
