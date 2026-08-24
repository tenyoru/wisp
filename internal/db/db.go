package db

import (
	"context"

	"wisp/internal/api"
)

type Store interface {
	UpsertFeed(ctx context.Context, url, title string, kind api.FeedKind) (api.Feed, error)

	ListFeeds(ctx context.Context) ([]api.Feed, error)

	GetFeed(ctx context.Context, feedID int64) (*api.Feed, error)

	SetFeedIcon(ctx context.Context, feedID int64, data []byte, mimeType string) error

	DeleteFeed(ctx context.Context, feedID int64) error

	UpsertItems(ctx context.Context, feedID int64, items []api.Item) error

	ListItems(ctx context.Context, feedID *int64) ([]api.Item, error)

	GetItem(ctx context.Context, itemID int64) (*api.Item, error)

	Close() error
}
