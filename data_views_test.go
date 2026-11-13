package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
	"twikko/internal/output"
	"twikko/internal/session"
	"twikko/internal/ui"

	"go.yaml.in/yaml/v3"
	"twikko/internal/schema"
)

func TestViewCLIExamplesSaveAndCancel(t *testing.T) {
	for _, ext := range []string{"yaml", "json", "csv"} {
		for _, action := range []string{"save", "cancel"} {
			t.Run(ext+"/"+action, func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				out := filepath.Join(t.TempDir(), "out.yaml")
				r, w := io.Pipe()
				defer r.Close()
				done := make(chan int, 1)
				go func() {
					done <- run(ctx, []string{"--view", "examples/tasks.view.yaml", "--data", "examples/tasks." + ext, "--output", out}, w)
					w.Close()
				}()
				scan := bufio.NewScanner(r)
				if !scan.Scan() {
					t.Fatal("missing startup line")
				}
				line := scan.Text()
				if !strings.HasPrefix(line, "twikko: url=") {
					t.Fatal(line)
				}
				address := strings.TrimPrefix(line, "twikko: url=")
				go func() {
					for scan.Scan() {
					}
				}()
				response, err := http.Get(address)
				if err != nil {
					t.Fatal(err)
				}
				page, _ := io.ReadAll(response.Body)
				response.Body.Close()
				if response.StatusCode != 200 || !bytes.Contains(page, []byte("Example tasks")) {
					t.Fatalf("page: %s", page)
				}
				response, err = http.Post(address+action, "application/x-www-form-urlencoded", nil)
				if err != nil {
					t.Fatal(err)
				}
				response.Body.Close()
				if response.StatusCode != 200 {
					t.Fatal(response.Status)
				}
				expected := 0
				if action == "cancel" {
					expected = 2
				}
				if code := <-done; code != expected {
					t.Fatalf("exit %d", code)
				}
				b, err := os.ReadFile(out)
				if err != nil {
					t.Fatal(err)
				}
				if action == "save" {
					var root yaml.Node
					if err := yaml.Unmarshal(b, &root); err != nil {
						t.Fatal(err)
					}
					changes := schema.MapValue(root.Content[0], "changes")
					if changes == nil || len(changes.Content) != 0 {
						t.Fatalf("no-op save: %s", b)
					}
					entities := schema.MapValue(root.Content[0], "entities")
					if entities == nil || len(entities.Content) != 2 {
						t.Fatalf("entities: %s", b)
					}
				} else if string(b) != "status: cancelled\n" {
					t.Fatalf("cancel: %s", b)
				}
			})
		}
	}
}
func TestViewCLIRejectsFlagCombinations(t *testing.T) {
	for _, args := range [][]string{
		{"--view", "v.yaml"}, {"--data", "d.json"}, {"--input", "i.yaml", "--view", "v.yaml", "--data", "d.json"},
		{"--view", "v.yaml", "--data", "d.json", "extra"},
	} {
		var out bytes.Buffer
		args = append(args, "--output", filepath.Join(t.TempDir(), "out.yaml"))
		if run(context.Background(), args, &out) != 1 {
			t.Fatalf("accepted %v", args)
		}
	}
}
func TestGojiraViewLoadsEmptyData(t *testing.T) {
	p := filepath.Join(t.TempDir(), "empty.yaml")
	os.WriteFile(p, []byte("[]"), 0600)
	if _, err := schema.LoadView("skills/gojira/assets/gojira.view.yaml", p); err != nil {
		t.Fatal(err)
	}
}
func TestAcceptanceVolumeLoad(t *testing.T) {
	dir := t.TempDir()
	v := filepath.Join(dir, "v.yaml")
	d := filepath.Join(dir, "d.json")
	var config, data strings.Builder
	config.WriteString("version: 1\nschema:\n  fields:\n")
	for f := 0; f < 20; f++ {
		fmt.Fprintf(&config, "    - {name: f%d, type: text}\n", f)
	}
	data.WriteString("[")
	for r := 0; r < 200; r++ {
		if r > 0 {
			data.WriteString(",")
		}
		data.WriteString("{")
		for f := 0; f < 20; f++ {
			if f > 0 {
				data.WriteString(",")
			}
			fmt.Fprintf(&data, `"f%d":"record %d field %d"`, f, r, f)
		}
		data.WriteString("}")
	}
	data.WriteString("]")
	os.WriteFile(v, []byte(config.String()), 0600)
	os.WriteFile(d, []byte(data.String()), 0600)
	start := time.Now()
	doc, err := schema.LoadView(v, d)
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Entities) != 200 {
		t.Fatal("truncated")
	}
	t.Logf("load 200x20: %s", time.Since(start))
	store := session.New(doc)
	for _, mode := range []string{"cards", "table"} {
		store.SetMode(mode)
		start = time.Now()
		var rendered strings.Builder
		if err := ui.Page(ui.NewPageData(store, "test-token")).Render(context.Background(), &rendered); err != nil {
			t.Fatal(err)
		}
		t.Logf("render %s 200x20: %s, %d bytes", mode, time.Since(start), rendered.Len())
	}
	timings := make([]time.Duration, 20)
	for i := range timings {
		start = time.Now()
		store.Edit(i, "f0", []string{"edited"}, true)
		ui.NewPageData(store, "test-token")
		timings[i] = time.Since(start)
	}
	sort.Slice(timings, func(i, j int) bool { return timings[i] < timings[j] })
	t.Logf("edit + page data p95: %s", timings[18])
	start = time.Now()
	if err := output.WriteAtomic(filepath.Join(dir, "out.yaml"), output.Saved(store, time.Now())); err != nil {
		t.Fatal(err)
	}
	t.Logf("save 200x20: %s", time.Since(start))

}
