package web

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleSetLocaleAuto(t *testing.T) {
	s := testServer(t)
	req := httptest.NewRequest(http.MethodPost, "/lang/fa?auto=1", nil)
	req.Host = "waldi.blog"
	req.SetPathValue("code", "fa")
	rec := httptest.NewRecorder()

	s.handleSetLocale(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("expected empty body, got %q", rec.Body.String())
	}

	found := false
	for _, c := range rec.Result().Cookies() {
		if c.Name == localeCookie {
			found = true
			if c.Value != "fa" {
				t.Fatalf("cookie value = %q, want fa", c.Value)
			}
		}
	}
	if !found {
		t.Fatal("expected waldi_lang cookie to be set")
	}
}

func TestHandleSetLocaleAutoUnsupportedLang(t *testing.T) {
	s := testServer(t)
	req := httptest.NewRequest(http.MethodPost, "/lang/xx?auto=1", nil)
	req.Host = "waldi.blog"
	req.SetPathValue("code", "xx")
	rec := httptest.NewRecorder()

	s.handleSetLocale(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}
