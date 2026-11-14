package session

import (
	"testing"
	"time"

	"go.yaml.in/yaml/v3"
	"twikko/internal/schema"
)

func highlightStore(t *testing.T, field schema.Field, value string, c schema.Condition) *Store {
	t.Helper()
	var entity yaml.Node
	if err := yaml.Unmarshal([]byte(value), &entity); err != nil {
		t.Fatal(err)
	}
	c.Field = field.Name
	return New(&schema.Document{
		Schema:   schema.Schema{Fields: []schema.Field{field}},
		Entities: []*yaml.Node{entity.Content[0]},
		View: &schema.View{Mode: "table", Table: schema.Table{RowHighlights: []schema.RowHighlight{
			{Intent: schema.IntentInfo, When: c},
		}}},
	})
}

func TestHighlightOperators(t *testing.T) {
	for _, tc := range []struct {
		name, value string
		field       schema.Field
		condition   schema.Condition
		match       bool
	}{
		{"exact greater", "{x: 9007199254740993.123456789}", schema.Field{Name: "x", Type: schema.Number}, schema.Condition{Op: "gt", Value: schema.Decimal("9007199254740993.123456788")}, true},
		{"exact equal", "{x: 1e-20}", schema.Field{Name: "x", Type: schema.Number}, schema.Condition{Op: "eq", Value: schema.Decimal("0.00000000000000000001")}, true},
		{"less", "{x: -2}", schema.Field{Name: "x", Type: schema.Number}, schema.Condition{Op: "lt", Value: schema.Decimal("0")}, true},
		{"not equal", "{x: 2}", schema.Field{Name: "x", Type: schema.Number}, schema.Condition{Op: "ne", Value: schema.Decimal("0")}, true},
		{"negative even", "{x: -2}", schema.Field{Name: "x", Type: schema.Number}, schema.Condition{Op: "even"}, true},
		{"negative odd", "{x: -3}", schema.Field{Name: "x", Type: schema.Number}, schema.Condition{Op: "odd"}, true},
		{"fraction not even", "{x: 2.5}", schema.Field{Name: "x", Type: schema.Number}, schema.Condition{Op: "even"}, false},
		{"contains", "{x: 'Abc def'}", schema.Field{Name: "x", Type: schema.Text}, schema.Condition{Op: "contains", Value: "bc "}, true},
		{"case sensitive", "{x: Abc}", schema.Field{Name: "x", Type: schema.Text}, schema.Condition{Op: "contains", Value: "abc"}, false},
		{"not contains", "{x: Abc}", schema.Field{Name: "x", Type: schema.Text}, schema.Condition{Op: "not_contains", Value: "z"}, true},
		{"Unicode length", "{x: '猫é🙂'}", schema.Field{Name: "x", Type: schema.Text}, schema.Condition{Op: "eq", Value: schema.Decimal("3"), Measure: schema.ConditionMeasure{Length: true}}, true},
		{"empty length", "{x: ''}", schema.Field{Name: "x", Type: schema.Text}, schema.Condition{Op: "eq", Value: schema.Decimal("0"), Measure: schema.ConditionMeasure{Length: true}}, true},
		{"bool false", "{x: false}", schema.Field{Name: "x", Type: schema.Bool}, schema.Condition{Op: "eq", Value: false}, true},
		{"bool unequal", "{x: false}", schema.Field{Name: "x", Type: schema.Bool}, schema.Condition{Op: "ne", Value: true}, true},
		{"enum equal", "{x: ready}", schema.Field{Name: "x", Type: schema.Enum, Options: []string{"new", "ready"}}, schema.Condition{Op: "eq", Value: "ready"}, true},
		{"enum any", "{x: ready}", schema.Field{Name: "x", Type: schema.Enum, Options: []string{"new", "ready"}}, schema.Condition{Op: "contains_any", Value: []string{"new", "ready"}}, true},
		{"enum all", "{x: ready}", schema.Field{Name: "x", Type: schema.Enum, Options: []string{"new", "ready"}}, schema.Condition{Op: "contains_all", Value: []string{"new", "ready"}}, false},
		{"set all duplicates", "{x: [b, a]}", schema.Field{Name: "x", Type: schema.MultiEnum, Options: []string{"a", "b"}}, schema.Condition{Op: "contains_all", Value: []string{"a", "a", "b"}}, true},
		{"set any", "{x: [a]}", schema.Field{Name: "x", Type: schema.MultiEnum, Options: []string{"a", "b"}}, schema.Condition{Op: "contains_any", Value: []string{"b"}}, false},
		{"empty set", "{x: []}", schema.Field{Name: "x", Type: schema.MultiEnum, Options: []string{"a"}}, schema.Condition{Op: "contains_all", Value: []string{"a"}}, false},
		{"missing not unequal", "{}", schema.Field{Name: "x", Type: schema.Number}, schema.Condition{Op: "ne", Value: schema.Decimal("0")}, false},
		{"null not unequal", "{x: null}", schema.Field{Name: "x", Type: schema.Number}, schema.Condition{Op: "ne", Value: schema.Decimal("0")}, false},
		{"missing not contains", "{}", schema.Field{Name: "x", Type: schema.Text}, schema.Condition{Op: "not_contains", Value: "a"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := highlightStore(t, tc.field, tc.value, tc.condition)
			if got := store.RowHighlight(0, time.Now()) != ""; got != tc.match {
				t.Fatalf("match = %v, want %v", got, tc.match)
			}
		})
	}
}

