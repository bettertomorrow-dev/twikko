package session

import (
	"testing"

	"multieditor/internal/schema"
)

func TestEditResetAndChanges(t *testing.T) {
	doc, err := schema.Parse([]byte(`
schema:
  id_field: key
  fields:
    - {name: key, type: text, readonly: true}
    - {name: summary, type: text, required: true}
    - {name: labels, type: multienum, options: [bug, ui]}
entities:
  - {key: A-1, summary: Before, labels: [bug], unknown: preserved}
`))
	if err != nil {
		t.Fatal(err)
	}
	store := New(doc)
	if err := store.Edit(0, "key", []string{"A-2"}, true); err == nil {
		t.Fatal("expected readonly rejection")
	}
	if err := store.Edit(0, "summary", []string{"After"}, true); err != nil {
		t.Fatal(err)
	}
	if !store.IsChanged(0, "summary") || store.ChangedCount() != 1 {
		t.Fatal("expected changed summary")
	}
	changes := store.Changes()
	if len(changes) != 1 || changes[0].EntityID != "A-1" || changes[0].From != "Before" {
		t.Fatalf("unexpected changes: %#v", changes)
	}
	if err := store.Reset(0); err != nil {
		t.Fatal(err)
	}
	if store.ChangedCount() != 0 {
		t.Fatal("expected reset")
	}
	if schema.MapValue(store.Entities()[0], "unknown").Value != "preserved" {
		t.Fatal("lost unknown key")
	}
}

func TestIdentifierChangeUsesOriginalIdentity(t *testing.T) {
	doc, err := schema.Parse([]byte(`
schema:
  id_field: key
  fields: [{name: key, type: text}]
entities: [{key: PROJ-101}]
`))
	if err != nil {
		t.Fatal(err)
	}
	store := New(doc)
	if err := store.Edit(0, "key", []string{"PROJ-102"}, true); err != nil {
		t.Fatal(err)
	}
	changes := store.Changes()
	if len(changes) != 1 || changes[0].EntityID != "PROJ-101" || changes[0].To != "PROJ-102" {
		t.Fatalf("a rename must identify the existing record: %#v", changes)
	}
}

func TestInvalidEditSetsError(t *testing.T) {
	doc, err := schema.Parse([]byte(`
schema:
  id_field: key
  fields:
    - {name: key, type: text}
    - {name: points, type: number}
entities: [{key: A-1, points: 1}]
`))
	if err != nil {
		t.Fatal(err)
	}
	store := New(doc)
	if err := store.Edit(0, "points", []string{"wrong"}, true); err != nil {
		t.Fatal(err)
	}
	if !store.HasErrors() || store.Error(0, "points") == "" {
		t.Fatal("expected field error")
	}
}

func TestCollapsedIsSessionStateOnly(t *testing.T) {
	doc, err := schema.Parse([]byte(`
schema:
  id_field: key
  fields:
    - {name: key, type: text}
    - {name: summary, type: text}
entities: [{key: A-1, summary: Before}]
`))
	if err != nil {
		t.Fatal(err)
	}
	store := New(doc)
	if err := store.SetCollapsed(3, true); err == nil {
		t.Fatal("expected unknown entity error")
	}
	if err := store.SetCollapsed(0, true); err != nil || !store.Collapsed(0) {
		t.Fatal("expected collapsed")
	}
	if err := store.Edit(0, "summary", []string{"After"}, true); err != nil {
		t.Fatal(err)
	}
	if err := store.Reset(0); err != nil {
		t.Fatal(err)
	}
	if !store.Collapsed(0) {
		t.Fatal("reset must keep the collapsed flag")
	}
	if err := store.SetCollapsed(0, false); err != nil || store.Collapsed(0) {
		t.Fatal("expected expanded")
	}
}
