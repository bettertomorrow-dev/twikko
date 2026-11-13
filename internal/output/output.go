package output

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"go.yaml.in/yaml/v3"
	"twikko/internal/session"
)

type Change struct {
	EntityID    string     `yaml:"entity_id,omitempty"`
	RecordIndex *int       `yaml:"record_index,omitempty"`
	Field       string     `yaml:"field"`
	From        *yaml.Node `yaml:"from,omitempty"`
	To          *yaml.Node `yaml:"to"`
}

type Result struct {
	Status   string       `yaml:"status"`
	SavedAt  string       `yaml:"saved_at,omitempty"`
	Changes  []Change     `yaml:"changes,omitempty"`
	Entities []*yaml.Node `yaml:"entities,omitempty"`
}

func Saved(store *session.Store, now time.Time) Result {
	changes := store.Changes()
	result := Result{
		Status:   "saved",
		SavedAt:  now.UTC().Format(time.RFC3339),
		Entities: store.Entities(),
		Changes:  make([]Change, len(changes)),
	}
	for i, change := range changes {
		result.Changes[i] = Change{EntityID: change.EntityID, RecordIndex: change.RecordIndex, Field: change.Field, From: change.FromNode, To: change.ToNode}
	}
	return result
}

func Cancelled() Result {
	return Result{Status: "cancelled"}
}

func WriteAtomic(path string, result Result) error {
	data, err := yaml.Marshal(result)
	if err != nil {
		return fmt.Errorf("marshal result: %w", err)
	}
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".twikko-*")
	if err != nil {
		return fmt.Errorf("create output temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write output: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("sync output: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close output: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("replace output: %w", err)
	}
	return nil
}

// Keep explicit empty lists for saved results while Cancel contains only status.
func (r Result) MarshalYAML() (any, error) {
	if r.Status != "saved" {
		return struct {
			Status string `yaml:"status"`
		}{r.Status}, nil
	}
	r.Entities = append([]*yaml.Node(nil), r.Entities...)
	r.Changes = append([]Change(nil), r.Changes...)
	// Resolve aliases independently of their original anchor position: editing an
	// anchor must not leave a hidden value referring to a removed definition.
	nextAnchor := 0
	for i, n := range r.Entities {
		r.Entities[i] = materialize(n, make(map[*yaml.Node]*yaml.Node), &nextAnchor)
	}
	for i := range r.Changes {
		r.Changes[i].From = materialize(r.Changes[i].From, make(map[*yaml.Node]*yaml.Node), &nextAnchor)
		r.Changes[i].To = materialize(r.Changes[i].To, make(map[*yaml.Node]*yaml.Node), &nextAnchor)
	}
	if r.Entities == nil {
		r.Entities = []*yaml.Node{}
	}
	if r.Changes == nil {
		r.Changes = []Change{}
	}
	return struct {
		Status   string       `yaml:"status"`
		SavedAt  string       `yaml:"saved_at"`
		Entities []*yaml.Node `yaml:"entities"`
		Changes  []Change     `yaml:"changes"`
	}{r.Status, r.SavedAt, r.Entities, r.Changes}, nil
}

func materialize(n *yaml.Node, seen map[*yaml.Node]*yaml.Node, next *int) *yaml.Node {
	if n == nil {
		return nil
	}
	for n.Kind == yaml.AliasNode {
		n = n.Alias
	}
	if parent := seen[n]; parent != nil {
		if parent.Anchor == "" {
			*next++
			parent.Anchor = fmt.Sprintf("value%d", *next)
		}
		return &yaml.Node{Kind: yaml.AliasNode, Value: parent.Anchor, Alias: parent}
	}
	copy := *n
	copy.Anchor = ""
	copy.Alias = nil
	copy.Content = nil
	seen[n] = &copy
	for _, child := range n.Content {
		copy.Content = append(copy.Content, materialize(child, seen, next))
	}
	return &copy
}
