package cdn

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCloudflarePurgerPurgeHosts(t *testing.T) {
	t.Run("sends hostname purge request", func(t *testing.T) {
		var got struct {
			Hosts []string `json:"hosts"`
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Fatalf("method = %s, want POST", r.Method)
			}
			if r.URL.Path != "/client/v4/zones/zone123/purge_cache" {
				t.Fatalf("path = %s", r.URL.Path)
			}
			if got := r.Header.Get("Authorization"); got != "Bearer token123" {
				t.Fatalf("authorization = %q", got)
			}
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(body, &got); err != nil {
				t.Fatal(err)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
		}))
		defer srv.Close()

		p := NewCloudflarePurger("zone123", "token123")
		p.HTTPClient = srv.Client()

		orig := cloudflareAPIBase
		t.Cleanup(func() { cloudflareAPIBase = orig })
		cloudflareAPIBase = srv.URL + "/client/v4"

		if err := p.PurgeHosts(context.Background(), []string{"alice.waldi.blog"}); err != nil {
			t.Fatal(err)
		}
		if len(got.Hosts) != 1 || got.Hosts[0] != "alice.waldi.blog" {
			t.Fatalf("hosts = %#v", got.Hosts)
		}
	})

	t.Run("no-op on empty hosts", func(t *testing.T) {
		p := NewCloudflarePurger("zone123", "token123")
		if err := p.PurgeHosts(context.Background(), nil); err != nil {
			t.Fatal(err)
		}
	})
}

func TestNormalizeHosts(t *testing.T) {
	got := normalizeHosts([]string{
		"alice.waldi.blog",
		"https://waldi.blog",
		"https://waldi.blog/",
		"",
	})
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2: %#v", len(got), got)
	}
	if got[0] != "alice.waldi.blog" || got[1] != "waldi.blog" {
		t.Fatalf("hosts = %#v", got)
	}
}
