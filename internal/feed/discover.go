package feed

import (
	"context"
	"fmt"
	"strings"

	"wisp/internal/api"
)

// DiscoverFeedURLs returns every feed siteURL's HTML advertises via
// <link rel="alternate" type="application/rss+xml|atom+xml"> — a page can
// have more than one (e.g. separate blog/photo feeds), so this doesn't
// guess which one the caller wants.
func DiscoverFeedURLs(ctx context.Context, siteURL string) ([]api.DiscoveredFeed, error) {
	matches, err := findAllLinkHrefs(ctx, siteURL, func(attrs map[string]string) bool {
		if strings.ToLower(attrs["rel"]) != "alternate" {
			return false
		}
		typ := strings.ToLower(attrs["type"])
		return strings.Contains(typ, "rss") || strings.Contains(typ, "atom")
	})
	if err != nil {
		return nil, fmt.Errorf("discover feeds at %s: %w", siteURL, err)
	}

	results := make([]api.DiscoveredFeed, len(matches))
	for i, m := range matches {
		title := m.Title
		if title == "" {
			title = m.Href
		}
		results[i] = api.DiscoveredFeed{Title: title, URL: m.Href}
	}
	return results, nil
}
