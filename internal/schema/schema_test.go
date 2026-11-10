package schema

import (
	"strings"
	"testing"
)

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
