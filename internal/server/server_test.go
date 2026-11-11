package server

import (
	"net/http"
	"net/http/httptest"
	"os"
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

func headerServer(t *testing.T, header string) *Server {
	t.Helper()
	doc, err := schema.Parse([]byte(`
schema:
  id_field: key
  header:
` + header + `
  fields:
    - {name: key, type: text, readonly: true}
    - {name: status, type: enum, options: [To Do, Done]}
entities:
  - {key: A-1, type: Bug, title: Fix login, status: To Do}
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

const testHeader = `    - {type: icon, source: type, variants: {Bug: {icon: bug, intent: danger}}}
    - {type: tag, source: key}
    - {type: text, source: title, style: {bold: true}}
    - {type: tag, source: status, variants: {Done: {intent: success}}}`

func TestHeaderWidgetsRenderInOrder(t *testing.T) {
	server := headerServer(t, testHeader)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/s/"+server.Token()+"/", nil))
	body := response.Body.String()
	last := -1
	for _, want := range []string{`aria-label="Bug"`, "icons/bug.svg", ">A-1</span>", ">Fix login</span>", ">To Do</span>"} {
		at := strings.Index(body, want)
		if at < 0 || at < last {
			t.Fatalf("%q missing or out of order in header: %s", want, body)
		}
		last = at
	}
	if !strings.Contains(body, "Collapse Fix login") {
		t.Error("the toggle label must use the first text widget")
	}
}

func TestHeaderFollowsEdits(t *testing.T) {
	server := headerServer(t, testHeader)
	edited := post(t, server, "/entities/0/fields/status", "value=Done")
	if !strings.Contains(edited, `id="widget-0-h-3"`) || !strings.Contains(edited, `hx-swap-oob="true"`) {
		t.Fatalf("an edit must refresh the header out of band: %s", edited)
	}
	if !strings.Contains(edited, ">Done</span>") || !strings.Contains(edited, "bg-success/10") {
		t.Fatalf("the status tag must show the new value: %s", edited)
	}
}

func TestEditWithoutHeaderLeavesTitleAlone(t *testing.T) {
	server := testServer(t)
	edited := post(t, server, "/entities/0/fields/summary", "value=after")
	if strings.Contains(edited, "widget-0-") {
		t.Fatalf("a card without a header must not get header widgets: %s", edited)
	}
}

func bodyServer(t *testing.T, body string) *Server {
	t.Helper()
	doc, err := schema.Parse([]byte(`
schema:
  id_field: key
  body:
` + body + `
  fields:
    - {name: key, type: text, readonly: true}
    - {name: summary, type: longtext}
    - {name: owner, type: text}
    - {name: status, type: enum, options: [To Do, Done]}
entities:
  - {key: A-1, summary: Fix login, owner: Dana, status: To Do}
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

const testBody = `
    - - {type: text, source: summary, label: What}
    - - {source: status, label: State}
      - {type: tag, source: status, label: Now, variants: {Done: {intent: success}}}
`

func get(t *testing.T, server *Server) string {
	t.Helper()
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/s/"+server.Token()+"/", nil))
	return response.Body.String()
}

func TestBodyRendersRowsAndHidesUnplacedFields(t *testing.T) {
	body := get(t, bodyServer(t, testBody))
	last := -1
	for _, want := range []string{">What</span>", ">Fix login</span>", "State", `id="field-0-status"`, ">Now</span>", ">To Do</span>"} {
		at := strings.Index(body, want)
		if at < 0 || at < last {
			t.Fatalf("%q missing or out of order in body: %s", want, body)
		}
		last = at
	}
	for _, hidden := range []string{`id="field-0-owner"`, `id="field-0-summary"`, `id="field-0-key"`} {
		if strings.Contains(body, hidden) {
			t.Errorf("an unplaced field must not render: %s", hidden)
		}
	}
}

func TestBodyDisplayCellsFollowEditsAndControlsStay(t *testing.T) {
	server := bodyServer(t, testBody)
	edited := post(t, server, "/entities/0/fields/status", "value=Done")
	if !strings.Contains(edited, `id="widget-0-1-1"`) || !strings.Contains(edited, `hx-swap-oob="true"`) {
		t.Fatalf("an edit must refresh display cells out of band: %s", edited)
	}
	if !strings.Contains(edited, ">Done</span>") || !strings.Contains(edited, "bg-success/10") {
		t.Fatalf("the tag must show the new status: %s", edited)
	}
	if strings.Contains(edited, `id="field-0-status"`) {
		t.Fatalf("the control being edited must not be re-rendered: %s", edited)
	}
}

func TestBodyEditOfHiddenFieldIsStillRefusedWhenReadonly(t *testing.T) {
	server := bodyServer(t, testBody)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/s/"+server.Token()+"/entities/0/fields/key", strings.NewReader("value=B-9"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Host = "127.0.0.1:1234"
	server.ServeHTTP(response, request)
	if response.Code == http.StatusOK {
		t.Fatalf("a read-only field must stay read-only: %s", response.Body.String())
	}
}

func TestBodyKeepsHiddenFieldsInTheOutput(t *testing.T) {
	server := bodyServer(t, testBody)
	post(t, server, "/entities/0/fields/status", "value=Done")
	post(t, server, "/save", "")
	event := <-server.Done()
	if event.Err != nil || event.Status != "saved" {
		t.Fatalf("unexpected event: %#v", event)
	}
	data, err := os.ReadFile(server.outputPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "owner: Dana") {
		t.Fatalf("a hidden field must stay in the output: %s", data)
	}
}

func TestEditWithoutBodyHasNoDisplayCells(t *testing.T) {
	server := testServer(t)
	if edited := post(t, server, "/entities/0/fields/summary", "value=after"); strings.Contains(edited, "widget-0-") {
		t.Fatalf("a card without a body must get no display cells: %s", edited)
	}
}

func controlServer(t *testing.T, layout string) *Server {
	t.Helper()
	doc, err := schema.Parse([]byte(`
schema:
  id_field: key
` + layout + `
  fields:
    - {name: key, type: text, readonly: true}
    - {name: team, type: text}
    - {name: notes, type: text}
    - {name: blocked, type: bool}
    - {name: at, type: time}
    - {name: when, type: datetime}
    - {name: status, type: enum, options: [To Do, Done]}
entities:
  - {key: A-1, team: Backend, notes: hi, blocked: false, at: "14:30", when: 2026-10-20T14:30, status: To Do}
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

const controlLayout = `  header:
    - {type: tag, source: key}
    - {source: status}
    - {type: tag, source: status, label: Now}
  body:
    - - {type: select, source: team, options: [Backend, Frontend]}
      - {type: textarea, source: notes}
    - - {type: toggle, source: blocked, label: Blocked}
      - {source: at}
      - {source: when}
`

func TestEachControlKindRenders(t *testing.T) {
	page := get(t, controlServer(t, controlLayout))
	for _, want := range []string{
		`<select id="field-0-status"`, `<select id="field-0-team"`, `<textarea id="field-0-notes"`,
		`type="checkbox" role="switch"`, ">Blocked</span>", `type="time"`, `type="datetime-local"`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("page lacks %q", want)
		}
	}
	if !strings.Contains(page, `aria-label="status"`) || strings.Contains(page, `for="field-0-status"`) {
		t.Error("a header control carries its label as an accessible name and shows no label element")
	}
	if !strings.Contains(page, `for="field-0-team"`) {
		t.Error("a body control shows its label")
	}
}

func TestHeaderControlEditsAndLeavesItselfAlone(t *testing.T) {
	server := controlServer(t, controlLayout)
	edited := post(t, server, "/entities/0/fields/status", "value=Done")
	if strings.Contains(edited, `id="field-0-status"`) {
		t.Fatalf("the control being edited must not be re-rendered: %s", edited)
	}
	for _, want := range []string{`id="entity-badges-0"`, `id="widget-0-h-2"`, ">Done</span>", `id="save-bar"`} {
		if !strings.Contains(edited, want) {
			t.Errorf("edit response lacks %q: %s", want, edited)
		}
	}
}

func TestCollapsedCardBadgeFollowsAHeaderEdit(t *testing.T) {
	server := controlServer(t, controlLayout)
	post(t, server, "/entities/0/collapsed", "collapsed=true")
	edited := post(t, server, "/entities/0/fields/status", "value=Done")
	if !strings.Contains(edited, "1 changed") {
		t.Fatalf("the badge must show the change without expanding: %s", edited)
	}
}

func TestSelectOverTextRefusesOtherValues(t *testing.T) {
	server := controlServer(t, controlLayout)
	edited := post(t, server, "/entities/0/fields/team", "value=Ops")
	if !strings.Contains(edited, "must be one of: Backend, Frontend") {
		t.Fatalf("a value outside the options must be refused: %s", edited)
	}
	if ok := post(t, server, "/entities/0/fields/team", "value=Frontend"); strings.Contains(ok, "must be one of") {
		t.Fatalf("an offered value must be accepted: %s", ok)
	}
}

func TestToggleSwitchesOnAndOff(t *testing.T) {
	server := controlServer(t, controlLayout)
	if on := post(t, server, "/entities/0/fields/blocked", "value=on"); !strings.Contains(on, "Changed") {
		t.Fatalf("turning the switch on is a change: %s", on)
	}
	if off := post(t, server, "/entities/0/fields/blocked", ""); strings.Contains(off, "Changed") {
		t.Fatalf("turning it back off restores the input value: %s", off)
	}
}

func TestTimeAndDateTimeAreValidated(t *testing.T) {
	server := controlServer(t, controlLayout)
	if bad := post(t, server, "/entities/0/fields/at", "value=25%3A99"); !strings.Contains(bad, "must be a time") {
		t.Fatalf("a bad time must be refused: %s", bad)
	}
	if bad := post(t, server, "/entities/0/fields/when", "value=yesterday"); !strings.Contains(bad, "must be a date and time") {
		t.Fatalf("a bad date-time must be refused: %s", bad)
	}
	if ok := post(t, server, "/entities/0/fields/when", "value=2026-10-21T09%3A15"); strings.Contains(ok, "must be") {
		t.Fatalf("a good date-time must be accepted: %s", ok)
	}
}
