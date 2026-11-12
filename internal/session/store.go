package session

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"sync"

	"go.yaml.in/yaml/v3"
	"multieditor/internal/schema"
)

type Change struct {
	EntityID string
	Field    string
	From     any
	To       any
}

type Store struct {
	mu        sync.RWMutex
	title     string
	desc      string
	schema    schema.Schema
	original  []*yaml.Node
	current   []*yaml.Node
	errors    map[int]map[string]string
	collapsed map[int]bool
}

func New(doc *schema.Document) *Store {
	original := cloneEntities(doc.Entities)
	store := &Store{
		title:     doc.Title,
		desc:      doc.Description,
		schema:    doc.Schema,
		original:  original,
		current:   cloneEntities(doc.Entities),
		errors:    make(map[int]map[string]string),
		collapsed: make(map[int]bool),
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
	return parsedValue(s.current[index], field)
}

func (s *Store) Edit(index int, name string, values []string, present bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	field, ok := s.field(name)
	if !ok {
		return fmt.Errorf("unknown field %q", name)
	}
	if field.Readonly {
		return fmt.Errorf("field %q is read-only", name)
	}
	if index < 0 || index >= len(s.current) {
		return errors.New("entity not found")
	}
	value, err := schema.ParseFormValue(field, values, present)
	if err != nil {
		s.setError(index, name, err.Error())
		return nil
	}
	schema.SetMapValue(s.current[index], name, valueNode(field, value))
	s.clearError(index, name)
	return nil
}

func (s *Store) Reset(index int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if index < 0 || index >= len(s.current) {
		return errors.New("entity not found")
	}
	s.current[index] = schema.Clone(s.original[index])
	delete(s.errors, index)
	s.validateEntity(index)
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
	return !reflect.DeepEqual(parsedValue(s.original[index], field), parsedValue(s.current[index], field))
}

func (s *Store) ChangedCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	count := 0
	for index := range s.current {
		for _, field := range s.schema.Fields {
			if !reflect.DeepEqual(parsedValue(s.original[index], field), parsedValue(s.current[index], field)) {
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
		id := fmt.Sprint(parsedValue(s.original[index], s.idField()))
		for _, field := range s.schema.Fields {
			from := parsedValue(s.original[index], field)
			to := parsedValue(s.current[index], field)
			if !reflect.DeepEqual(from, to) {
				changes = append(changes, Change{EntityID: id, Field: field.Name, From: from, To: to})
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

func (s *Store) idField() schema.Field {
	field, _ := s.field(s.schema.IDField)
	return field
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
		return ""
	}
	return value
}

func valueNode(field schema.Field, value any) *yaml.Node {
	switch field.Type {
	case schema.MultiEnum:
		values := value.([]string)
		node := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
		for _, item := range values {
			node.Content = append(node.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: item})
		}
		return node
	case schema.Bool:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: strconv.FormatBool(value.(bool))}
	case schema.Number:
		if value == "" {
			return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: ""}
		}
		n := value.(float64)
		tag := "!!float"
		text := strconv.FormatFloat(n, 'f', -1, 64)
		if n == float64(int64(n)) {
			tag = "!!int"
		}
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: tag, Value: text}
	default:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value.(string)}
	}
}

func cloneEntities(entities []*yaml.Node) []*yaml.Node {
	result := make([]*yaml.Node, len(entities))
	for i, entity := range entities {
		result[i] = schema.Clone(entity)
	}
	return result
}
