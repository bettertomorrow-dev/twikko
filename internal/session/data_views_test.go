package session

import (
	"go.yaml.in/yaml/v3"
	"os"
	"path/filepath"
	"testing"
	"twikko/internal/schema"
)

func TestExactEditsDraftsAndEmptyStates(t *testing.T) {
	dir := t.TempDir()
	v := filepath.Join(dir, "v.yaml")
	d := filepath.Join(dir, "d.json")
	os.WriteFile(v, []byte(`version: 1
schema:
  fields:
    - {name: n, type: number}
    - {name: text, type: text}
    - {name: flag, type: bool}
    - {name: tags, type: multienum, options: [a]}
`), 0600)
	os.WriteFile(d, []byte(`[{"n":9007199254740993.0000000001,"text":null,"flag":null,"extra":{"n":9007199254740995}},{"tags":[]}]`), 0600)
	doc, err := schema.LoadView(v, d)
	if err != nil {
		t.Fatal(err)
	}
	s := New(doc)
	if s.HasErrors() || s.ChangedCount() != 0 {
		t.Fatal("load mutated or invalidated data")
	}
	s.Edit(0, "n", []string{"9007199254740993.00000000010"}, true)
	if s.ChangedCount() != 0 {
		t.Fatal("equal decimal changed")
	}
	s.Edit(0, "n", []string{"1e-"}, true)
	if !s.HasErrors() || s.FieldValue(0, doc.Schema.Fields[0]) != "1e-" {
		t.Fatal("lost invalid draft")
	}
	s.SetMode("table")
	if s.FieldValue(0, doc.Schema.Fields[0]) != "1e-" {
		t.Fatal("switch lost draft")
	}
	s.Edit(0, "n", []string{"9007199254740993.0000000002"}, true)
	s.Edit(0, "text", []string{""}, true)
	s.Edit(1, "text", []string{"new"}, true)
	s.Edit(0, "flag", []string{"false"}, true)
	if s.HasErrors() || s.ChangedCount() != 4 {
		t.Fatalf("errors %v changes %v", s.Errors(), s.Changes())
	}
	changes := s.Changes()
	if changes[0].RecordIndex == nil || *changes[0].RecordIndex != 0 {
		t.Fatal("missing zero index")
	}
	if changes[2].RecordIndex == nil {
		t.Fatal("missing index")
	}
	s.Reset(0)
	if s.HasErrors() || schema.MapValue(s.Entity(0), "text").Tag != "!!null" {
		t.Fatal("reset lost null")
	}
	if schema.MapValue(s.Entity(1), "flag") != nil {
		t.Fatal("missing field was added")
	}
}

func TestClearTypedValuesUsesNull(t *testing.T) {
	doc, err := schema.Parse([]byte(`schema:
  id_field: id
  fields:
    - {name: id, type: text}
    - {name: n, type: number}
    - {name: flag, type: bool}
    - {name: due, type: date}
    - {name: text, type: text}
entities: [{id: PROJ-101, n: 5, flag: true, due: 2026-11-13, text: hello}]
`))
	if err != nil {
		t.Fatal(err)
	}
	s := New(doc)
	for _, name := range []string{"n", "flag", "due", "text"} {
		if err := s.Edit(0, name, []string{""}, true); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"n", "flag", "due"} {
		if schema.MapValue(s.Entity(0), name).Tag != "!!null" {
			t.Fatalf("%s did not clear to null", name)
		}
	}
	if schema.MapValue(s.Entity(0), "text").Tag != "!!str" {
		t.Fatal("text must clear to empty string")
	}
}

func TestInvalidRecursiveValueCanBeComparedAndCorrected(t *testing.T) {
	var root yaml.Node
	if err := yaml.Unmarshal([]byte("- {x: &v [*v]}"), &root); err != nil {
		t.Fatal(err)
	}
	s := New(&schema.Document{Schema: schema.Schema{Fields: []schema.Field{{Name: "x", Type: schema.Text}}}, Entities: root.Content[0].Content})
	if !s.HasErrors() || s.ChangedCount() != 0 {
		t.Fatal("invalid initial value lost")
	}
	if err := s.Edit(0, "x", []string{"fixed"}, true); err != nil {
		t.Fatal(err)
	}
	if s.HasErrors() || s.ChangedCount() != 1 {
		t.Fatal("could not correct recursive value")
	}
}

func TestAbsentSubmissionDoesNotChangeValues(t *testing.T) {
	doc, err := schema.Parse([]byte("schema:\n  id_field: id\n  fields: [{name: id, type: text}, {name: flag, type: bool}, {name: text, type: text}]\nentities: [{id: PROJ-101, flag: true, text: original}]\n"))
	if err != nil {
		t.Fatal(err)
	}
	s := New(doc)
	for _, f := range []string{"flag", "text"} {
		if err := s.Edit(0, f, nil, false); err != nil {
			t.Fatal(err)
		}
	}
	if s.ChangedCount() != 0 || s.HasErrors() {
		t.Fatal("absent submission changed data")
	}
}
