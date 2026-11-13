package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"twikko/internal/schema"
	"twikko/internal/session"
)

func TestTableSwitchDraftsResetAndSave(t *testing.T) {
	dir := t.TempDir()
	v := filepath.Join(dir, "v.yaml")
	d := filepath.Join(dir, "d.json")
	os.WriteFile(v, []byte(`version: 1
schema:
  id_field: id
  fields:
    - {name: id, type: text}
    - {name: title, type: text}
    - {name: n, type: number}
    - {name: flag, type: bool}
    - {name: tags, type: multienum, options: [a]}
view:
  mode: table
  show_labels: false
  table:
    columns:
      - {source: id}
      - {source: title, show_label: true}
      - {source: n}
      - {source: flag}
      - {source: tags}
`), 0600)
	os.WriteFile(d, []byte(`[{"id":"PROJ-101","title":"before","n":5,"flag":true,"tags":["a"]}]`), 0600)
	doc, err := schema.LoadView(v, d)
	if err != nil {
		t.Fatal(err)
	}
	s, err := New(session.New(doc), filepath.Join(dir, "out.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	table := post(t, s, "/mode?value=table", "")
	for _, want := range []string{"<table", `scope="col"`, `class="sr-only">n`, `value="before"`} {
		if !strings.Contains(table, want) {
			t.Fatalf("missing %s: %s", want, table)
		}
	}
	edit := post(t, s, "/entities/0/fields/n", "value=1e-")
	if strings.Contains(edit, `id="field-0-n"`) || strings.Contains(edit, "entity-badges") {
		t.Fatal("edit replaced control or targeted absent badge")
	}
	cards := post(t, s, "/mode?value=cards", "")
	if !strings.Contains(cards, `value="1e-"`) || !strings.Contains(cards, "must be") {
		t.Fatal("lost invalid draft")
	}
	r := httptest.NewRecorder()
	s.ServeHTTP(r, httptest.NewRequest(http.MethodPost, "/s/"+s.Token()+"/save", nil))
	if r.Code != 422 {
		t.Fatal("saved invalid draft")
	}
	r = httptest.NewRecorder()
	s.ServeHTTP(r, httptest.NewRequest(http.MethodPost, "/s/"+s.Token()+"/entities/0/fields/id", strings.NewReader("value=other")))
	if r.Code != 400 {
		t.Fatal("changed immutable ID")
	}
	post(t, s, "/mode?value=table", "")
	reset := post(t, s, "/entities/0/reset", "")
	if !strings.HasPrefix(reset, "<tr") || !strings.Contains(reset, `value="5"`) {
		t.Fatal("reset did not render restored row")
	}
	post(t, s, "/entities/0/fields/flag", "value=")
	post(t, s, "/entities/0/fields/tags", "present=true")
	post(t, s, "/entities/0/fields/title", "value=after")
	post(t, s, "/save", "")
	b, _ := os.ReadFile(s.outputPath)
	for _, want := range []string{"title: after", "flag: null", "tags: []"} {
		if !strings.Contains(string(b), want) {
			t.Fatalf("missing %s: %s", want, b)
		}
	}
}
