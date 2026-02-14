package tools

import "context"

// SearchProvider abstracts concrete web-search providers.
type SearchProvider interface {
	Name() string
	Search(ctx context.Context, query string, count int) (string, error)
}

// BuildSearchProviders resolves provider chain and max-results policy.
func BuildSearchProviders(opts WebSearchToolOptions) (primary SearchProvider, fallback SearchProvider, maxResults int) {
	maxResults = 5

	if opts.BraveAPIKey != "" {
		primary = NewBraveSearchProvider(opts.BraveAPIKey)
		if opts.BraveMaxResults > 0 {
			maxResults = opts.BraveMaxResults
		}
		if opts.DuckDuckGoEnabled {
			fallback = NewDuckDuckGoSearchProvider()
		}
	} else if opts.DuckDuckGoEnabled {
		primary = NewDuckDuckGoSearchProvider()
		if opts.DuckDuckGoMaxResults > 0 {
			maxResults = opts.DuckDuckGoMaxResults
		}
	}

	if maxResults <= 0 || maxResults > 10 {
		maxResults = 5
	}
	return primary, fallback, maxResults
}
