package db

import (
	"context"

	"wisp/internal/api"
)

type Store interface {
	UpsertFeed(ctx context.Context, url, title string, kind api.FeedKind) (api.Feed, error)

	ListFeeds(ctx context.Context) ([]api.Feed, error)

	GetFeed(ctx context.Context, feedID int64) (*api.Feed, error)

	UpdateFeed(ctx context.Context, feedID int64, title, url string) (api.Feed, error)

	SetFeedIcon(ctx context.Context, feedID int64, data []byte, mimeType string) error

	DeleteFeed(ctx context.Context, feedID int64) error

	UpsertItems(ctx context.Context, feedID int64, items []api.Item) error

	ListItems(ctx context.Context, feedID *int64, limit, offset int) ([]api.Item, error)

	CountItems(ctx context.Context, feedID *int64) (int, error)

	GetItem(ctx context.Context, itemID int64) (*api.Item, error)

	SetItemDownload(ctx context.Context, itemID int64, filename string) error

	Close() error
}
