package cdn

import "context"

// Purger clears cached responses at an edge CDN.
type Purger interface {
	PurgePrefixes(ctx context.Context, prefixes []string) error
	PurgeURLs(ctx context.Context, urls []string) error
}
