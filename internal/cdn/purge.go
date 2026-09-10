package cdn

import "context"

// Purger clears cached responses at an edge CDN.
type Purger interface {
	PurgeHosts(ctx context.Context, hosts []string) error
	PurgeURLs(ctx context.Context, urls []string) error
}
