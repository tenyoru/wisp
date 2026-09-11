package api

// FeedKind is inferred from content, not trusted from caller input.
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
	ID               int64  `json:"id"`
	FeedID           int64  `json:"feedId"`
	GUID             string `json:"guid"` // stable identity; Link is often absent or shared
	Title            string `json:"title"`
	Link             string `json:"link"`
	PubDate          string `json:"pubDate"`
	AudioURL         string `json:"audioUrl"` // empty => article
	Description      string `json:"description"`
	ContentEncoded   string `json:"contentEncoded"`
	DownloadFilename string `json:"downloadFilename,omitempty"`
	TranscriptURL    string `json:"transcriptUrl,omitempty"`
	TranscriptType   string `json:"transcriptType,omitempty"`
}

type ParsedFeed struct {
	Title    string
	Kind     FeedKind
	SiteLink string // channel website, not the feed URL
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

type Settings struct {
	Theme              string  `json:"theme"`         // "dark" or "light"
	PlaybackSpeed      float64 `json:"playbackSpeed"` // 0.5–3
	WhisperModel       string  `json:"whisperModel"`  // tiny, base, small, medium, large
	CloudTranscription bool    `json:"cloudTranscription"`
}

func DefaultSettings() Settings {
	return Settings{Theme: "dark", PlaybackSpeed: 1, WhisperModel: "base"}
}

func (s Settings) Clamp() Settings {
	if s.Theme != "light" {
		s.Theme = "dark"
	}
	if s.PlaybackSpeed < 0.5 || s.PlaybackSpeed > 3 {
		s.PlaybackSpeed = 1
	}
	switch s.WhisperModel {
	case "tiny", "base", "small", "medium", "large":
	default:
		s.WhisperModel = "base"
	}
	return s
}
