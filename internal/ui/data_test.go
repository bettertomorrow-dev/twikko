package ui

import (
	"strings"
	"testing"

	"multieditor/internal/icons"
	"multieditor/internal/schema"
)

func resolve(t *testing.T, header, entity string) ([]WidgetView, *schema.Document) {
	t.Helper()
	doc, err := schema.Parse([]byte(`
schema:
  id_field: key
  header:
` + header + `
  fields:
    - {name: key, type: text}
entities:
  - ` + entity + `
`))
	if err != nil {
		t.Fatal(err)
	}
	return ResolveWidgets(doc.Schema.Header, doc.Entities[0]), doc
}

func TestResolveIconUsesMatchingVariant(t *testing.T) {
	views, _ := resolve(t, `    - {type: icon, source: type, variants: {Bug: {icon: bug, intent: danger}}}`, `{key: A-1, type: Bug}`)
	if len(views) != 1 || views[0].Icon != "bug" || views[0].Text != "Bug" {
		t.Fatalf("unexpected views: %#v", views)
	}
	if !strings.Contains(views[0].Class, "text-destructive") {
		t.Errorf("danger icon must use the destructive colour: %q", views[0].Class)
	}
}

func TestResolveIconFallsBackWhenNoVariantMatches(t *testing.T) {
	views, _ := resolve(t, `    - {type: icon, source: type, variants: {Bug: {icon: bug}}}`, `{key: A-1, type: Mystery}`)
	if len(views) != 1 || views[0].Icon != icons.Fallback {
		t.Fatalf("an unmatched value must draw the fallback icon: %#v", views)
	}
	if !strings.Contains(views[0].Class, "text-muted-foreground") {
		t.Errorf("the fallback icon must be neutral: %q", views[0].Class)
	}
}

func TestResolveIconVariantWithIntentOnlyKeepsFallbackIcon(t *testing.T) {
	views, _ := resolve(t, `    - {type: icon, source: type, variants: {Bug: {intent: warning}}}`, `{key: A-1, type: Bug}`)
	if len(views) != 1 || views[0].Icon != icons.Fallback || !strings.Contains(views[0].Class, "text-warning-foreground") {
		t.Fatalf("unexpected views: %#v", views)
	}
}

func TestResolveSkipsMissingAndEmptyValues(t *testing.T) {
	doc, err := schema.Parse([]byte(`
schema:
  id_field: key
  header:
    - {type: tag, source: key}
    - {type: tag, source: due}
  fields:
    - {name: key, type: text}
entities:
  - {key: A-1, due: 2026-11-20}
  - {key: A-2}
  - {key: A-3, due: ""}
`))
	if err != nil {
		t.Fatal(err)
	}
	for i, want := range []int{2, 1, 1} {
		if got := len(ResolveWidgets(doc.Schema.Header, doc.Entities[i])); got != want {
			t.Errorf("entity %d: %d widgets, want %d", i, got, want)
		}
	}
}

func TestResolveTagIntentAndTextStyle(t *testing.T) {
	views, _ := resolve(t, `    - {type: tag, source: status, variants: {Done: {intent: success}}}
    - {type: tag, source: owner}
    - {type: text, source: title, style: {bold: true, italic: true, muted: true}}
    - {type: text, source: key}`, `{key: A-1, status: Done, owner: Dana, title: Fix login}`)
	if len(views) != 4 {
		t.Fatalf("unexpected views: %#v", views)
	}
	if !strings.Contains(views[0].Class, "bg-success/10") || !strings.Contains(views[1].Class, "bg-secondary") {
		t.Errorf("tag intents: %q, %q", views[0].Class, views[1].Class)
	}
	for _, want := range []string{"font-semibold", "italic", "text-muted-foreground", "truncate"} {
		if !strings.Contains(views[2].Class, want) {
			t.Errorf("styled text lacks %q: %q", want, views[2].Class)
		}
	}
	if strings.Contains(views[3].Class, "font-semibold") || strings.Contains(views[3].Class, "italic") {
		t.Errorf("unstyled text must stay plain: %q", views[3].Class)
	}
}

func TestHeaderTitleIsFirstTextWidgetOrID(t *testing.T) {
	views := []WidgetView{{Type: schema.WidgetTag, Text: "A-1"}, {Type: schema.WidgetText, Text: "Fix login"}}
	if got := headerTitle(views, "A-1"); got != "Fix login" {
		t.Errorf("got %q", got)
	}
	if got := headerTitle(views[:1], "A-1"); got != "A-1" {
		t.Errorf("without a text widget the title is the identifier, got %q", got)
	}
}
