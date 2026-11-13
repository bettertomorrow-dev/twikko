package schema

import (
	"os"
	"path/filepath"
	"testing"

	"go.yaml.in/yaml/v3"
)

func TestLoadViewFormats(t *testing.T) {
	dir := t.TempDir()
	view := filepath.Join(dir, "view.yaml")
	config := `version: 1
schema:
  fields:
    - {name: code, type: text}
    - {name: amount, type: number}
view:
  mode: table
`
	if err := os.WriteFile(view, []byte(config), 0600); err != nil {
		t.Fatal(err)
	}
	for ext, data := range map[string]string{"yaml": "- {code: '001', amount: 12345678901234567890.123456789}\n", "json": `[{"code":"001","amount":12345678901234567890.123456789}]`, "csv": "code,amount\n001,12345678901234567890.123456789\n"} {
		t.Run(ext, func(t *testing.T) {
			p := filepath.Join(dir, "data."+ext)
			if err := os.WriteFile(p, []byte(data), 0600); err != nil {
				t.Fatal(err)
			}
			doc, err := LoadView(view, p)
			if err != nil {
				t.Fatal(err)
			}
			if doc.Schema.IDField != "" || MapValue(doc.Entities[0], "code").Value != "001" || MapValue(doc.Entities[0], "amount").Value != "12345678901234567890.123456789" {
				t.Fatalf("unexpected document: %#v", doc)
			}
		})
	}
}
func TestLoadViewStrict(t *testing.T) {
	dir := t.TempDir()
	v := filepath.Join(dir, "view.yaml")
	d := filepath.Join(dir, "data.json")
	os.WriteFile(d, []byte(`[{"id":"x"},{"id":"x"}]`), 0600)
	for _, config := range []string{"version: 2\nschema: {}", "version: 1\nschema:\n  fields: []\n  bogus: true", "version: 1\nschema:\n  id_field: id\n  fields: [{name: id, type: text}]"} {
		os.WriteFile(v, []byte(config), 0600)
		if _, err := LoadView(v, d); err == nil {
			t.Fatalf("accepted %s", config)
		}
	}
}

func TestLoadViewRejectsInvalidLayoutsAndIDs(t *testing.T) {
	dir := t.TempDir()
	view := filepath.Join(dir, "view.yaml")
	data := filepath.Join(dir, "data.yaml")
	if err := os.WriteFile(data, []byte("- {id: 1}\n- {id: 1.0}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, config := range []string{
		"version: 1\nschema:\n  id_field: id\n  fields: [{name: id, type: number}]\n",
		"version: 1\nschema:\n  fields: [{name: id, type: number}]\nview:\n  cards:\n    header: {template: issue}\n",
		"version: 1\nschema:\n  fields: [{name: id, type: number}]\nview:\n  table: {unknown: true}\n",
	} {
		if err := os.WriteFile(view, []byte(config), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadView(view, data); err == nil {
			t.Fatalf("accepted %s", config)
		}
	}
}

func TestLoadViewImportRegressions(t *testing.T) {
	dir := t.TempDir()
	view := filepath.Join(dir, "view.yaml")
	for _, tc := range []struct {
		name, field, ext, data string
		bad                    bool
	}{
		{"trailing data", "{name: x, type: text}", "yaml", "- {x: a}\n---\n- {x: b}\n", true},
		{"number array", "{name: x, type: number, readonly: true}", "json", `[{"x":[]}]`, true},
		{"bool array", "{name: x, type: bool, readonly: true}", "yaml", "- {x: []}", true},
		{"number empty string", "{name: x, type: number, readonly: true}", "json", `[{"x":""}]`, true},
		{"text alias", "{name: x, type: text, readonly: true}", "yaml", "- {raw: &v hello, x: *v}", false},
		{"number alias", "{name: x, type: number, readonly: true}", "yaml", "- {raw: &v 42, x: *v}", false},
		{"null alias", "{name: x, type: bool, readonly: true}", "yaml", "- {raw: &v null, x: *v}", false},
		{"csv null", "{name: x, type: multienum, options: [''], readonly: true}", "csv", "x\n[null]\n", true},
		{"json number", "{name: x, type: multienum, options: ['1'], readonly: true}", "json", `[{"x":[1]}]`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := filepath.Join(dir, "data."+tc.ext)
			os.WriteFile(view, []byte("version: 1\nschema:\n  fields: ["+tc.field+"]\n"), 0600)
			os.WriteFile(data, []byte(tc.data), 0600)
			_, err := LoadView(view, data)
			if (err != nil) != tc.bad {
				t.Fatalf("error = %v, want error %v", err, tc.bad)
			}
		})
	}
	os.WriteFile(view, []byte("version: 1\nschema: {fields: [{name: x, type: number}]}\n---\n{}\n"), 0600)
	data := filepath.Join(dir, "data.csv")
	os.WriteFile(data, []byte("x\n010\n"), 0600)
	if _, err := LoadView(view, data); err == nil {
		t.Fatal("accepted second view document")
	}
	os.WriteFile(view, []byte("version: 1\nschema: {fields: [{name: x, type: number}]}\n"), 0600)
	doc, err := LoadView(view, data)
	if err != nil {
		t.Fatal(err)
	}
	b, err := yaml.Marshal(doc.Entities)
	if err != nil {
		t.Fatal(err)
	}
	var result []map[string]any
	if err := yaml.Unmarshal(b, &result); err != nil {
		t.Fatal(err)
	}
	if result[0]["x"] != 10 {
		t.Fatalf("round trip: %s", b)
	}
}

func TestCSVRejectsNonDecimalNumberNotation(t *testing.T) {
	dir := t.TempDir()
	v := filepath.Join(dir, "v.yaml")
	d := filepath.Join(dir, "d.csv")
	os.WriteFile(v, []byte("version: 1\nschema:\n  fields: [{name: n, type: number, readonly: true}]\n"), 0600)
	for _, value := range []string{"0x10", "0b10", "1_000"} {
		os.WriteFile(d, []byte("n\n"+value+"\n"), 0600)
		if _, err := LoadView(v, d); err == nil {
			t.Fatalf("accepted CSV %s", value)
		}
	}
}

func TestLaunchModeValidatesOnlyReachableEditors(t *testing.T) {
	dir := t.TempDir()
	v := filepath.Join(dir, "v.yaml")
	d := filepath.Join(dir, "d.yaml")
	os.WriteFile(v, []byte(`version: 1
schema:
  fields:
    - {name: title, type: text}
    - {name: n, type: number}
view:
  mode: cards
  cards:
    body: [[{source: title}]]
  table:
    columns: [{source: n}]
`), 0600)
	os.WriteFile(d, []byte("- {title: Before, n: invalid}\n"), 0600)
	if _, err := LoadView(v, d); err == nil {
		t.Fatal("cards opened with an unfixable hidden value")
	}
	doc, err := LoadViewWithMode(v, d, "table")
	if err != nil {
		t.Fatal(err)
	}
	if doc.View.Mode != "table" {
		t.Fatal("launch override ignored")
	}
	if _, err := LoadViewWithMode(v, d, "grid"); err == nil {
		t.Fatal("accepted invalid launch mode")
	}
}
