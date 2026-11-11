package schema

import (
	"strings"
	"testing"
)

// parseBody parses an input whose schema carries the given body YAML (already
// indented under `schema:`). The fields cover the usual kinds.
func parseBody(body, entities string) (*Document, error) {
	return Parse([]byte(`
schema:
  id_field: key
` + body + `
  fields:
    - {name: key, type: text, readonly: true}
    - {name: summary, type: longtext}
    - {name: due, type: date, label: Due on}
    - {name: status, type: enum, options: [To Do, Done]}
    - {name: needed, type: text, required: true}
entities:
` + entities))
}

const bodyEntities = `
  - {key: A-1, summary: Fix, due: 2026-10-20, status: To Do, needed: x, owner: {name: Dana}}
  - {key: A-2, summary: Add, status: Done, needed: y}
`

const needed = `    - - {source: needed}
`

func TestBodyInlineRows(t *testing.T) {
	doc, err := parseBody(`
  body:
    - - {type: text, source: summary, label: What}
    - - {label: Due date, source: due}
      - {type: tag, source: status, variants: {Done: {intent: success}}}
      - {type: control, source: needed}
`, bodyEntities)
	if err != nil {
		t.Fatal(err)
	}
	body := doc.Schema.Body
	if body == nil || len(body.Rows) != 2 || len(body.Rows[0]) != 1 || len(body.Rows[1]) != 3 || body.Template != "" {
		t.Fatalf("unexpected body: %#v", body)
	}
	if cell := body.Rows[1][0]; cell.Type != WidgetControl || cell.Label != "Due date" || !cell.IsControl() {
		t.Errorf("an omitted type must mean field: %#v", cell)
	}
	if got := body.Rows[1][1].Variants["Done"].Intent; got != IntentSuccess {
		t.Errorf("variant intent lost: %q", got)
	}
}

func TestFieldLabelDefaultsToName(t *testing.T) {
	doc, err := parseBody("", bodyEntities)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range doc.Schema.Fields {
		if field.Label == "" {
			t.Errorf("field %q has no label", field.Name)
		}
	}
	if doc.Schema.Fields[2].Label != "Due on" {
		t.Errorf("an explicit label must stay: %q", doc.Schema.Fields[2].Label)
	}
}

func TestBodyTemplateByName(t *testing.T) {
	for _, name := range []string{"gojira", "gojira.yaml"} {
		doc, err := Parse([]byte(`
schema:
  id_field: key
  body: {template: ` + name + `}
  fields:
    - {name: key, type: text}
entities:
  - {key: A-1}
`))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if doc.Schema.Body.Template != "gojira" || len(doc.Schema.Body.Rows) == 0 {
			t.Errorf("%s: unexpected body: %#v", name, doc.Schema.Body)
		}
	}
}

// A broken template must fail here, not for a user who picks it. The fields it
// reads need not be declared, so only the layout itself is checked.
func TestEveryEmbeddedBodyTemplateIsValid(t *testing.T) {
	names := BodyTemplateNames()
	if len(names) == 0 {
		t.Fatal("no body templates are embedded")
	}
	for _, name := range names {
		var body Body
		node := mustNode(t, "template: "+name)
		if err := node.Decode(&body); err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if err := ValidateBody(&body, nil); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestBodyRejections(t *testing.T) {
	tooManyRows := "  body:\n" + strings.Repeat("    - - {type: text, source: summary}\n", MaxBodyRows+1)
	for name, tc := range map[string]struct {
		body string
		want string
	}{
		"too many rows":                  {tooManyRows, "at most 10"},
		"too many columns":               {"  body:\n    - - {source: needed}\n      - {type: text, source: summary}\n      - {type: text, source: summary}\n      - {type: text, source: summary}\n      - {type: text, source: summary}\n", "5 cells (at most 4)"},
		"empty row":                      {"  body:\n    - []\n    - - {source: needed}\n", "body row 1 is empty"},
		"empty body":                     {"  body: []", "has no rows"},
		"row that is not a list":         {"  body:\n    - {source: needed}\n", "must be a list of cells"},
		"body of the wrong kind":         {"  body: gojira", "must be a list of rows"},
		"unknown template lists choices": {"  body: {template: nope}", "available: gojira"},
		"template with a path":           {"  body: {template: ../headers/gojira}", "unknown body template"},
		"template with other keys":       {"  body: {template: gojira, extra: 1}", `unknown key "extra"`},
		"unknown cell type":              {"  body:\n" + needed + "    - - {type: chart, source: summary}\n", `unsupported type "chart"`},
		"typo in a key":                  {"  body:\n" + needed + "    - - {type: text, soure: summary}\n", `unknown key "soure"`},
		"field cell with unknown field":  {"  body:\n" + needed + "    - - {source: nope}\n", "not a field declared"},
		"field cell without source":      {"  body:\n" + needed + "    - - {label: x}\n", "source is required"},
		"field placed twice":             {"  body:\n" + needed + "    - - {source: due}\n      - {source: due}\n", `field "due" is already placed`},
		"variants on a field cell":       {"  body:\n" + needed + "    - - {source: due, variants: {a: {intent: info}}}\n", "cannot have variants or style"},
		"style on a field cell":          {"  body:\n" + needed + "    - - {source: due, style: {bold: true}}\n", "cannot have variants or style"},
		"style on a tag":                 {"  body:\n" + needed + "    - - {type: tag, source: status, style: {bold: true}}\n", "style applies only to text"},
		"unknown icon":                   {"  body:\n" + needed + "    - - {type: icon, source: status, variants: {Done: {icon: nope}}}\n", `unknown icon "nope"`},
		"display source with no entity":  {"  body:\n" + needed + "    - - {type: text, source: colour}\n", `source "colour" matches no entity`},
		"display source not a scalar":    {"  body:\n" + needed + "    - - {type: text, source: owner}\n", "must point at a scalar"},
		"hidden required field":          {"  body:\n    - - {type: text, source: summary}\n", `required field "needed" is not placed`},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := parseBody(tc.body, bodyEntities)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want error containing %q, got %v", tc.want, err)
			}
		})
	}
}

