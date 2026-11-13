package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/a-h/templ"
	"twikko/internal/icons"
	"twikko/internal/output"
	"twikko/internal/schema"
	"twikko/internal/session"
	"twikko/internal/ui"
)

// assetName lists what /assets/ serves: the stylesheet, the scripts and the
// vendored icons.
var assetName = regexp.MustCompile(`^(styles\.css|htmx\.min\.js|app\.js|icons/[a-z0-9-]+\.svg)$`)

type Event struct {
	Status string
	Err    error
}

type Server struct {
	// Serialize edits and completion for this single-user session.
	mu          sync.Mutex
	finished    bool
	store       *session.Store
	outputPath  string
	token       string
	allowedHost string
	done        chan Event
	mux         *http.ServeMux
}

func New(store *session.Store, outputPath string) (*Server, error) {
	token, err := randomToken()
	if err != nil {
		return nil, err
	}
	server := &Server{
		store:      store,
		outputPath: outputPath,
		token:      token,
		done:       make(chan Event, 1),
		mux:        http.NewServeMux(),
	}
	server.mux.HandleFunc("/", server.handle)
	return server, nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) Token() string { return s.token }

func (s *Server) Done() <-chan Event { return s.done }

func (s *Server) SetAllowedHost(host string) { s.allowedHost = host }

func (s *Server) URL(addr string) string {
	return "http://" + addr + "/s/" + s.token + "/"
}

func (s *Server) Cancel() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.finish(output.Cancelled())
}

func (s *Server) handle(w http.ResponseWriter, r *http.Request) {
	prefix := "/s/" + s.token + "/"
	if !strings.HasPrefix(r.URL.Path, prefix) {
		http.NotFound(w, r)
		return
	}
	relative := strings.TrimPrefix(r.URL.Path, prefix)
	if strings.HasPrefix(relative, "assets/") {
		s.serveAsset(w, r, strings.TrimPrefix(relative, "assets/"))
		return
	}
	switch {
	case r.Method == http.MethodGet && relative == "":
		s.render(w, ui.Page(ui.NewPageData(s.store, s.token)))
	case r.Method == http.MethodPost && relative == "save":
		s.requireWriteHost(w, r, s.save)
	case r.Method == http.MethodPost && relative == "cancel":
		s.requireWriteHost(w, r, s.cancel)
	case r.Method == http.MethodPost && strings.HasPrefix(relative, "entities/"):
		s.requireWriteHost(w, r, s.entityAction)
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) serveAsset(w http.ResponseWriter, r *http.Request, name string) {
	if !assetName.MatchString(name) {
		http.NotFound(w, r)
		return
	}
	var data []byte
	var err error
	if icon, ok := strings.CutPrefix(name, "icons/"); ok {
		data, err = icons.Read(strings.TrimSuffix(icon, ".svg"))
	} else {
		data, err = ui.Assets.ReadFile("static/" + name)
	}
	if err != nil {
		http.NotFound(w, r)
		return
	}
	switch {
	case strings.HasSuffix(name, ".css"):
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
	case strings.HasSuffix(name, ".svg"):
		w.Header().Set("Content-Type", "image/svg+xml")
	default:
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	}
	_, _ = w.Write(data)
}

func (s *Server) requireWriteHost(w http.ResponseWriter, r *http.Request, next func(http.ResponseWriter, *http.Request)) {
	if s.allowedHost != "" && r.Host != s.allowedHost {
		http.Error(w, "invalid Host header", http.StatusForbidden)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.finished {
		http.Error(w, "session has ended", http.StatusConflict)
		return
	}
	next(w, r)
}

func (s *Server) entityAction(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.EscapedPath(), "/s/"+s.token+"/"), "/")
	if len(parts) < 3 || parts[0] != "entities" {
		http.NotFound(w, r)
		return
	}
	index, err := strconv.Atoi(parts[1])
	if err != nil || index < 0 || index >= s.store.EntityCount() {
		http.NotFound(w, r)
		return
	}
	if parts[2] == "reset" && len(parts) == 3 {
		if err := s.store.Reset(index); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		data := ui.NewPageData(s.store, s.token)
		if data.Mode == "table" {
			s.render(w, ui.TableRow(data, data.Entity(index)))
		} else {
			s.render(w, ui.EntityCard(data, data.Entity(index)))
		}
		s.renderOOBSaveBar(w, data)
		return
	}
	if parts[2] == "collapsed" && len(parts) == 3 {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}
		if err := s.store.SetCollapsed(index, r.Form.Get("collapsed") == "true"); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		data := ui.NewPageData(s.store, s.token)
		s.render(w, ui.EntityCard(data, data.Entity(index)))
		return
	}
	if len(parts) != 4 || parts[2] != "fields" {
		http.NotFound(w, r)
		return
	}
	name, err := url.PathUnescape(parts[3])
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	if err := s.store.Edit(index, name, r.Form["value"], (r.Form.Has("value") || r.Form.Get("present") == "true")); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	data := ui.NewPageData(s.store, s.token)
	field, ok := findField(data.Entity(index), name)
	if !ok {
		http.NotFound(w, r)
		return
	}
	s.render(w, ui.FieldStatus(index, field))
	if data.Mode == "table" {
		for _, cell := range data.Entity(index).Cells {
			if cell.Field != nil && cell.Field.Field.Name == name && (cell.Type == schema.WidgetTextarea || cell.Type == schema.WidgetMultiSelect) {
				s.render(w, ui.CellSummary(index, field, true))
			}
		}
	}
	// Out of band: the display widgets that show the edited value, and the badges
	// of a collapsed card. Controls are left alone, so the one being typed in
	// keeps its focus.
	entity := data.Entity(index)
	s.render(w, ui.DisplaySlots(data, entity))
	if data.Mode != "table" {
		s.render(w, ui.EntityBadges(entity, true))
	}
	s.renderOOBSaveBar(w, data)
}

func (s *Server) save(w http.ResponseWriter, r *http.Request) {
	if s.store.HasErrors() {
		http.Error(w, "fix validation errors before saving", http.StatusUnprocessableEntity)
		return
	}
	if err := s.finish(output.Saved(s.store, time.Now())); err != nil {
		http.Error(w, "write output: "+err.Error(), http.StatusInternalServerError)
		return
	}
	s.render(w, ui.DonePage(true))
}

func (s *Server) cancel(w http.ResponseWriter, r *http.Request) {
	if err := s.finish(output.Cancelled()); err != nil {
		http.Error(w, "write output: "+err.Error(), http.StatusInternalServerError)
		return
	}
	s.render(w, ui.DonePage(false))
}

func (s *Server) render(w io.Writer, component templ.Component) {
	if err := component.Render(context.Background(), w); err != nil {
		return
	}
}

func (s *Server) renderOOBSaveBar(w io.Writer, data ui.PageData) {
	s.render(w, ui.SaveBar(data, true))
}

// finish runs with mu held. Failed writes leave the session open for retry.
func (s *Server) finish(result output.Result) error {
	if s.finished {
		return nil
	}
	if err := output.WriteAtomic(s.outputPath, result); err != nil {
		return err
	}
	s.finished = true
	s.done <- Event{Status: result.Status}
	return nil
}

func randomToken() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate session token: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}

func findField(entity ui.EntityView, name string) (ui.FieldView, bool) {
	for _, field := range entity.Fields {
		if field.Field.Name == name {
			return field, true
		}
	}
	return ui.FieldView{}, false
}
