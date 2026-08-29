// Package feed fetches, parses, and discovers RSS/Atom feeds and their
// icons — the feed itself, not article or podcast content.
package feed

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mmcdole/gofeed"

	"wisp/internal/api"
	"wisp/internal/httpx"
)

// FetchAndParse fetches feedURL and parses it into a title, an inferred
// FeedKind, and its items. Kind is computed once here (any item with an
// audio enclosure means Podcast) rather than left to each caller.
func FetchAndParse(ctx context.Context, feedURL string) (api.ParsedFeed, error) {
	ctx, cancel := context.WithTimeout(ctx, httpx.DefaultTimeout)
	defer cancel()

	body, _, err := httpx.FetchCapped(ctx, feedURL, httpx.MaxBytes)
	if err != nil {
		return api.ParsedFeed{}, fmt.Errorf("fetch feed %s: %w", feedURL, err)
	}

	parsed, err := gofeed.NewParser().Parse(bytes.NewReader(body))
	if err != nil {
		return api.ParsedFeed{}, fmt.Errorf("parse feed %s: %w", feedURL, err)
	}

	items := make([]api.Item, 0, len(parsed.Items))
	hasAudio := false
	for _, entry := range parsed.Items {
		transcriptURL, transcriptType := podcastTranscript(entry)
		guid := entry.GUID
		if guid == "" {
			guid = entry.Link
		}
		item := api.Item{
			GUID:           guid,
			Title:          entry.Title,
			Link:           entry.Link,
			PubDate:        pubDate(entry),
			Description:    entry.Description,
			ContentEncoded: entry.Content,
			AudioURL:       audioEnclosure(entry),
			TranscriptURL:  transcriptURL,
			TranscriptType: transcriptType,
		}
		if item.AudioURL != "" {
			hasAudio = true
		}
		items = append(items, item)
	}

	kind := api.FeedKindArticle
	if hasAudio {
		kind = api.FeedKindPodcast
	}

	return api.ParsedFeed{
		Title:    parsed.Title,
		Kind:     kind,
		SiteLink: parsed.Link,
		Items:    items,
	}, nil
}

func pubDate(entry *gofeed.Item) string {
	if entry.PublishedParsed != nil {
		return entry.PublishedParsed.Format(time.RFC3339)
	}
	return entry.Published
}

func audioEnclosure(entry *gofeed.Item) string {
	for _, enc := range entry.Enclosures {
		if strings.HasPrefix(enc.Type, "audio") {
			return enc.URL
		}
	}
	return ""
}

// parseableTranscriptTypes ranks formats internal/podcast.FetchTranscript
// can actually parse; feeds don't reliably list their preferred format
// first (Buzzsprout lists text/html before text/vtt), so document order
// alone isn't a usable signal.
var parseableTranscriptTypes = []string{"vtt", "srt", "subrip", "plain"}

// podcastTranscript reads the Podcasting 2.0 <podcast:transcript> tag, if
// present, preferring a format that's actually parseable over whichever
// the feed lists first.
func podcastTranscript(entry *gofeed.Item) (url, mimeType string) {
	matches := entry.Extensions["podcast"]["transcript"]
	if len(matches) == 0 {
		return "", ""
	}
	for _, want := range parseableTranscriptTypes {
		for _, m := range matches {
			if strings.Contains(m.Attrs["type"], want) {
				return m.Attrs["url"], m.Attrs["type"]
			}
		}
	}
	return matches[0].Attrs["url"], matches[0].Attrs["type"]
}
