package session

import (
	"errors"
	"fmt"
	"sync"

	"go.yaml.in/yaml/v3"
	"twikko/internal/schema"
)

type Change struct {
	EntityID    string
	RecordIndex *int
	FromPresent bool
	FromNode    *yaml.Node
	ToNode      *yaml.Node
	Field       string
	From        any
	To          any
}

type Store struct {
	mu        sync.RWMutex
	title     string
	desc      string
	schema    schema.Schema
	view      *schema.View
	mode      string
	drafts    map[int]map[string][]string
	original  []*yaml.Node
	current   []*yaml.Node
	errors    map[int]map[string]string
	collapsed map[int]bool
	history   []historyEntry
	cursor    int
}

func New(doc *schema.Document) *Store {
	original := cloneEntities(doc.Entities)
	store := &Store{
		title:  doc.Title,
		desc:   doc.Description,
		schema: doc.Schema,
		view:   doc.View, mode: "cards", drafts: make(map[int]map[string][]string),
		original:  original,
		current:   cloneEntities(doc.Entities),
		errors:    make(map[int]map[string]string),
		collapsed: make(map[int]bool),
	}
	store.schema.Fields = append([]schema.Field(nil), doc.Schema.Fields...)
	for i := range store.schema.Fields {
		if store.schema.Fields[i].Name == store.schema.IDField {
			store.schema.Fields[i].Readonly = true
		}
	}
	if doc.View != nil {
		store.mode = doc.View.Mode
	}
	store.validateAll()
	return store
}

// Title and Description are the optional heading of the page.
func (s *Store) Title() string { return s.title }

func (s *Store) Description() string { return s.desc }

func (s *Store) Schema() schema.Schema {
	return s.schema
}

func (s *Store) EntityCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.current)
}

func (s *Store) Entity(index int) *yaml.Node {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if index < 0 || index >= len(s.current) {
		return nil
	}
	return schema.Clone(s.current[index])
}

func (s *Store) FieldValue(index int, field schema.Field) any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if index < 0 || index >= len(s.current) {
		return ""
	}
	if raw, ok := s.drafts[index][field.Name]; ok {
		if field.Type == schema.MultiEnum {
			return append([]string{}, raw...)
		}
		if len(raw) > 0 {
			return raw[0]
		}
		return ""
	}
	return parsedValue(s.current[index], field)
}

func (s *Store) Edit(index int, name string, values []string, present bool) error {
	return s.EditGrouped(index, name, values, present, "")
}

// EditGrouped coalesces updates from one uninterrupted field interaction.
func (s *Store) EditGrouped(index int, name string, values []string, present bool, group string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	field, ok := s.field(name)
	if !ok {
		return fmt.Errorf("unknown field %q", name)
	}
	if field.Readonly || (s.schema.IDField != "" && name == s.schema.IDField) {
		return fmt.Errorf("field %q is read-only", name)
	}
	if index < 0 || index >= len(s.current) {
		return errors.New("entity not found")
	}
	if !present {
		return nil
	}
	before := s.fieldState(index, name)
	defer func() { s.record([]fieldChange{{index, name, before, s.fieldState(index, name)}}, group) }()
	if s.drafts[index] == nil {
		s.drafts[index] = make(map[string][]string)
	}
	s.drafts[index][name] = append([]string{}, values...)
	value, err := schema.ParseFormValue(field, values, present)
	if err != nil {
		s.setError(index, name, err.Error())
		return nil
	}
	candidate := valueNode(field, value)
	existing := schema.MapValue(s.current[index], name)
	if !equalValue(field, existing, candidate) {
		schema.SetMapValue(s.current[index], name, candidate)
	}
	delete(s.drafts[index], name)
	s.clearError(index, name)
	return nil
}

func (s *Store) Reset(index int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if index < 0 || index >= len(s.current) {
		return errors.New("entity not found")
	}
	s.reset([]int{index})
	return nil
}

// SetCollapsed records whether an entity card is collapsed to its header. The
// flag is session state only and is not written to the output.
func (s *Store) SetCollapsed(index int, collapsed bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if index < 0 || index >= len(s.current) {
		return errors.New("entity not found")
	}
	if collapsed {
		s.collapsed[index] = true
	} else {
		delete(s.collapsed, index)
	}
	return nil
}

func (s *Store) Collapsed(index int) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.collapsed[index]
}

func (s *Store) Error(index int, name string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.errors[index][name]
}

func (s *Store) Errors() map[int]map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make(map[int]map[string]string, len(s.errors))
	for index, fields := range s.errors {
		result[index] = make(map[string]string, len(fields))
		for name, message := range fields {
			result[index][name] = message
		}
	}
	return result
}

func (s *Store) HasErrors() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, fields := range s.errors {
		if len(fields) != 0 {
			return true
		}
	}
	return false
}

func (s *Store) IsChanged(index int, name string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if index < 0 || index >= len(s.current) {
		return false
	}
	field, ok := s.field(name)
	if !ok {
		return false
	}
	return !equalValue(field, schema.MapValue(s.original[index], field.Name), schema.MapValue(s.current[index], field.Name))
}