func TestBodyAllowsHidingOptionalAndReadonlyFields(t *testing.T) {
	if _, err := parseBody("  body:\n"+needed, bodyEntities); err != nil {
		t.Fatal(err)
	}
}

func TestRenamedFieldTypePointsAtControl(t *testing.T) {
	for name, header := range map[string]string{
		"header": "  header: [{type: field, source: key}]",
		"body":   "  body:\n    - - {type: field, source: key}",
	} {
		_, err := parseHeader(header, entitiesWithType)
		if err == nil || !strings.Contains(err.Error(), `unsupported type "field": it was renamed to "control"`) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// A template's optional sources, such as a due date, may be missing from every
// ticket in a result, which must not fail the load.
func TestBodyTemplateToleratesSourcesNoEntityHas(t *testing.T) {
	_, err := Parse([]byte(`
schema:
  id_field: key
  body: {template: gojira}
  fields:
    - {name: key, type: text}
entities:
  - {key: A-1, summary: only this}
`))
	if err != nil {
		t.Fatal(err)
	}
}

func TestBodyLinesDoNotCountAgainstLimits(t *testing.T) {
	// Four cells with three vlines, and ten rows with an hline between each.
	body := "  body:\n    - - {source: needed}\n      - {type: vline}\n      - {type: text, source: summary}\n      - {type: vline}\n      - {type: text, source: summary}\n      - {type: vline, intent: info}\n      - {type: text, source: summary}\n"
	for i := 1; i < MaxBodyRows; i++ {
		body += "    - - {type: hline}\n    - - {type: text, source: summary}\n"
	}
	doc, err := parseBody(body+"    - - {type: hline, intent: danger}\n", bodyEntities)
	if err != nil {
		t.Fatal(err)
	}
	if got := doc.Schema.Body.Rows[0][1].Intent; got != IntentNeutral {
		t.Errorf("a line without intent is neutral, got %q", got)
	}
	if got := doc.Schema.Body.Rows[0][5].Intent; got != IntentInfo {
		t.Errorf("a line keeps its intent, got %q", got)
	}
}

func TestBodyLineRejections(t *testing.T) {
	for name, tc := range map[string]struct {
		body string
		want string
	}{
		"hline with a cell beside it": {"  body:\n" + needed + "    - - {type: hline}\n      - {type: text, source: summary}\n", "an hline takes a row of its own"},
		"vline first":                 {"  body:\n    - - {type: vline}\n      - {source: needed}\n", "a vline must sit between two widgets"},
		"vline last":                  {"  body:\n    - - {source: needed}\n      - {type: vline}\n", "a vline must sit between two widgets"},
		"two vlines in a row":         {"  body:\n    - - {source: needed}\n      - {type: vline}\n      - {type: vline}\n      - {type: text, source: summary}\n", "a vline must sit between two widgets"},
		"five cells around vlines":    {"  body:\n    - - {source: needed}\n      - {type: vline}\n      - {type: text, source: summary}\n      - {type: text, source: summary}\n      - {type: text, source: summary}\n      - {type: text, source: summary}\n", "5 cells (at most 4)"},
		"only lines":                  {"  body:\n    - - {type: hline}\n", "has no rows"},
		"eleven rows beside hlines":   {"  body:\n    - - {type: hline}\n" + strings.Repeat("    - - {type: text, source: summary}\n", MaxBodyRows+1), "at most 10"},
		"line with a source":          {"  body:\n" + needed + "    - - {type: hline, source: summary}\n", "takes no source"},
		"line with a label":           {"  body:\n" + needed + "    - - {type: hline, label: x}\n", "takes no source"},
		"line with unknown intent":    {"  body:\n" + needed + "    - - {type: hline, intent: loud}\n", `unsupported intent "loud"`},
		"intent on a text widget":     {"  body:\n" + needed + "    - - {type: text, source: summary, intent: info}\n", "intent applies only to hline and vline"},
		"intent on a control":         {"  body:\n    - - {source: needed, intent: info}\n", "intent applies only to hline and vline"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := parseBody(tc.body, bodyEntities)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want error containing %q, got %v", tc.want, err)
			}
		})
	}
}

func TestHeaderLines(t *testing.T) {
	const fieldsAndBody = "  body:\n" + needed
	ok := "  header:\n    - {type: text, source: summary}\n    - {type: vline, intent: warning}\n    - {type: tag, source: status}\n"
	if _, err := parseBody(ok+fieldsAndBody, bodyEntities); err != nil {
		t.Fatal(err)
	}
	for name, tc := range map[string]struct{ header, want string }{
		"hline":      {"  header:\n    - {type: text, source: summary}\n    - {type: hline}\n", "takes vline, not hline"},
		"leading":    {"  header:\n    - {type: vline}\n    - {type: text, source: summary}\n", "a vline must sit between two widgets"},
		"trailing":   {"  header:\n    - {type: text, source: summary}\n    - {type: vline}\n", "a vline must sit between two widgets"},
		"with value": {"  header:\n    - {type: text, source: summary}\n    - {type: vline, source: x}\n    - {type: tag, source: status}\n", "takes no source"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := parseBody(tc.header+fieldsAndBody, bodyEntities)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want error containing %q, got %v", tc.want, err)
			}
		})
	}
}
