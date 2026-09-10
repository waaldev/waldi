package web

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"waldi/internal/store"
)

type publicStateResponse struct {
	Authenticated bool `json:"authenticated"`
	Owner         bool `json:"owner"`
	CanFollow     bool `json:"can_follow"`
	Following     bool `json:"following"`
	CanSendLetter bool `json:"can_send_letter"`
}

func (s *Server) handlePublicState(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	viewer := currentUser(r)
	if viewer == nil {
		_ = json.NewEncoder(w).Encode(publicStateResponse{})
		return
	}
	if s.store == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	username := strings.TrimSpace(r.URL.Query().Get("username"))
	if username == "" {
		http.Error(w, "missing username", http.StatusBadRequest)
		return
	}
	owner, err := s.store.UserByUsername(r.Context(), username)
	if errors.Is(err, store.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		s.logger.Error("loading public state owner", "username", username, "err", err)
		http.Error(w, "state unavailable", http.StatusInternalServerError)
		return
	}

	state := publicStateResponse{Authenticated: true, Owner: viewer.ID == owner.ID}
	if state.Owner {
		_ = json.NewEncoder(w).Encode(state)
		return
	}

	state.CanFollow = true
	state.Following, err = s.store.IsFollowing(r.Context(), viewer.ID, owner.ID)
	if err != nil {
		s.logger.Error("loading public follow state", "viewer_id", viewer.ID, "owner_id", owner.ID, "err", err)
	}
	if r.URL.Query().Has("post") {
		completed, err := s.store.CompletedReadingsCount(r.Context(), viewer.ID)
		if err != nil {
			s.logger.Error("loading public letter state", "viewer_id", viewer.ID, "err", err)
		} else {
			state.CanSendLetter = completed >= minCompletedReadingsForLetters
		}
	}

	_ = json.NewEncoder(w).Encode(state)
}
