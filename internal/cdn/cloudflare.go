package cdn

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

var cloudflareAPIBase = "https://api.cloudflare.com/client/v4"

// CloudflarePurger clears cached URLs by prefix in a Cloudflare zone.
type CloudflarePurger struct {
	ZoneID     string
	APIToken   string
	HTTPClient *http.Client
}

func NewCloudflarePurger(zoneID, apiToken string) *CloudflarePurger {
	return &CloudflarePurger{
		ZoneID:   strings.TrimSpace(zoneID),
		APIToken: strings.TrimSpace(apiToken),
		HTTPClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func (p *CloudflarePurger) PurgePrefixes(ctx context.Context, prefixes []string) error {
	prefixes = normalizePrefixes(prefixes)
	if len(prefixes) == 0 {
		return nil
	}
	return p.purge(ctx, map[string][]string{"prefixes": prefixes})
}

func (p *CloudflarePurger) PurgeURLs(ctx context.Context, urls []string) error {
	urls = normalizeURLs(urls)
	if len(urls) == 0 {
		return nil
	}
	return p.purge(ctx, map[string][]string{"files": urls})
}

func (p *CloudflarePurger) purge(ctx context.Context, body map[string][]string) error {
	if p == nil {
		return nil
	}
	if p.ZoneID == "" || p.APIToken == "" {
		return fmt.Errorf("cloudflare zone id and api token are required")
	}

	rawBody, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("encoding purge request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cloudflareAPIBase+"/zones/"+p.ZoneID+"/purge_cache", bytes.NewReader(rawBody))
	if err != nil {
		return fmt.Errorf("creating purge request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+p.APIToken)
	req.Header.Set("Content-Type", "application/json")

	client := p.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("purging cloudflare cache: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("reading purge response: %w", err)
	}

	var payload struct {
		Success bool `json:"success"`
		Errors  []struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return fmt.Errorf("decoding purge response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 || !payload.Success {
		if len(payload.Errors) > 0 {
			return fmt.Errorf("cloudflare purge failed: %s", payload.Errors[0].Message)
		}
		return fmt.Errorf("cloudflare purge failed: status %d", resp.StatusCode)
	}
	return nil
}

func normalizePrefixes(prefixes []string) []string {
	out := make([]string, 0, len(prefixes))
	seen := make(map[string]struct{}, len(prefixes))
	for _, prefix := range prefixes {
		prefix = strings.TrimSpace(prefix)
		if prefix == "" {
			continue
		}
		prefix = strings.TrimPrefix(prefix, "https://")
		prefix = strings.TrimPrefix(prefix, "http://")
		if !strings.HasSuffix(prefix, "/") {
			prefix += "/"
		}
		if _, ok := seen[prefix]; ok {
			continue
		}
		seen[prefix] = struct{}{}
		out = append(out, prefix)
	}
	return out
}

func normalizeURLs(urls []string) []string {
	out := make([]string, 0, len(urls))
	seen := make(map[string]struct{}, len(urls))
	for _, u := range urls {
		u = strings.TrimSpace(u)
		if u == "" {
			continue
		}
		if !strings.HasPrefix(u, "https://") && !strings.HasPrefix(u, "http://") {
			u = "https://" + u
		}
		if _, ok := seen[u]; ok {
			continue
		}
		seen[u] = struct{}{}
		out = append(out, u)
	}
	return out
}
