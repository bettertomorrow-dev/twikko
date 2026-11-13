package output

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.yaml.in/yaml/v3"
	"twikko/internal/schema"
	"twikko/internal/session"
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
	if len(entries) != 1 || strings.HasPrefix(entries[0].Name(), ".twikko-") {
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

func TestNoIDOutputPreservesPrecisionAndPresence(t *testing.T) {
	dir := t.TempDir()
	v := filepath.Join(dir, "v.yaml")
	d := filepath.Join(dir, "d.json")
	os.WriteFile(v, []byte("version: 1\nschema:\n  fields: [{name: n, type: number}, {name: text, type: text}]\n"), 0600)
	os.WriteFile(d, []byte(`[{"n":9007199254740993.123456789,"text":null},{}]`), 0600)
	doc, err := schema.LoadView(v, d)
	if err != nil {
		t.Fatal(err)
	}
	s := session.New(doc)
	s.Edit(0, "n", []string{"9007199254740993.123456788"}, true)
	s.Edit(0, "text", []string{""}, true)
	s.Edit(1, "text", []string{"new"}, true)
	path := filepath.Join(dir, "out.yaml")
	if err := WriteAtomic(path, Saved(s, testTime)); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(path)
	out := string(b)
	for _, wanted := range []string{"record_index: 0", "from: 9007199254740993.123456789", "to: 9007199254740993.123456788", "from: null", "record_index: 1"} {
		if !strings.Contains(out, wanted) {
			t.Fatalf("missing %s in %s", wanted, out)
		}
	}
	if strings.Contains(out, "entity_id") {
		t.Fatal("invented ID")
	}
	if strings.Count(out, "from:") != 2 {
		t.Fatalf("missing/null confused: %s", out)
	}
	empty := session.New(&schema.Document{Schema: doc.Schema})
	if err := WriteAtomic(path, Saved(empty, testTime)); err != nil {
		t.Fatal(err)
	}
	b, _ = os.ReadFile(path)
	for _, wanted := range []string{"entities: []", "changes: []"} {
		if !strings.Contains(string(b), wanted) {
			t.Fatalf("missing %s", wanted)
		}
	}
}

func TestEditingAnchorPreservesHiddenAliasValue(t *testing.T) {
	doc, err := schema.Parse([]byte("schema:\n  id_field: id\n  fields: [{name: id, type: text}, {name: x, type: text}]\nentities: [{id: PROJ-101, x: &v old, hidden: *v}]\n"))
	if err != nil {
		t.Fatal(err)
	}
	s := session.New(doc)
	s.Edit(0, "x", []string{"new"}, true)
	path := filepath.Join(t.TempDir(), "out.yaml")
	if err := WriteAtomic(path, Saved(s, testTime)); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	var result struct {
		Entities []map[string]any `yaml:"entities"`
	}
	if err := yaml.Unmarshal(data, &result); err != nil {
		t.Fatalf("unreadable YAML %s: %v", data, err)
	}
	if result.Entities[0]["hidden"] != "old" || result.Entities[0]["x"] != "new" {
		t.Fatalf("alias changed: %s", data)
	}
}
