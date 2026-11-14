package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"twikko/internal/schema"
)

func TestHistoryEndpointsAndCompletion(t *testing.T) {
	s := testServer(t)
	post(t, s, "/entities/0/fields/summary", "value=a&history_group=visit")
	post(t, s, "/entities/0/fields/summary", "value=after&history_group=visit")
	response := post(t, s, "/undo", "")
	if s.store.ChangedCount() != 0 || !strings.Contains(response, `hx-swap-oob="true"`) || !strings.Contains(response, `id="save-status"`) {
		t.Fatalf("undo did not restore data and UI: %s", response)
	}
	post(t, s, "/redo", "")
	if schema.MapValue(s.store.Entity(0), "summary").Value != "after" {
		t.Fatal("redo failed")
	}
	post(t, s, "/reset", "")
	if s.store.ChangedCount() != 0 {
		t.Fatal("global reset failed")
	}
	post(t, s, "/undo", "")
	if s.store.ChangedCount() != 1 {
		t.Fatal("global reset not undoable")
	}
	// Reloading reads the same server session, without losing history.
	s.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/s/"+s.Token()+"/", nil))
	post(t, s, "/undo", "")
	if s.store.ChangedCount() != 0 {
		t.Fatal("reload lost history")
	}
	post(t, s, "/save", "")
	for _, action := range []string{"undo", "redo", "reset"} {
		request := httptest.NewRequest(http.MethodPost, "/s/"+s.Token()+"/"+action, nil)
		request.Host = "127.0.0.1:1234"
		response := httptest.NewRecorder()
		s.ServeHTTP(response, request)
		if response.Code != http.StatusConflict {
			t.Fatalf("%s accepted after save", action)
		}
	}
}

func TestHistoryRejectsWrongHost(t *testing.T) {
	s := testServer(t)
	post(t, s, "/entities/0/fields/summary", "value=after")
	for _, action := range []string{"undo", "redo", "reset"} {
		request := httptest.NewRequest(http.MethodPost, "/s/"+s.Token()+"/"+action, nil)
		request.Host = "example.com"
		response := httptest.NewRecorder()
		s.ServeHTTP(response, request)
		if response.Code != http.StatusForbidden {
			t.Fatalf("%s accepted wrong host", action)
		}
	}
	if s.store.ChangedCount() != 1 {
		t.Fatal("rejected action changed data")
	}
}
