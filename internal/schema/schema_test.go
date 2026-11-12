package schema

import (
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

func TestValueValidationEdges(t *testing.T) {
	for _, tc := range []struct {
		field Field
		value string
		valid bool
	}{
		{Field{Type: Text, Required: true}, "", false},
		{Field{Type: LongText, Required: true}, "", false},
		{Field{Type: Text}, "", true},
		{Field{Type: Enum, Options: []string{"open"}}, "", true},
		{Field{Type: Enum, Required: true, Options: []string{"open"}}, "", false},
		{Field{Type: Number}, "NaN", false},
		{Field{Type: Number}, "+Inf", false},
		{Field{Type: Number}, "-Inf", false},
		{Field{Type: Number}, "0.0000001", true},
		{Field{Type: Bool}, "garbage", false},
		{Field{Type: Bool}, "true", true},
		{Field{Type: Bool}, "false", true},
		{Field{Type: Bool}, "on", true},
	} {
		_, err := ParseFormValue(tc.field, []string{tc.value}, true)
		if (err == nil) != tc.valid {
			t.Errorf("%s %q: valid=%v, error=%v", tc.field.Type, tc.value, tc.valid, err)
		}
	}
}

func TestNodeRejectsCollectionsAsScalars(t *testing.T) {
	for _, kind := range []FieldType{Text, LongText, Number, Enum, Date, Time, DateTime} {
		for _, value := range []string{"{}", "[]"} {
			if _, err := ParseNode(Field{Type: kind}, mustNode(t, value)); err == nil {
				t.Errorf("%s accepted %s", kind, value)
			}
		}
	}
	if _, err := ParseNode(Field{Type: MultiEnum, Options: []string{""}}, mustNode(t, "[{}]")); err == nil {
		t.Fatal("multienum accepted a mapping as an option")
	}
	entity := mustNode(t, "{source: &value hello, copy: *value}")
	if got, err := ParseNode(Field{Type: Text}, MapValue(entity, "copy")); err != nil || got != "hello" {
		t.Fatalf("scalar alias: %v, %v", got, err)
	}
}

func TestClonePreservesAliasGraph(t *testing.T) {
	entity := mustNode(t, "&record {key: PROJ-101, nested: [*record]}")
	copy := Clone(entity)
	alias := MapValue(copy, "nested").Content[0]
	if copy == entity || alias.Kind != yaml.AliasNode || alias.Alias != copy {
		t.Fatal("clone must preserve recursive aliases without keeping the original nodes")
	}
}

func TestYAMLBooleanSpellings(t *testing.T) {
	for text, want := range map[string]bool{"true": true, "True": true, "TRUE": true, "false": false, "False": false, "FALSE": false} {
		got, err := ParseNode(Field{Type: Bool}, mustNode(t, text))
		if err != nil || got != want {
			t.Errorf("%s: got %v, %v, want %v", text, got, err, want)
		}
	}
}

func TestParseAndValidateValues(t *testing.T) {
	input := []byte(`
schema:
  id_field: key
  fields:
    - {name: key, type: text}
    - {name: status, type: enum, options: [open, done]}
    - {name: labels, type: multienum, options: [bug, ui]}
    - {name: due, type: date}
entities:
  - {key: A-1, status: open, labels: [bug], due: 2026-10-13}
`)
	doc, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Entities) != 1 || doc.Schema.IDField != "key" {
		t.Fatalf("unexpected document: %#v", doc)
	}
	if _, err := ParseFormValue(doc.Schema.Fields[1], []string{"other"}, true); err == nil {
		t.Fatal("expected enum error")
	}
	if _, err := ParseFormValue(doc.Schema.Fields[3], []string{"13-10-2026"}, true); err == nil {
		t.Fatal("expected date error")
	}
}

func TestParseRejectsStructuralErrors(t *testing.T) {
	_, err := Parse([]byte(`
schema:
  id_field: missing
  fields:
    - {name: key, type: text}
entities: [{key: A-1}]
`))
	if err == nil || !strings.Contains(err.Error(), "not declared") {
		t.Fatalf("expected id-field error, got %v", err)
	}
}

func TestTitleFieldMustBeScalar(t *testing.T) {
	_, err := Parse([]byte(`
schema:
  id_field: key
  title_field: card_title
  fields:
    - {name: key, type: text}
entities: [{key: A-1, card_title: [not, scalar]}]
`))
	if err == nil || !strings.Contains(err.Error(), "must be a scalar") {
		t.Fatalf("expected scalar title error, got %v", err)
	}
}

func TestParsePageHeading(t *testing.T) {
	const body = `
schema:
  id_field: key
  fields:
    - {name: key, type: text}
entities:
  - {key: A-1}
`
	doc, err := Parse([]byte("title:  Sprint triage \ndescription: Open bugs of the sprint.\n" + body))
	if err != nil {
		t.Fatal(err)
	}
	if doc.Title != "Sprint triage" || doc.Description != "Open bugs of the sprint." {
		t.Fatalf("title %q, description %q", doc.Title, doc.Description)
	}
	doc, err = Parse([]byte(body))
	if err != nil || doc.Title != "" || doc.Description != "" {
		t.Fatalf("absent heading should be empty: %v %#v", err, doc)
	}
	if _, err := Parse([]byte("title: [a]\n" + body)); err == nil || !strings.Contains(err.Error(), "title must be text") {
		t.Fatalf("expected a title error, got %v", err)
	}
}
