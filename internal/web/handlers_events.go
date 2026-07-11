package web

import (
	"encoding/json"
	"net/http"
)

type readingEventRequest struct {
	ImpressionID int64 `json:"impression_id"`
	MaxScrollPct int   `json:"max_scroll_pct"`
	DwellSeconds int   `json:"dwell_seconds"`
	Completed    bool  `json:"completed"`
}

func (s *Server) handleReadingEvent(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	var req readingEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if !validReadingEvent(req) {
		http.Error(w, "bad event", http.StatusBadRequest)
		return
	}
	if err := s.store.UpsertReading(r.Context(), req.ImpressionID, req.MaxScrollPct, req.DwellSeconds, req.Completed); err != nil {
		s.logger.Error("recording reading event", "err", err)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func validReadingEvent(req readingEventRequest) bool {
	if req.ImpressionID < 1 {
		return false
	}
	if req.MaxScrollPct < 0 || req.MaxScrollPct > 100 {
		return false
	}
	if req.DwellSeconds < 0 || req.DwellSeconds > 24*60*60 {
		return false
	}
	return true
}
