package api

type FeedKind int

const (
	FeedKindPodcast FeedKind = iota
	FeedKindArticle
)

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

type PodcastResult struct {
	Title      string `json:"title"`
	Author     string `json:"author"`
	FeedURL    string `json:"feedUrl"`
	ArtworkURL string `json:"artworkUrl"`
}

type DiscoveredFeed struct {
	Title string `json:"title"`
	URL   string `json:"url"`
}
