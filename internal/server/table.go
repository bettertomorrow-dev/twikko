package server

import (
	"encoding/json"
	"net/http"
	"time"

	"twikko/internal/ui"
)

func (s *Server) sortTable(w http.ResponseWriter, r *http.Request) {
	order, err := s.store.SortedIndices(r.URL.Query().Get("column"), r.URL.Query().Get("direction"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(map[string]any{"order": order})
}

func (s *Server) tableHighlights(w http.ResponseWriter, r *http.Request) {
	if s.store.Mode() != "table" {
		http.Error(w, "highlights require table mode", http.StatusBadRequest)
		return
	}
	now := time.Now()
	highlights := make(map[int]string, s.store.EntityCount())
	for i := 0; i < s.store.EntityCount(); i++ {
		highlights[i] = ui.RowHighlightClass(s.store.RowHighlight(i, now))
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(map[string]any{"highlights": highlights})
}
