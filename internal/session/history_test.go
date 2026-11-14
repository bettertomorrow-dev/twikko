package session

import (
	"testing"
	"twikko/internal/schema"
)

func historyStore(t *testing.T) *Store {
	t.Helper()
	doc, err := schema.Parse([]byte(`
schema:
  id_field: key
  fields:
    - {name: key, type: text}
    - {name: title, type: text}
    - {name: points, type: number}
    - {name: labels, type: multienum, options: [one, two]}
entities:
  - {key: PROJ-101, title: Before, points: 1, labels: [one], extra: preserved}
  - {key: PROJ-102, title: Second}
`))
	if err != nil {
		t.Fatal(err)
	}
	return New(doc)
}

func TestHistoryGroupsFieldInteraction(t *testing.T) {
	s := historyStore(t)
	for _, value := range []string{"A", "Af", "After"} {
		if err := s.EditGrouped(0, "title", []string{value}, true, "visit-1"); err != nil {
			t.Fatal(err)
		}
	}
	s.MoveHistory(false)
	if s.ChangedCount() != 0 {
		t.Fatal("typing was not undone as one field edit")
	}
	if got := s.MoveHistory(false); len(got) != 0 {
		t.Fatal("unexpected extra step")
	}
	s.MoveHistory(true)
	if schema.MapValue(s.Entity(0), "title").Value != "After" {
		t.Fatal("redo lost final value")
	}
	s.EditGrouped(0, "title", []string{"Again"}, true, "visit-2")
	s.MoveHistory(false)
	if schema.MapValue(s.Entity(0), "title").Value != "After" {
		t.Fatal("new visit was merged")
	}
	s.EditGrouped(0, "title", []string{"Branch"}, true, "visit-2")
	if len(s.MoveHistory(true)) != 0 {
		t.Fatal("new edit retained redo branch")
	}
}

func TestHistoryRestoresDraftsAndReset(t *testing.T) {
	s := historyStore(t)
	s.Edit(0, "points", []string{"invalid"}, true)
	s.Edit(1, "title", []string{"Changed"}, true)
	s.ResetAll()
	if s.HasErrors() || s.HasDrafts() || s.ChangedCount() != 0 {
		t.Fatal("reset did not clear edits")
	}
	if len(s.MoveHistory(false)) != 2 {
		t.Fatal("reset should be one transaction")
	}
	if !s.HasErrors() || !s.HasDrafts() || s.FieldValue(0, s.Schema().Fields[2]) != "invalid" {
		t.Fatal("undo reset lost invalid draft")
	}
	s.MoveHistory(false)
	s.MoveHistory(false)
	if s.HasErrors() || s.HasDrafts() || s.ChangedCount() != 0 {
		t.Fatal("undo did not reach original state")
	}
	s.MoveHistory(true)
	if !s.HasErrors() {
		t.Fatal("redo lost validation error")
	}
	if schema.MapValue(s.Entity(0), "extra").Value != "preserved" {
		t.Fatal("unknown field lost")
	}
}

func TestHistoryMissingNullAndMultiSelect(t *testing.T) {
	s := historyStore(t)
	s.Edit(1, "points", []string{""}, true)
	if schema.MapValue(s.Entity(1), "points") == nil {
		t.Fatal("expected explicit null")
	}
	s.MoveHistory(false)
	if schema.MapValue(s.Entity(1), "points") != nil {
		t.Fatal("undo must restore absence")
	}
	s.MoveHistory(true)
	if schema.MapValue(s.Entity(1), "points").Tag != "!!null" {
		t.Fatal("redo must restore null")
	}
	s.Edit(0, "labels", []string{"one", "two"}, true)
	s.Reset(0)
	s.MoveHistory(false)
	if len(schema.MapValue(s.Entity(0), "labels").Content) != 2 {
		t.Fatal("undo record reset lost multiselect")
	}
	s.MoveHistory(false)
	if len(schema.MapValue(s.Entity(0), "labels").Content) != 1 {
		t.Fatal("undo multiselect failed")
	}
}

func TestHistoryNoopAndRejectedEdits(t *testing.T) {
	s := historyStore(t)
	s.Edit(0, "title", []string{"Before"}, true)
	s.Edit(0, "key", []string{"PROJ-999"}, true)
	s.Edit(0, "title", []string{"Ignored"}, false)
	s.ResetAll()
	if len(s.MoveHistory(false)) != 0 {
		t.Fatal("no-op created history")
	}
	s.EditGrouped(0, "title", []string{"A"}, true, "visit")
	s.EditGrouped(0, "title", []string{"Before"}, true, "visit")
	if len(s.MoveHistory(false)) != 0 {
		t.Fatal("net-zero interaction created history")
	}
}
