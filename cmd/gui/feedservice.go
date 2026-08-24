package main

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"wisp/internal/api"
	"wisp/internal/article"
	"wisp/internal/db"
	"wisp/internal/feed"
	"wisp/internal/podcast"
)

// eventFeedRefreshed fires once a background RefreshFeed completes.
const eventFeedRefreshed = "feed-refreshed"

func init() {
	application.RegisterEvent[FeedRefreshResult](eventFeedRefreshed)
}

// refreshTimeout is independent of any context passed to RefreshFeed
// itself — that context is gone before the goroutine it started finishes.
const refreshTimeout = 30 * time.Second

// FeedRefreshResult is the payload of the "feed-refreshed" event.
type FeedRefreshResult struct {
	FeedID int64    `json:"feedId"`
	Feed   api.Feed `json:"feed,omitempty"`
	Error  string   `json:"error,omitempty"`
}

// FeedService is exposed to the frontend via Wails bindings.
type FeedService struct {
	store db.Store
	// emit defaults to the live app's emitter (set in main.go); overridden
	// in tests so they don't depend on a running application.App.
	emit func(event string, data ...any)
}

func (s *FeedService) emitEvent(event string, data ...any) {
	if s.emit != nil {
		s.emit(event, data...)
	}
}

// AddFeed fetches and parses feedURL, then persists the feed and its items.
func (s *FeedService) AddFeed(ctx context.Context, feedURL string) (api.Feed, error) {
	return s.fetchAndStore(ctx, normalizeURL(feedURL), "")
}

// AddFeedFromSearch is AddFeed but uses artworkURL directly as the icon
// instead of the favicon-discovery site scan, which often finds nothing
// for a podcast's own site.
func (s *FeedService) AddFeedFromSearch(ctx context.Context, feedURL, artworkURL string) (api.Feed, error) {
	return s.fetchAndStore(ctx, normalizeURL(feedURL), artworkURL)
}

// normalizeURL prepends "https://" when raw has no scheme, so pasting a
// bare domain (e.g. "example.com/feed") works the same as a full URL.
func normalizeURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if u, err := url.Parse(raw); err != nil || u.Scheme == "" {
		return "https://" + raw
	}
	return raw
}

// RefreshFeed queues a background refetch and returns immediately; the
// result arrives later via the "feed-refreshed" event.
//
// ponytail: one refresh in flight per feed isn't enforced — a double-click
// can queue two overlapping fetches. Upsert idempotency makes it harmless
// today, just wasteful; add de-duplication if that changes.
func (s *FeedService) RefreshFeed(ctx context.Context, feedID int64) error {
	existing, err := s.store.GetFeed(ctx, feedID)
	if err != nil {
		return err
	}
	if existing == nil {
		return fmt.Errorf("no such feed: %d", feedID)
	}

	s.refreshInBackground(existing.ID, existing.URL)
	return nil
}

// RefreshAllFeeds queues an independent refresh per feed, so one slow or
// failing feed doesn't block or fail the others.
func (s *FeedService) RefreshAllFeeds(ctx context.Context) error {
	feeds, err := s.store.ListFeeds(ctx)
	if err != nil {
		return err
	}
	for _, f := range feeds {
		s.refreshInBackground(f.ID, f.URL)
	}
	return nil
}

func (s *FeedService) refreshInBackground(feedID int64, feedURL string) {
	go func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), refreshTimeout)
		defer cancel()

		updated, err := s.fetchAndStore(bgCtx, feedURL, "")
		if err != nil {
			s.emitEvent(eventFeedRefreshed, FeedRefreshResult{FeedID: feedID, Error: err.Error()})
			return
		}
		s.emitEvent(eventFeedRefreshed, FeedRefreshResult{FeedID: feedID, Feed: updated})
	}()
}

// fetchAndStore fetches feedURL, upserts the feed, and best-effort stores
// an icon if it doesn't have one yet.
//
// feedURL must already be a feed — resolving a webpage to its feed URL(s)
// is DiscoverFeeds' job, kept out of here deliberately since a page can
// advertise more than one and silently picking one would take that choice
// away from the caller.
//
// iconURLHint, if non-empty, is fetched directly as the icon instead of
// running favicon discovery (see AddFeedFromSearch).
func (s *FeedService) fetchAndStore(ctx context.Context, feedURL, iconURLHint string) (api.Feed, error) {
	parsed, err := feed.FetchAndParse(ctx, feedURL)
	if err != nil {
		return api.Feed{}, err
	}

	stored, err := s.store.UpsertFeed(ctx, feedURL, parsed.Title, parsed.Kind)
	if err != nil {
		return api.Feed{}, err
	}

	if err := s.store.UpsertItems(ctx, stored.ID, parsed.Items); err != nil {
		return api.Feed{}, err
	}

	if len(stored.Icon) == 0 {
		var data []byte
		var mimeType string
		var iconErr error
		if iconURLHint != "" {
			data, mimeType, iconErr = feed.FetchDirectIcon(ctx, iconURLHint)
		} else {
			siteURL := parsed.SiteLink
			if siteURL == "" {
				siteURL = feedURL
			}
			data, mimeType, iconErr = feed.FetchIcon(ctx, siteURL)
		}
		if iconErr == nil {
			if err := s.store.SetFeedIcon(ctx, stored.ID, data, mimeType); err == nil {
				stored.Icon = data
				stored.IconMime = mimeType
			}
		}
	}

	return stored, nil
}

func (s *FeedService) ListFeeds(ctx context.Context) ([]api.Feed, error) {
	return s.store.ListFeeds(ctx)
}

func (s *FeedService) DeleteFeed(ctx context.Context, feedID int64) error {
	return s.store.DeleteFeed(ctx, feedID)
}

// ItemCount reports how many items are stored for feedID, so the UI can
// show a count without fetching every item's full fields.
func (s *FeedService) ItemCount(ctx context.Context, feedID int64) (int, error) {
	items, err := s.store.ListItems(ctx, &feedID)
	if err != nil {
		return 0, err
	}
	return len(items), nil
}

// ListItems returns feedID's items, newest first.
func (s *FeedService) ListItems(ctx context.Context, feedID int64) ([]api.Item, error) {
	return s.store.ListItems(ctx, &feedID)
}

func (s *FeedService) ItemMarkdown(ctx context.Context, itemID int64) (string, error) {
	item, err := s.store.GetItem(ctx, itemID)
	if err != nil {
		return "", err
	}
	if item == nil {
		return "", fmt.Errorf("no such item: %d", itemID)
	}
	return article.ResolveArticleMarkdown(item.Link, item.ContentEncoded, item.Description)
}

// SearchPodcasts queries iTunes for podcasts matching term.
func (s *FeedService) SearchPodcasts(ctx context.Context, term string) ([]api.PodcastResult, error) {
	return podcast.SearchPodcasts(ctx, term)
}

// DiscoverFeeds scans siteURL's HTML for feed links, for when AddFeed
// fails because siteURL is a webpage rather than a feed directly.
func (s *FeedService) DiscoverFeeds(ctx context.Context, siteURL string) ([]api.DiscoveredFeed, error) {
	return feed.DiscoverFeedURLs(ctx, normalizeURL(siteURL))
}
