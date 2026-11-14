package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"twikko/internal/schema"
	"twikko/internal/session"
)

func tableServer(t *testing.T) *Server {
	t.Helper()
	dir := t.TempDir()
	view := filepath.Join(dir, "view.yaml")
	data := filepath.Join(dir, "data.yaml")
	if err := os.WriteFile(view, []byte(`version: 1
schema:
  fields:
    - {name: amount, type: number}
view:
  mode: table
  table:
    row_highlights:
      - intent: warning
        when: {field: amount, op: gt, value: 2}
`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(data, []byte("- {amount: 3}\n- {amount: 1}\n- {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	doc, err := schema.LoadView(view, data)
	if err != nil {
		t.Fatal(err)
	}
	s, err := New(session.New(doc), filepath.Join(dir, "out.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	s.SetAllowedHost("127.0.0.1:1234")
	return s
}

func TestTableSortAndHighlightResponses(t *testing.T) {
	s := tableServer(t)
	page := get(t, s)
	if !strings.Contains(page, "row-highlight-warning") {
		t.Fatal("initial highlight missing")
	}
	var sorted struct {
		Order []int `json:"order"`
	}
	if err := json.Unmarshal([]byte(post(t, s, "/table/sort?column=0&direction=asc", "")), &sorted); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(sorted.Order, []int{1, 0, 2}) {
		t.Fatalf("sort: %v", sorted.Order)
	}
	request := httptest.NewRequest(http.MethodPost, "/s/"+s.Token()+"/entities/0/fields/amount", strings.NewReader("value=0"))
	request.Host = "127.0.0.1:1234"
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	s.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Header().Get("HX-Trigger-After-Swap"), `"className":""`) {
		t.Fatalf("highlight edit: %d %s", response.Code, response.Header())
	}
	if strings.Contains(response.Body.String(), `id="field-0-amount"`) {
		t.Fatal("edit replaced editor")
	}
	request = httptest.NewRequest(http.MethodGet, "/s/"+s.Token()+"/table/highlights", nil)
	request.Host = "127.0.0.1:1234"
	response = httptest.NewRecorder()
	s.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"0":""`) {
		t.Fatalf("highlight refresh: %d %s", response.Code, response.Body.String())
	}
	post(t, s, "/save", "")
	result, err := os.ReadFile(s.outputPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(result), "record_index: 0") || strings.Index(string(result), "amount: 0") > strings.Index(string(result), "amount: 1") {
		t.Fatalf("sort changed saved identity/order: %s", result)
	}
}

func TestTableEndpointsRejectInvalidRequests(t *testing.T) {
	s := tableServer(t)
	for _, tc := range []struct {
		path, host string
		status     int
	}{
		{"/table/sort?column=actions&direction=asc", "127.0.0.1:1234", 400},
		{"/table/sort?column=0&direction=none", "127.0.0.1:1234", 400},
		{"/table/sort?column=0&direction=asc", "example.com", 403},
	} {
		r := httptest.NewRequest(http.MethodPost, "/s/"+s.Token()+tc.path, nil)
		r.Host = tc.host
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		if w.Code != tc.status {
			t.Fatalf("%s: %d", tc.path, w.Code)
		}
	}
}
