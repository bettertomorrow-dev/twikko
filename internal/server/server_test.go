package server

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"multieditor/internal/schema"
	"multieditor/internal/session"
)

func testServer(t *testing.T) *Server {
	t.Helper()
	doc, err := schema.Parse([]byte(`
schema:
  id_field: key
  fields:
    - {name: key, type: text}
    - {name: summary, type: text, required: true}
entities: [{key: A-1, summary: before}]
`))
	if err != nil {
		t.Fatal(err)
	}
	server, err := New(session.New(doc), filepath.Join(t.TempDir(), "out.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	server.SetAllowedHost("127.0.0.1:1234")
	return server
}

func TestTokenAndHostChecks(t *testing.T) {
	server := testServer(t)
	request := httptest.NewRequest(http.MethodGet, "/s/wrong/", nil)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("unexpected status: %d", response.Code)
	}

	request = httptest.NewRequest(http.MethodPost, "/s/"+server.Token()+"/save", nil)
	request.Host = "example.com"
	response = httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("unexpected status: %d", response.Code)
	}
}

func TestEditAndSave(t *testing.T) {
	server := testServer(t)
	edit := httptest.NewRequest(http.MethodPost, "/s/"+server.Token()+"/entities/0/fields/summary", strings.NewReader("value=after"))
	edit.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	edit.Host = "127.0.0.1:1234"
	response := httptest.NewRecorder()
	server.ServeHTTP(response, edit)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "hx-swap-oob") {
		t.Fatalf("unexpected edit response: %d %s", response.Code, response.Body.String())
	}

	save := httptest.NewRequest(http.MethodPost, "/s/"+server.Token()+"/save", nil)
	save.Host = "127.0.0.1:1234"
	response = httptest.NewRecorder()
	server.ServeHTTP(response, save)
	if response.Code != http.StatusOK {
		t.Fatalf("unexpected save status: %d", response.Code)
	}
	event := <-server.Done()
	if event.Status != "saved" || event.Err != nil {
		t.Fatalf("unexpected event: %#v", event)
	}
}

func post(t *testing.T, server *Server, path, body string) string {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/s/"+server.Token()+path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Host = "127.0.0.1:1234"
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("%s: status %d: %s", path, response.Code, response.Body.String())
	}
	return response.Body.String()
}

func TestCollapseToggleHidesFieldsAndKeepsChangeBadge(t *testing.T) {
	server := testServer(t)
	post(t, server, "/entities/0/fields/summary", "value=after")

	collapsed := post(t, server, "/entities/0/collapsed", "collapsed=true")
	if strings.Contains(collapsed, `name="value"`) {
		t.Fatal("collapsed card must not render field inputs")
	}
	if !strings.Contains(collapsed, `aria-expanded="false"`) || !strings.Contains(collapsed, "Expand A-1") {
		t.Fatalf("collapsed card must offer to expand: %s", collapsed)
	}
	if !strings.Contains(collapsed, "1 changed") || !strings.Contains(collapsed, "Reset") {
		t.Fatalf("collapsed card lost its badge or menu: %s", collapsed)
	}

	reset := post(t, server, "/entities/0/reset", "")
	if strings.Contains(reset, `name="value"`) {
		t.Fatal("reset must keep the card collapsed")
	}

	expanded := post(t, server, "/entities/0/collapsed", "collapsed=false")
	if !strings.Contains(expanded, `name="value"`) {
		t.Fatal("collapsed=false must expand the card")
	}
	if !strings.Contains(expanded, `aria-expanded="true"`) || !strings.Contains(expanded, "Collapse A-1") {
		t.Fatalf("expanded card must offer to collapse: %s", expanded)
	}
}

func TestCardTitleFromTitleField(t *testing.T) {
	doc, err := schema.Parse([]byte(`
schema:
  id_field: key
  title_field: card_title
  fields:
    - {name: key, type: text, readonly: true}
entities:
  - {key: A-1, card_title: "A-1 · Fix login"}
  - {key: A-2}
`))
	if err != nil {
		t.Fatal(err)
	}
	server, err := New(session.New(doc), filepath.Join(t.TempDir(), "out.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/s/"+server.Token()+"/", nil)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	body := response.Body.String()
	if !strings.Contains(body, ">A-1 · Fix login</h2>") {
		t.Fatalf("custom title missing: %s", body)
	}
	if !strings.Contains(body, ">A-2</h2>") {
		t.Fatal("entity without a title value must fall back to its identifier")
	}
}

func TestAssetsServeOnlyKnownFiles(t *testing.T) {
	server := testServer(t)
	for path, want := range map[string]int{
		"/assets/styles.css":         http.StatusOK,
		"/assets/icons/eye.svg":      http.StatusOK,
		"/assets/icons/missing.svg":  http.StatusNotFound,
		"/assets/icons/../app.js":    http.StatusTemporaryRedirect, // the mux cleans the path before the handler
		"/assets/icons/eye.svg/more": http.StatusNotFound,
		"/assets/icons/LICENSE":      http.StatusNotFound,
	} {
		response := httptest.NewRecorder()
		server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/s/"+server.Token()+path, nil))
		if response.Code != want {
			t.Errorf("%s: got %d, want %d", path, response.Code, want)
		}
	}
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/s/"+server.Token()+"/assets/icons/eye.svg", nil))
	if got := response.Header().Get("Content-Type"); got != "image/svg+xml" {
		t.Errorf("icon content type: %q", got)
	}
}
