package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPublicStateAnonymous(t *testing.T) {
	s := &Server{}
	r := httptest.NewRequest(http.MethodGet, "/api/public-state?username=sara", nil)
	w := httptest.NewRecorder()

	s.handlePublicState(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if got := w.Header().Get("Cache-Control"); got != "private, no-store" {
		t.Fatalf("Cache-Control = %q", got)
	}
	var state publicStateResponse
	if err := json.NewDecoder(w.Body).Decode(&state); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if state.Authenticated {
		t.Fatal("anonymous state reported as authenticated")
	}
}
