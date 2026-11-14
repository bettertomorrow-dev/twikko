package schema

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

var conditionFields = []Field{
	{Name: "amount", Type: Number},
	{Name: "title", Type: Text},
	{Name: "done", Type: Bool},
	{Name: "status", Type: Enum, Options: []string{"new", "ready"}},
	{Name: "tags", Type: MultiEnum, Options: []string{"review", "external"}},
	{Name: "due", Type: Date},
	{Name: "started", Type: DateTime},
	{Name: "finished", Type: DateTime},
}

func parseTestCondition(text string) (Condition, error) {
	var n yaml.Node
	if err := yaml.Unmarshal([]byte(text), &n); err != nil {
		return Condition{}, err
	}
	count := 0
	return decodeCondition(n.Content[0], "rule.when", fieldMap(conditionFields), 0, &count)
}

func TestConditionTypes(t *testing.T) {
	for _, text := range []string{
		"{field: amount, op: gt, value: 9007199254740993.123456789}",
		"{field: amount, op: even}",
		"{field: title, op: not_contains, value: ''}",
		"{field: title, measure: length, op: eq, value: 3}",
		"{field: done, op: eq, value: false}",
		"{field: status, op: contains_all, value: [new]}",
		"{field: tags, op: contains_any, value: [review, external]}",
		"{field: due, measure: {distance_from: today, unit: days}, op: lt, value: 0}",
		"{field: finished, measure: {distance_from: {field: started}, unit: hours}, op: gt, value: 24}",
		"{all: [{field: done, op: eq, value: true}, {any: [{field: amount, op: odd}]}]}",
	} {
		t.Run(text, func(t *testing.T) {
			if _, err := parseTestCondition(text); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestConditionRejectsInvalidConfig(t *testing.T) {
	for _, text := range []string{
		"{}",
		"{all: []}",
		"{all: [{field: amount, op: odd}], any: []}",
		"{field: amount, op: odd, value: 1}",
		"{field: amount, op: gt, value: '1'}",
		"{field: amount, op: ge, value: 1}",
		"{field: amount, op: contains, value: 1}",
		"{field: missing, op: eq, value: 1}",
		"{field: title, measure: length, op: eq, value: null}",
		"{field: done, op: eq, value: 'true'}",
		"{field: done, op: gt, value: false}",
		"{field: status, op: eq, value: unknown}",
		"{field: tags, op: eq, value: review}",
		"{field: tags, op: contains_any, value: []}",
		"{field: tags, op: contains_all, value: [unknown]}",
		"{field: due, op: lt, value: '2026-01-01'}",
		"{field: due, measure: {distance_from: now, unit: days}, op: lt, value: 0}",
		"{field: due, measure: {distance_from: today, unit: hours}, op: lt, value: 0}",
		"{field: due, measure: {distance_from: today, unit: days}, op: even}",
		"{field: finished, measure: {distance_from: {field: due}, unit: hours}, op: lt, value: 0}",
		"{field: finished, measure: {distance_from: now, unit: months}, op: lt, value: 0}",
		"{field: amount, measure: length, op: gt, value: 0}",
		"{field: title, op: eq, value: x, typo: true}",
	} {
		t.Run(text, func(t *testing.T) {
			_, err := parseTestCondition(text)
			if err == nil || !strings.Contains(err.Error(), "rule.when") {
				t.Fatalf("expected located error, got %v", err)
			}
		})
	}
	deep := "{field: amount, op: odd}"
	for i := 0; i < 10; i++ {
		deep = "{all: [" + deep + "]}"
	}
	if _, err := parseTestCondition(deep); err == nil {
		t.Fatal("accepted deeply nested condition")
	}
	wide := "{any: [" + strings.Repeat("{field: amount, op: odd},", 256) + "]}"
	if _, err := parseTestCondition(wide); err == nil {
		t.Fatal("accepted excessive condition count")
	}
}

func TestLoadViewHighlights(t *testing.T) {
	dir := t.TempDir()
	v, d := filepath.Join(dir, "view.yaml"), filepath.Join(dir, "data.yaml")
	if err := os.WriteFile(d, []byte("- {amount: 10}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		table string
		bad   bool
	}{
		{"timezone: Europe/Helsinki\n    row_highlights: [{intent: warning, when: {field: amount, op: even}}]", false},
		{"timezone: Unknown/Zone", true},
		{"timezone: 100", true},
		{"row_highlights: [{intent: typo, when: {field: amount, op: even}}]", true},
		{"row_highlights: [{intent: info, when: {field: amount, op: even}, typo: true}]", true},
		{"row_highlights: [{intent: info}]", true},
	} {
		config := "version: 1\nschema:\n  fields: [{name: amount, type: number}]\nview:\n  mode: table\n  table:\n    " + tc.table + "\n"
		if err := os.WriteFile(v, []byte(config), 0600); err != nil {
			t.Fatal(err)
		}
		doc, err := LoadView(v, d)
		if (err != nil) != tc.bad {
			t.Fatalf("%s: error %v", tc.table, err)
		}
		if err == nil && (doc.View.Table.Timezone != "Europe/Helsinki" || doc.View.Table.Location == nil || len(doc.View.Table.RowHighlights) != 1) {
			t.Fatalf("highlight config lost: %#v", doc.View.Table)
		}
	}
}
