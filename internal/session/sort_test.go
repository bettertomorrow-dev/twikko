package session

import (
	"reflect"
	"testing"

	"go.yaml.in/yaml/v3"
	"twikko/internal/schema"
)

func sortStore(t *testing.T, kind schema.FieldType, options []string, records string) *Store {
	t.Helper()
	var root yaml.Node
	if err := yaml.Unmarshal([]byte(records), &root); err != nil {
		t.Fatal(err)
	}
	entities := root.Content[0].Content
	return New(&schema.Document{Schema: schema.Schema{Fields: []schema.Field{{Name: "value", Type: kind, Options: options}}}, View: &schema.View{Mode: "table"}, Entities: entities})
}

func TestSortedIndices(t *testing.T) {
	for _, tc := range []struct {
		name      string
		kind      schema.FieldType
		options   []string
		records   string
		asc, desc []int
	}{
		{"exact numbers", schema.Number, nil, `[{value: 9007199254740993}, {value: 9007199254740992}, {value: 9.007199254740993e15}, {}, {value: null}, {value: broken}]`, []int{1, 0, 2, 3, 4, 5}, []int{0, 2, 1, 3, 4, 5}},
		{"text", schema.Text, nil, `[{value: z}, {value: A}, {value: a}, {value: ""}]`, []int{3, 1, 2, 0}, []int{0, 2, 1, 3}},
		{"bool", schema.Bool, nil, `[{value: true}, {value: false}, {}]`, []int{1, 0, 2}, []int{0, 1, 2}},
		{"enum", schema.Enum, []string{"z", "a"}, `[{value: a}, {value: z}]`, []int{1, 0}, []int{0, 1}},
		{"sets", schema.MultiEnum, []string{"z", "a", "b"}, `[{value: [a,z]}, {value: [z,a]}, {value: [b]}, {value: []}]`, []int{3, 0, 1, 2}, []int{2, 0, 1, 3}},
		{"time precision", schema.Time, nil, `[{value: "12:00:01"}, {value: "12:00"}, {value: "12:00:00"}]`, []int{1, 2, 0}, []int{0, 1, 2}},
		{"datetime", schema.DateTime, nil, `[{value: "2026-11-14T12:00:01"}, {value: "2026-11-14T12:00"}]`, []int{1, 0}, []int{0, 1}},
		{"date", schema.Date, nil, `[{value: "2026-11-14"}, {value: "2026-01-01"}]`, []int{1, 0}, []int{0, 1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := sortStore(t, tc.kind, tc.options, tc.records)
			before := s.Entities()
			for direction, want := range map[string][]int{"asc": tc.asc, "desc": tc.desc} {
				got, err := s.SortedIndices("0", direction)
				if err != nil || !reflect.DeepEqual(got, want) {
					t.Fatalf("%s: got %v, %v; want %v", direction, got, err, want)
				}
			}
			if !reflect.DeepEqual(before, s.Entities()) || len(s.Changes()) != 0 {
				t.Fatal("sort changed records")
			}
		})
	}
}

func TestSortInvalidDraftAndIdentity(t *testing.T) {
	s := sortStore(t, schema.Number, nil, `[{value: 1},{value: 2},{value: 3}]`)
	if err := s.Edit(0, "value", []string{"broken"}, true); err != nil {
		t.Fatal(err)
	}
	got, _ := s.SortedIndices("0", "asc")
	if !reflect.DeepEqual(got, []int{1, 2, 0}) {
		t.Fatalf("invalid draft sorted as old value: %v", got)
	}
	if err := s.Edit(1, "value", []string{"4"}, true); err != nil {
		t.Fatal(err)
	}
	change := s.Changes()[0]
	if change.RecordIndex == nil || *change.RecordIndex != 1 {
		t.Fatalf("identity changed: %+v", change)
	}
	for direction, want := range map[string][]int{"asc": {0, 1, 2}, "desc": {2, 1, 0}} {
		got, err := s.SortedIndices("number", direction)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("number %s: %v %v", direction, got, err)
		}
	}
	for _, args := range [][2]string{{"actions", "asc"}, {"-1", "asc"}, {"1", "asc"}, {"0", "none"}} {
		if _, err := s.SortedIndices(args[0], args[1]); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
}
