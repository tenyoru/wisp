// Package db defines the Store interface and its SQLite implementation.
package db

import (
	"context"

	"wisp/internal/api"
)

type Store interface {
	// UpsertFeed updates title/kind if url is already subscribed.
	UpsertFeed(ctx context.Context, url, title string, kind api.FeedKind) (api.Feed, error)

	ListFeeds(ctx context.Context) ([]api.Feed, error)

	// GetFeed returns nil, nil if feedID doesn't exist.
	GetFeed(ctx context.Context, feedID int64) (*api.Feed, error)

	// UpdateFeed sets feedID's display title (empty clears the override) and URL.
	UpdateFeed(ctx context.Context, feedID int64, title, url string) (api.Feed, error)

	// SetFeedIcon: nil data clears the icon.
	SetFeedIcon(ctx context.Context, feedID int64, data []byte, mimeType string) error

	// DeleteFeed also removes every item belonging to it.
	DeleteFeed(ctx context.Context, feedID int64) error

	UpsertItems(ctx context.Context, feedID int64, items []api.Item) error

	// ListItems: nil feedID lists across every feed.
	ListItems(ctx context.Context, feedID *int64) ([]api.Item, error)

	GetItem(ctx context.Context, itemID int64) (*api.Item, error)

	Close() error
}
