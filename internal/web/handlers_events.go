package web

import (
	"encoding/json"
	"errors"
	"net/http"
	"waldi/internal/store"
)

type impressionEventRequest struct {
	PostID int64  `json:"post_id"`
	Source string `json:"source"`
}

func (s *Server) handleImpressionEvent(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	var req impressionEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.PostID < 1 {
		http.Error(w, "bad event", http.StatusBadRequest)
		return
	}
	if s.store == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	post, err := s.store.PublishedPostByID(r.Context(), req.PostID)
	if errors.Is(err, store.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		s.logger.Error("loading post for impression", "post_id", req.PostID, "err", err)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if user := currentUser(r); user != nil && user.ID == post.UserID {
		_ = json.NewEncoder(w).Encode(map[string]int64{"impression_id": 0})
		return
	}

	readerKey, cookie := readerKeyFromRequest(r, s.baseDomain)
	if cookie != nil {
		http.SetCookie(w, cookie)
	}
	var userID *int64
	if user := currentUser(r); user != nil {
		id := user.ID
		userID = &id
	}

	source := impressionSource(req.Source)
	id, err := s.store.EnsureImpression(r.Context(), post.ID, readerKey, userID, source)
	if err != nil {
		s.logger.Error("creating impression", "post_id", post.ID, "source", source, "err", err)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if source == store.ImpressionWildcard && userID != nil {
		if err := s.store.MarkWildcardOpened(r.Context(), *userID, post.ID, today()); err != nil {
			s.logger.Error("marking wildcard opened", "post_id", post.ID, "user_id", *userID, "err", err)
		}
	}
	_ = json.NewEncoder(w).Encode(map[string]int64{"impression_id": id})
}

func impressionSource(raw string) store.ImpressionSource {
	switch raw {
	case "feed":
		return store.ImpressionFeed
	case "wildcard":
		return store.ImpressionWildcard
	default:
		return store.ImpressionDirect
	}
}

type readingEventRequest struct {
	ImpressionID int64 `json:"impression_id"`
	MaxScrollPct int   `json:"max_scroll_pct"`
	DwellSeconds int   `json:"dwell_seconds"`
	Completed    bool  `json:"completed"`
}

func (s *Server) handleReadingEvent(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
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