func TestHighlightTracksDraftResetAndRuleOrder(t *testing.T) {
	store := highlightStore(t, schema.Field{Name: "x", Type: schema.Number}, "{x: 2}", schema.Condition{Op: "even"})
	store.view.Table.RowHighlights = append(store.view.Table.RowHighlights, schema.RowHighlight{Intent: schema.IntentWarning, When: schema.Condition{Field: "x", Op: "gt", Value: schema.Decimal("0")}})
	if store.RowHighlight(0, time.Now()) != schema.IntentInfo {
		t.Fatal("first matching rule must win")
	}
	if err := store.Edit(0, "x", []string{"invalid"}, true); err != nil {
		t.Fatal(err)
	}
	if store.RowHighlight(0, time.Now()) != "" {
		t.Fatal("invalid draft matched old value")
	}
	if err := store.Edit(0, "x", []string{"3"}, true); err != nil {
		t.Fatal(err)
	}
	if store.RowHighlight(0, time.Now()) != schema.IntentWarning {
		t.Fatal("valid edit did not refresh rule")
	}
	store.Reset(0)
	if store.RowHighlight(0, time.Now()) != schema.IntentInfo {
		t.Fatal("reset did not restore rule")
	}
	if store.RowHighlight(-1, time.Now()) != "" || store.RowHighlight(1, time.Now()) != "" {
		t.Fatal("out of bounds record matched")
	}
}

func TestHighlightTemporal(t *testing.T) {
	zone, err := time.LoadLocation("Europe/Helsinki")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 3, 29, 12, 0, 59, 0, zone)
	for _, tc := range []struct {
		name, value, reference, unit, expected string
		typ                                    schema.FieldType
		match                                  bool
	}{
		{"calendar days across DST", "2026-03-28", "today", "days", "-1", schema.Date, true},
		{"elapsed hours across DST", "2026-03-28T12:00", "now", "hours", "-23", schema.DateTime, true},
		{"elapsed days across DST", "2026-03-28T12:00", "now", "days", "-1", schema.DateTime, false},
		{"minute precision", "2026-03-29T12:00", "now", "minutes", "0", schema.DateTime, true},
		{"time same day", "00:00", "now", "hours", "-12", schema.Time, true},
		{"missing DST hour", "2026-03-29T03:30", "now", "hours", "0", schema.DateTime, false},
		{"ambiguous DST hour", "2026-10-25T03:30", "now", "hours", "0", schema.DateTime, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := schema.Condition{Op: "eq", Value: schema.Decimal(tc.expected), Measure: schema.ConditionMeasure{DistanceFrom: tc.reference, Unit: tc.unit}}
			store := highlightStore(t, schema.Field{Name: "x", Type: tc.typ}, "{x: '"+tc.value+"'}", c)
			store.view.Table.Timezone = "Europe/Helsinki"
			store.view.Table.Location = zone
			if !store.HasRelativeHighlights() {
				t.Fatal("relative rule not detected")
			}
			if got := store.RowHighlight(0, now) != ""; got != tc.match {
				t.Fatalf("match = %v, want %v", got, tc.match)
			}
		})
	}
}

func TestHighlightEventDistanceAndGroups(t *testing.T) {
	var data yaml.Node
	yaml.Unmarshal([]byte("{x: '2026-11-01T12:30', start: '2026-10-31T12:00'}"), &data)
	c := schema.Condition{All: []schema.Condition{
		{Field: "x", Op: "gt", Value: schema.Decimal("24"), Measure: schema.ConditionMeasure{ReferenceField: "start", Unit: "hours"}},
		{Any: []schema.Condition{{Field: "x", Op: "eq", Value: schema.Decimal("24.5"), Measure: schema.ConditionMeasure{ReferenceField: "start", Unit: "hours"}}}},
	}}
	store := New(&schema.Document{
		Schema:   schema.Schema{Fields: []schema.Field{{Name: "x", Type: schema.DateTime}, {Name: "start", Type: schema.DateTime}}},
		Entities: []*yaml.Node{data.Content[0]},
		View:     &schema.View{Table: schema.Table{Timezone: "UTC", RowHighlights: []schema.RowHighlight{{Intent: schema.IntentSuccess, When: c}}}},
	})
	store.view.Table.Location = time.UTC
	if store.HasRelativeHighlights() || store.RowHighlight(0, time.Now()) != schema.IntentSuccess {
		t.Fatal("event distance or groups failed")
	}
	store.Edit(0, "start", []string{"invalid"}, true)
	if store.RowHighlight(0, time.Now()) != "" {
		t.Fatal("invalid reference used previous value")
	}
}

func TestConditionTemporalRejectsDSTGapAndOverlap(t *testing.T) {
	zone, err := time.LoadLocation("Europe/Helsinki")
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"2026-03-29T03:30", "2026-10-25T03:30"} {
		if _, ok := conditionTemporal(schema.DateTime, value, zone); ok {
			t.Fatalf("accepted nonexistent or ambiguous local time %s", value)
		}
	}
	if _, ok := conditionTemporal(schema.DateTime, "2026-10-25T04:30", zone); !ok {
		t.Fatal("rejected unambiguous local time")
	}
}
