package schema

import (
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

// parseHeader parses an input whose schema carries the given header YAML
// (already indented under `schema:`) and a few entities.
func parseHeader(header, entities string) (*Document, error) {
	return Parse([]byte(`
schema:
  id_field: key
` + header + `
  fields:
    - {name: key, type: text}
entities:
` + entities))
}

const entitiesWithType = `
  - {key: A-1, type: Bug, title: Fix login, owner: {name: Dana}}
  - {key: A-2, type: Task, title: Add logs}
`

func TestHeaderInlineWidgets(t *testing.T) {
	doc, err := parseHeader(`
  header:
    - type: icon
      source: type
      variants:
        Bug: {icon: bug, intent: danger}
        Task: {icon: checkbox}
    - {type: tag, source: key}
    - type: text
      source: title
      style: {bold: true, italic: false}
`, entitiesWithType)
	if err != nil {
		t.Fatal(err)
	}
	header := doc.Schema.Header
	if header == nil || len(header.Widgets) != 3 || header.Template != "" {
		t.Fatalf("unexpected header: %#v", header)
	}
	if got := header.Widgets[0].Variants["Task"].Intent; got != IntentNeutral {
		t.Errorf("a variant without intent must default to neutral, got %q", got)
	}
	if !header.Widgets[2].Style.Bold || header.Widgets[2].Style.Italic {
		t.Errorf("unexpected style: %#v", header.Widgets[2].Style)
	}
}

func TestHeaderTemplateByName(t *testing.T) {
	entities := `
  - {key: A-1, type: Bug, title: Fix login, status: Done}
`
	for _, name := range []string{"gojira", "gojira.yaml"} {
		doc, err := parseHeader("  header: {template: "+name+"}", entities)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if doc.Schema.Header.Template != "gojira" || len(doc.Schema.Header.Widgets) == 0 {
			t.Errorf("%s: unexpected header: %#v", name, doc.Schema.Header)
		}
	}
}

// A broken template must fail here, not for a user who picks it.
func TestEveryEmbeddedTemplateIsValid(t *testing.T) {
	names := TemplateNames()
	if len(names) == 0 {
		t.Fatal("no header templates are embedded")
	}
	for _, name := range names {
		var header Header
		if err := yaml.Unmarshal([]byte("template: "+name), &header); err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if err := ValidateHeader(&header, nil); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestHeaderRejections(t *testing.T) {
	for name, tc := range map[string]struct {
		header   string
		entities string
		want     string
	}{
		"unknown template lists the available ones": {"  header: {template: nope}", entitiesWithType, "available: gojira"},
		"template with a path":                      {"  header: {template: ../schema}", entitiesWithType, "unknown header template"},
		"template mixed with other keys":            {"  header: {template: gojira, extra: 1}", entitiesWithType, `unknown key "extra"`},
		"header of the wrong kind":                  {"  header: gojira", entitiesWithType, "must be a list"},
		"empty list":                                {"  header: []", entitiesWithType, "no widgets"},
		"unknown widget type":                       {"  header: [{type: chart, source: key}]", entitiesWithType, `unsupported type "chart"`},
		"missing source":                            {"  header: [{type: text}]", entitiesWithType, "source is required"},
		"typo in a key":                             {"  header: [{type: text, soure: key}]", entitiesWithType, `unknown key "soure"`},
		"empty path segment":                        {"  header: [{type: text, source: a..b}]", entitiesWithType, "empty path segment"},
		"unknown intent":                            {"  header: [{type: tag, source: key, variants: {A-1: {intent: loud}}}]", entitiesWithType, `unsupported intent "loud"`},
		"unknown icon":                              {"  header: [{type: icon, source: type, variants: {Bug: {icon: nope}}}]", entitiesWithType, `unknown icon "nope"`},
		"icon on a tag":                             {"  header: [{type: tag, source: type, variants: {Bug: {icon: bug}}}]", entitiesWithType, "only icon widgets"},
		"variants on text":                          {"  header: [{type: text, source: key, variants: {A-1: {intent: info}}}]", entitiesWithType, "text widgets cannot have variants"},
		"style on an icon":                          {"  header: [{type: icon, source: type, style: {bold: true}}]", entitiesWithType, "style applies only to text"},
		"unknown style key":                         {"  header: [{type: text, source: key, style: {huge: true}}]", entitiesWithType, `unknown key "huge"`},
		"title_field together with header":          {"  title_field: title\n  header: [{type: text, source: key}]", entitiesWithType, "cannot be combined"},
		"source pointing at a mapping":              {"  header: [{type: text, source: owner}]", entitiesWithType, "must point at a scalar"},
		"source found in no entity":                 {"  header: [{type: text, source: colour}]", entitiesWithType, `source "colour" matches no entity`},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := parseHeader(tc.header, tc.entities)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want error containing %q, got %v", tc.want, err)
			}
		})
	}
}

func TestHeaderValue(t *testing.T) {
	var root yaml.Node
	if err := yaml.Unmarshal([]byte(`{a: 1, b: {c: deep}, "d.e": direct, empty: "", none: ~, list: [x]}`), &root); err != nil {
		t.Fatal(err)
	}
	entity := root.Content[0]
	for source, want := range map[string]string{"a": "1", "b.c": "deep", "d.e": "direct"} {
		got, ok, err := HeaderValue(entity, source)
		if err != nil || !ok || got != want {
			t.Errorf("%s: got %q ok=%v err=%v, want %q", source, got, ok, err, want)
		}
	}
	for _, source := range []string{"missing", "b.missing", "empty", "none", "a.b"} {
		if got, ok, err := HeaderValue(entity, source); ok || err != nil {
			t.Errorf("%s: want no value, got %q ok=%v err=%v", source, got, ok, err)
		}
	}
	if _, _, err := HeaderValue(entity, "list"); err == nil {
		t.Error("a sequence must be rejected")
	}
}

// The gojira skill writes the entity keys the gojira header template reads. If
// either side renames a key, the header would go blank without an error, so
// this loads the skill's example input and checks every widget finds its data.
func TestGojiraSkillExampleFitsTheGojiraTemplate(t *testing.T) {
	doc, err := Load("../../skills/gojira/assets/example-input.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if doc.Schema.Header == nil || doc.Schema.Header.Template != "gojira" {
		t.Fatalf("the example must use the gojira template: %#v", doc.Schema.Header)
	}
	if len(doc.Schema.Header.Widgets) == 0 {
		t.Fatal("the gojira template has no widgets")
	}
	if doc.Schema.Body == nil || doc.Schema.Body.Template != "gojira" || len(doc.Schema.Body.Rows) == 0 {
		t.Fatalf("the example must use the gojira body template: %#v", doc.Schema.Body)
	}
	for _, shown := range doc.Schema.displayWidgets() {
		if !anyEntityHas(doc.Entities, shown.widget.Source) {
			t.Errorf("%s reads %q, which no example ticket has", shown.where, shown.widget.Source)
		}
	}
}

func mustNode(t *testing.T, doc string) *yaml.Node {
	t.Helper()
	var root yaml.Node
	if err := yaml.Unmarshal([]byte(doc), &root); err != nil {
		t.Fatal(err)
	}
	return root.Content[0]
}
