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

const eventFeedRefreshed = "feed-refreshed"

const eventEpisodeDownload = "episode-download"

func init() {
	application.RegisterEvent[FeedRefreshResult](eventFeedRefreshed)
	application.RegisterEvent[EpisodeDownloadEvent](eventEpisodeDownload)
}

const refreshTimeout = 30 * time.Second

const downloadTimeout = 30 * time.Minute

type FeedRefreshResult struct {
	FeedID int64    `json:"feedId"`
	Feed   api.Feed `json:"feed,omitempty"`
	Error  string   `json:"error,omitempty"`
}

type EpisodeDownloadEvent struct {
	ItemID           int64  `json:"itemId"`
	Downloaded       int64  `json:"downloaded"`
	Total            int64  `json:"total"`
	Done             bool   `json:"done"`
	DownloadFilename string `json:"downloadFilename,omitempty"`
	Error            string `json:"error,omitempty"`
}

type FeedService struct {
	store db.Store
	emit func(event string, data ...any)
}

func (s *FeedService) emitEvent(event string, data ...any) {
	if s.emit != nil {
		s.emit(event, data...)
	}
}

func (s *FeedService) AddFeed(ctx context.Context, feedURL string) (api.Feed, error) {
	return s.fetchAndStore(ctx, normalizeURL(feedURL), "")
}

func (s *FeedService) AddFeedFromSearch(ctx context.Context, feedURL, artworkURL string) (api.Feed, error) {
	return s.fetchAndStore(ctx, normalizeURL(feedURL), artworkURL)
}

func normalizeURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if u, err := url.Parse(raw); err != nil || u.Scheme == "" {
		return "https://" + raw
	}
	return raw
}

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
	feed, err := s.store.GetFeed(ctx, feedID)
	if err != nil {
		return err
	}
	if feed != nil {
		_ = podcast.DeletePodcastDir(feed.Title)
	}
	return s.store.DeleteFeed(ctx, feedID)
}

func (s *FeedService) GetFeed(ctx context.Context, feedID int64) (api.Feed, error) {
	f, err := s.store.GetFeed(ctx, feedID)
	if err != nil {
		return api.Feed{}, err
	}
	if f == nil {
		return api.Feed{}, fmt.Errorf("no such feed: %d", feedID)
	}
	return *f, nil
}

func (s *FeedService) UpdateFeed(ctx context.Context, feedID int64, title, url string) (api.Feed, error) {
	updated, err := s.store.UpdateFeed(ctx, feedID, title, normalizeURL(url))
	if err != nil {
		return api.Feed{}, err
	}
	s.refreshInBackground(updated.ID, updated.URL)
	return updated, nil
}

func (s *FeedService) ItemCount(ctx context.Context, feedID int64) (int, error) {
	return s.store.CountItems(ctx, &feedID)
}

func (s *FeedService) ListItems(ctx context.Context, feedID int64, limit, offset int) ([]api.Item, error) {
	return s.store.ListItems(ctx, &feedID, limit, offset)
}

func (s *FeedService) ItemMarkdown(ctx context.Context, itemID int64) (string, error) {
	item, err := s.store.GetItem(ctx, itemID)
	if err != nil {
		return "", err
	}
	if item == nil {
		return "", fmt.Errorf("no such item: %d", itemID)
	}
	if item.AudioURL != "" {
		if item.TranscriptURL != "" {
			if text, err := podcast.FetchTranscript(ctx, item.TranscriptURL, item.TranscriptType); err == nil && strings.TrimSpace(text) != "" {
				return text, nil
			}
		}
		return article.ResolveShowNotes(item.Link, item.ContentEncoded, item.Description)
	}
	return article.ResolveArticleMarkdown(item.Link, item.ContentEncoded, item.Description)
}

func (s *FeedService) SearchPodcasts(ctx context.Context, term string) ([]api.PodcastResult, error) {
	return podcast.SearchPodcasts(ctx, term)
}

func (s *FeedService) DiscoverFeeds(ctx context.Context, siteURL string) ([]api.DiscoveredFeed, error) {
	return feed.DiscoverFeedURLs(ctx, normalizeURL(siteURL))
}

func (s *FeedService) DownloadEpisode(ctx context.Context, itemID int64) error {
	item, err := s.store.GetItem(ctx, itemID)
	if err != nil {
		return err
	}
	if item == nil {
		return fmt.Errorf("no such item: %d", itemID)
	}
	if item.AudioURL == "" {
		return fmt.Errorf("item %d has no audio", itemID)
	}
	feed, err := s.store.GetFeed(ctx, item.FeedID)
	if err != nil {
		return err
	}
	if feed == nil {
		return fmt.Errorf("no such feed: %d", item.FeedID)
	}

	go func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), downloadTimeout)
		defer cancel()

		relPath, err := podcast.DownloadEpisode(bgCtx, feed.Title, item.Title, item.AudioURL, func(downloaded, total int64) {
			s.emitEvent(eventEpisodeDownload, EpisodeDownloadEvent{ItemID: itemID, Downloaded: downloaded, Total: total})
		})
		if err != nil {
			s.emitEvent(eventEpisodeDownload, EpisodeDownloadEvent{ItemID: itemID, Error: err.Error()})
			return
		}
		if err := s.store.SetItemDownload(bgCtx, itemID, relPath); err != nil {
			s.emitEvent(eventEpisodeDownload, EpisodeDownloadEvent{ItemID: itemID, Error: err.Error()})
			return
		}
		s.emitEvent(eventEpisodeDownload, EpisodeDownloadEvent{ItemID: itemID, Done: true, DownloadFilename: relPath})
	}()
	return nil
}

func (s *FeedService) DeleteDownload(ctx context.Context, itemID int64) error {
	item, err := s.store.GetItem(ctx, itemID)
	if err != nil {
		return err
	}
	if item == nil || item.DownloadFilename == "" {
		return nil
	}
	if err := podcast.DeleteEpisode(item.DownloadFilename); err != nil {
		return err
	}
	return s.store.SetItemDownload(ctx, itemID, "")
}
