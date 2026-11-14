package session

import (
	"reflect"

	"go.yaml.in/yaml/v3"
	"twikko/internal/schema"
)

// Only changed fields are retained, including invalid input and missing values.
type fieldState struct {
	node    *yaml.Node
	draft   []string
	invalid string
}
type fieldChange struct {
	index         int
	name          string
	before, after fieldState
}
type historyEntry struct {
	changes []fieldChange
	group   string
}

func (s *Store) fieldState(index int, name string) fieldState {
	var draft []string
	if raw, ok := s.drafts[index][name]; ok {
		draft = append([]string{}, raw...)
	}
	return fieldState{schema.Clone(schema.MapValue(s.current[index], name)), draft, s.errors[index][name]}
}

func sameState(a, b fieldState) bool {
	return equalValue(schema.Field{}, a.node, b.node) && reflect.DeepEqual(a.draft, b.draft) && a.invalid == b.invalid
}

func (s *Store) record(changes []fieldChange, group string) {
	kept := changes[:0]
	for _, c := range changes {
		if !sameState(c.before, c.after) {
			kept = append(kept, c)
		}
	}
	if len(kept) == 0 {
		return
	}
	s.history = s.history[:s.cursor]
	if group != "" && s.cursor > 0 && len(kept) == 1 {
		last := &s.history[s.cursor-1]
		c := kept[0]
		if last.group == group && len(last.changes) == 1 && last.changes[0].index == c.index && last.changes[0].name == c.name {
			last.changes[0].after = c.after
			if sameState(last.changes[0].before, c.after) {
				s.cursor--
				s.history = s.history[:s.cursor]
			}
			return
		}
	}
	s.history = append(s.history, historyEntry{kept, group})
	s.cursor++
}

func (s *Store) restore(c fieldChange, state fieldState) {
	entity := s.current[c.index]
	if state.node == nil {
		for i := 0; i < len(entity.Content); i += 2 {
			if entity.Content[i].Value == c.name {
				entity.Content = append(entity.Content[:i], entity.Content[i+2:]...)
				break
			}
		}
	} else {
		schema.SetMapValue(entity, c.name, schema.Clone(state.node))
	}
	delete(s.drafts[c.index], c.name)
	if state.draft != nil {
		if s.drafts[c.index] == nil {
			s.drafts[c.index] = make(map[string][]string)
		}
		s.drafts[c.index][c.name] = append([]string{}, state.draft...)
	}
	s.clearError(c.index, c.name)
	if state.invalid != "" {
		s.setError(c.index, c.name, state.invalid)
	}
}

// MoveHistory returns the records whose controls need refreshing.
func (s *Store) MoveHistory(redo bool) []int {
	s.mu.Lock()
	defer s.mu.Unlock()
	if (!redo && s.cursor == 0) || (redo && s.cursor == len(s.history)) {
		return nil
	}
	position := s.cursor - 1
	if redo {
		position = s.cursor
		s.cursor++
	} else {
		s.cursor--
	}
	if s.cursor > 0 {
		s.history[s.cursor-1].group = ""
	}
	entry := &s.history[position]
	entry.group = "" // Edits after undo/redo start a fresh step.
	indices := []int{}
	seen := map[int]bool{}
	for _, c := range entry.changes {
		state := c.before
		if redo {
			state = c.after
		}
		s.restore(c, state)
		if !seen[c.index] {
			indices = append(indices, c.index)
			seen[c.index] = true
		}
	}
	return indices
}

func (s *Store) ResetAll() []int {
	s.mu.Lock()
	defer s.mu.Unlock()
	indices := make([]int, len(s.current))
	for i := range indices {
		indices[i] = i
	}
	s.reset(indices)
	return indices
}

func (s *Store) reset(indices []int) {
	changes := []fieldChange{}
	for _, index := range indices {
		for _, field := range s.schema.Fields {
			before := s.fieldState(index, field.Name)
			after := fieldState{node: schema.Clone(schema.MapValue(s.original[index], field.Name))}
			s.restore(fieldChange{index: index, name: field.Name}, after)
			node := schema.MapValue(s.current[index], field.Name)
			if node == nil && field.Required {
				s.setError(index, field.Name, "is required")
			} else if node != nil {
				if _, err := schema.ParseNode(field, node); err != nil {
					s.setError(index, field.Name, err.Error())
				}
			}
			changes = append(changes, fieldChange{index, field.Name, before, s.fieldState(index, field.Name)})
		}
	}
	s.record(changes, "")
}

func (s *Store) HasDrafts() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, fields := range s.drafts {
		if len(fields) > 0 {
			return true
		}
	}
	return false
}
