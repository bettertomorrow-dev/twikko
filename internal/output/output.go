package output

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"go.yaml.in/yaml/v3"
	"multieditor/internal/session"
)

type Change struct {
	EntityID string `yaml:"entity_id"`
	Field    string `yaml:"field"`
	From     any    `yaml:"from"`
	To       any    `yaml:"to"`
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
		result.Changes[i] = Change(change)
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
	tmp, err := os.CreateTemp(dir, ".multieditor-*")
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
