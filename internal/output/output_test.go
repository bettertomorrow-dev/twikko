package output

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"multieditor/internal/schema"
	"multieditor/internal/session"
)

func TestWriteAtomic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "result.yaml")
	if err := WriteAtomic(path, Cancelled()); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "status: cancelled\n" {
		t.Fatalf("unexpected output: %q", data)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || strings.HasPrefix(entries[0].Name(), ".multieditor-") {
		t.Fatalf("unexpected directory entries: %#v", entries)
	}
}

func TestSavedIncludesEntitiesAndChanges(t *testing.T) {
	doc, err := schema.Parse([]byte(`
schema:
  id_field: key
  fields: [{name: key, type: text}, {name: summary, type: text}]
entities: [{key: A-1, summary: old}]
`))
	if err != nil {
		t.Fatal(err)
	}
	store := session.New(doc)
	if err := store.Edit(0, "summary", []string{"new"}, true); err != nil {
		t.Fatal(err)
	}
	result := Saved(store, testTime)
	if result.Status != "saved" || len(result.Changes) != 1 || len(result.Entities) != 1 {
		t.Fatalf("unexpected result: %#v", result)
	}
}

var testTime = mustTime("2026-10-13T08:15:30Z")

func mustTime(value string) (resultTime time.Time) {
	resultTime, _ = time.Parse(time.RFC3339, value)
	return resultTime
}
