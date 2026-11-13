package ui

import (
	"context"
	"strings"
	"testing"
	"twikko/internal/schema"
	"twikko/internal/session"
)

func boolPointer(b bool) *bool { return &b }
func TestLabelVisibilityPrecedenceAndAccessibleHTML(t *testing.T) {
	for _, tc := range []struct {
		name          string
		global, local *bool
		want          bool
	}{
		{"default", nil, nil, true}, {"global false", boolPointer(false), nil, false},
		{"local true", boolPointer(false), boolPointer(true), true}, {"local false", boolPointer(true), boolPointer(false), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fields := []schema.Field{{Name: "title", Label: "Title", Type: schema.Text}, {Name: "due", Label: "Due", Type: schema.Date}, {Name: "flag", Label: "Flag", Type: schema.Bool}}
			body := &schema.Body{Rows: [][]schema.Widget{{{Type: schema.WidgetControl, Source: "title", ShowLabel: tc.local}, {Type: schema.WidgetDate, Source: "due", ShowLabel: tc.local}, {Type: schema.WidgetToggle, Source: "flag", ShowLabel: tc.local}}}}
			s := session.New(&schema.Document{Schema: schema.Schema{Fields: fields, Body: body}, View: &schema.View{Mode: "cards", ShowLabels: tc.global}})
			data := NewPageData(s, "token")
			// Empty data still has semantic table headings.
			var html strings.Builder
			s.SetMode("table")
			data = NewPageData(s, "token")
			if err := Page(data).Render(context.Background(), &html); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(html.String(), `scope="col"`) || !strings.Contains(html.String(), "Title") {
				t.Fatal("empty table lost headings")
			}
			s.SetMode("cards")
			// Use a synthetic record to exercise controls, including a masked picker.
			doc, err := schema.Parse([]byte("schema:\n  id_field: id\n  fields: [{name: id,type: text},{name: title,type: text},{name: due,type: date},{name: flag,type: bool}]\nentities: [{id: PROJ-101, title: text, due: 2026-11-13, flag: true}]"))
			if err != nil {
				t.Fatal(err)
			}
			doc.Schema.Fields = fields
			doc.Schema.IDField = ""
			doc.Schema.Body = body
			doc.View = &schema.View{Mode: "cards", ShowLabels: tc.global}
			data = NewPageData(session.New(doc), "token")
			if data.Entities[0].Rows[0].Widgets[0].ShowLabel != tc.want {
				t.Fatal("wrong precedence")
			}
			html.Reset()
			if err := Page(data).Render(context.Background(), &html); err != nil {
				t.Fatal(err)
			}
			for _, want := range []string{`aria-label="Title"`, `aria-label="Due"`, `aria-label="Flag"`, `aria-describedby="status-0-due"`} {
				if !strings.Contains(html.String(), want) {
					t.Fatalf("missing accessible name %s", want)
				}
			}
		})
	}
}
func TestHeaderLabelExplicitOverridesAndDisplay(t *testing.T) {
	doc, err := schema.Parse([]byte("schema:\n  id_field: id\n  fields: [{name: id,type: text},{name: title,label: Title,type: text}]\nentities: [{id: PROJ-101, title: hello}]"))
	if err != nil {
		t.Fatal(err)
	}
	doc.Schema.Header = &schema.Header{Widgets: []schema.Widget{{Type: schema.WidgetText, Source: "title", ShowLabel: boolPointer(true)}, {Type: schema.WidgetControl, Source: "title", ShowLabel: boolPointer(true)}}}
	doc.View = &schema.View{Mode: "cards", ShowLabels: boolPointer(false)}
	data := NewPageData(session.New(doc), "token")
	for _, w := range data.Entities[0].Widgets {
		if !w.ShowLabel {
			t.Fatal("explicit true hidden")
		}
	}
	if data.Entities[0].Widgets[0].Label != "Title" {
		t.Fatal("display label did not use field label")
	}
}

func TestDOMKeysAreDistinctAndSafe(t *testing.T) {
	names := []string{"title", "a/b", "a b", "a.b", "--YS9i", "日本語", "_field"}
	seen := map[string]bool{}
	for _, name := range names {
		key := domKey(name)
		if seen[key] {
			t.Fatal("DOM key collision")
		}
		seen[key] = true
		for _, r := range key {
			if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_') {
				t.Fatalf("unsafe DOM key %q", key)
			}
		}
	}
}
