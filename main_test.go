package main

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRunCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	path := filepath.Join(t.TempDir(), "output.yaml")
	var stdout bytes.Buffer
	if code := run(ctx, []string{"--input", "examples/jira.yaml", "--output", path}, &stdout); code != 2 {
		t.Fatalf("cancel exit code %d: %s", code, &stdout)
	}
	if !strings.HasPrefix(stdout.String(), "twikko: url=") || !strings.Contains(stdout.String(), "twikko: status=cancelled output="+path) {
		t.Fatalf("unexpected CLI output: %s", &stdout)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "status: cancelled\n" {
		t.Fatalf("cancel output: %q, %v", data, err)
	}
}

func TestRunSave(t *testing.T) {
	input := filepath.Join(t.TempDir(), "input.yaml")
	output := filepath.Join(t.TempDir(), "output.yaml")
	if err := os.WriteFile(input, []byte(`
schema:
  id_field: key
  fields:
    - {name: key, type: text}
    - {name: summary, type: text, required: true}
entities: [{key: A-1, summary: before}]
`), 0o600); err != nil {
		t.Fatal(err)
	}
	reader, writer := io.Pipe()
	result := make(chan int, 1)
	go func() {
		result <- run(context.Background(), []string{"--input", input, "--output", output}, writer)
		writer.Close()
	}()
	scanner := bufio.NewScanner(reader)
	if !scanner.Scan() {
		t.Fatal("missing URL output")
	}
	address := strings.TrimPrefix(scanner.Text(), "twikko: url=")
	go func() {
		for scanner.Scan() {
		}
	}()
	parsed, err := url.Parse(address)
	if err != nil {
		t.Fatal(err)
	}
	edit, err := http.NewRequest(http.MethodPost, address+"entities/0/fields/summary", strings.NewReader("value=after"))
	if err != nil {
		t.Fatal(err)
	}
	edit.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	edit.Host = parsed.Host
	if response, err := http.DefaultClient.Do(edit); err != nil || response.StatusCode != http.StatusOK {
		t.Fatalf("edit: %v", err)
	}
	save, err := http.NewRequest(http.MethodPost, address+"save", nil)
	if err != nil {
		t.Fatal(err)
	}
	save.Host = parsed.Host
	if response, err := http.DefaultClient.Do(save); err != nil || response.StatusCode != http.StatusOK {
		t.Fatalf("save: %v", err)
	}
	select {
	case code := <-result:
		if code != 0 {
			t.Fatalf("unexpected exit code: %d", code)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("run did not exit")
	}
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte("summary: after")) || !bytes.Contains(data, []byte("status: saved")) {
		t.Fatalf("unexpected output: %s", data)
	}
}
