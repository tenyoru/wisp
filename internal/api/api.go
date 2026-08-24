// Package api holds the shared data model used across the app.
package api

// FeedKind is inferred from content, not trusted from caller input.
type FeedKind int

const (
	FeedKindPodcast FeedKind = iota
	FeedKindArticle
)

// Feed is a subscribed feed as stored in internal/db.
type Feed struct {
	ID       int64    `json:"id"`
	URL      string   `json:"url"`
	Title    string   `json:"title"`
	Kind     FeedKind `json:"kind"`
	Icon     []byte   `json:"icon,omitempty"` // []byte auto-marshals to base64
	IconMime string   `json:"iconMime,omitempty"`
}

type Item struct {
	ID             int64  `json:"id"`
	FeedID         int64  `json:"feedId"`
	Title          string `json:"title"`
	Link           string `json:"link"`
	PubDate        string `json:"pubDate"`        // RFC3339, best-effort from the feed
	AudioURL       string `json:"audioUrl"`       // empty => article, not a podcast episode
	Description    string `json:"description"`    // short teaser, always feed-supplied
	ContentEncoded string `json:"contentEncoded"` // full body if the feed included one; empty otherwise
}

type ParsedFeed struct {
	Title    string
	Kind     FeedKind
	SiteLink string // channel website link, distinct from the feed URL; may be empty
	Items    []Item
}

// PodcastResult is one match from an iTunes podcast search.
type PodcastResult struct {
	Title      string `json:"title"`
	Author     string `json:"author"`
	FeedURL    string `json:"feedUrl"`
	ArtworkURL string `json:"artworkUrl"`
}

// DiscoveredFeed is one feed link found by internal/feed.DiscoverFeedURLs.
type DiscoveredFeed struct {
	Title string `json:"title"`
	URL   string `json:"url"`
}