func (s *Store) ChangedCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	count := 0
	for index := range s.current {
		for _, field := range s.schema.Fields {
			if !equalValue(field, schema.MapValue(s.original[index], field.Name), schema.MapValue(s.current[index], field.Name)) {
				count++
			}
		}
	}
	return count
}

func (s *Store) Changes() []Change {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var changes []Change
	for index := range s.current {
		id := ""
		var recordIndex *int
		if s.schema.IDField != "" {
			n := schema.MapValue(s.original[index], s.schema.IDField)
			for n.Kind == yaml.AliasNode {
				n = n.Alias
			}
			id = n.Value
		} else {
			n := index
			recordIndex = &n
		}
		for _, field := range s.schema.Fields {
			from := parsedValue(s.original[index], field)
			to := parsedValue(s.current[index], field)
			if !equalValue(field, schema.MapValue(s.original[index], field.Name), schema.MapValue(s.current[index], field.Name)) {
				changes = append(changes, Change{EntityID: id, RecordIndex: recordIndex, FromPresent: schema.MapValue(s.original[index], field.Name) != nil, FromNode: schema.Clone(schema.MapValue(s.original[index], field.Name)), ToNode: schema.Clone(schema.MapValue(s.current[index], field.Name)), Field: field.Name, From: from, To: to})
			}
		}
	}
	return changes
}

func (s *Store) Entities() []*yaml.Node {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneEntities(s.current)
}

func (s *Store) validateAll() {
	for index := range s.current {
		s.validateEntity(index)
	}
}

func (s *Store) validateEntity(index int) {
	for _, field := range s.schema.Fields {
		node := schema.MapValue(s.current[index], field.Name)
		if node == nil {
			if field.Required {
				s.setError(index, field.Name, "is required")
			}
			continue
		}
		if _, err := schema.ParseNode(field, node); err != nil {
			s.setError(index, field.Name, err.Error())
		} else {
			s.clearError(index, field.Name)
		}
	}
}

func (s *Store) field(name string) (schema.Field, bool) {
	for _, field := range s.schema.Fields {
		if field.Name == name {
			return field, true
		}
	}
	return schema.Field{}, false
}

func (s *Store) setError(index int, name, message string) {
	if s.errors[index] == nil {
		s.errors[index] = make(map[string]string)
	}
	s.errors[index][name] = message
}

func (s *Store) clearError(index int, name string) {
	if s.errors[index] == nil {
		return
	}
	delete(s.errors[index], name)
	if len(s.errors[index]) == 0 {
		delete(s.errors, index)
	}
}

func parsedValue(entity *yaml.Node, field schema.Field) any {
	node := schema.MapValue(entity, field.Name)
	if node == nil {
		if field.Type == schema.MultiEnum {
			return []string{}
		}
		return ""
	}
	value, err := schema.ParseNode(field, node)
	if err != nil {
		if node.Kind == yaml.ScalarNode {
			return node.Value
		}
		b, _ := yaml.Marshal(node)
		return string(b)
	}
	return value
}

func valueNode(field schema.Field, value any) *yaml.Node {
	if value == nil {
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!null", Value: "null"}
	}
	if d, ok := value.(schema.Decimal); ok {
		return d.Node()
	}
	n := &yaml.Node{}
	_ = n.Encode(value)
	return n
}

func equalValue(field schema.Field, a, b *yaml.Node) bool {
	return equalNodes(field, a, b, make(map[[2]*yaml.Node]bool))
}
func equalNodes(field schema.Field, a, b *yaml.Node, seen map[[2]*yaml.Node]bool) bool {
	if a == nil || b == nil {
		return a == b
	}
	for a.Kind == yaml.AliasNode {
		a = a.Alias
	}
	for b.Kind == yaml.AliasNode {
		b = b.Alias
	}
	pair := [2]*yaml.Node{a, b}
	if seen[pair] {
		return true
	}
	seen[pair] = true
	if field.Type == schema.Number && a.Tag != "!!null" && b.Tag != "!!null" {
		x, xe := schema.NodeDecimal(a)
		y, ye := schema.NodeDecimal(b)
		if xe == nil && ye == nil {
			return x.Equal(y)
		}
	}
	if a.Kind != b.Kind || a.Tag != b.Tag || a.Value != b.Value || len(a.Content) != len(b.Content) {
		return false
	}
	for i := range a.Content {
		if !equalNodes(schema.Field{}, a.Content[i], b.Content[i], seen) {
			return false
		}
	}
	return true
}

func (s *Store) View() *schema.View { return s.view }
func (s *Store) Mode() string       { s.mu.RLock(); defer s.mu.RUnlock(); return s.mode }

func cloneEntities(entities []*yaml.Node) []*yaml.Node {
	result := make([]*yaml.Node, len(entities))
	for i, entity := range entities {
		result[i] = schema.Clone(entity)
	}
	return result
}
