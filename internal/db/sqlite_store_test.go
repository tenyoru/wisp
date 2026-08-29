package db

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"

	"wisp/internal/api"
)

func openTestStore(t *testing.T) *SQLiteStore {
	t.Helper()
	path := filepath.Join(t.TempDir(), "wisp.db")
	store, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func TestUpsertFeed(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)

	f, err := store.UpsertFeed(ctx, "https://example.com/feed.xml", "Example", api.FeedKindArticle)
	if err != nil {
		t.Fatalf("UpsertFeed: %v", err)
	}
	if f.ID == 0 {
		t.Fatal("expected a non-zero ID")
	}

	// Re-upserting the same URL updates in place rather than duplicating.
	f2, err := store.UpsertFeed(ctx, "https://example.com/feed.xml", "Example Renamed", api.FeedKindPodcast)
	if err != nil {
		t.Fatalf("UpsertFeed (update): %v", err)
	}
	if f2.ID != f.ID {
		t.Errorf("ID changed on re-upsert: %d -> %d", f.ID, f2.ID)
	}
	if f2.Title != "Example Renamed" || f2.Kind != api.FeedKindPodcast {
		t.Errorf("update didn't take: got %+v", f2)
	}

	if len(f2.Icon) != 0 {
		t.Errorf("Icon = %v, want empty (never set)", f2.Icon)
	}

	feeds, err := store.ListFeeds(ctx)
	if err != nil {
		t.Fatalf("ListFeeds: %v", err)
	}
	if len(feeds) != 1 {
		t.Fatalf("len(ListFeeds) = %d, want 1", len(feeds))
	}
}

func TestGetFeed(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)

	created, err := store.UpsertFeed(ctx, "https://example.com/feed.xml", "Example", api.FeedKindArticle)
	if err != nil {
		t.Fatalf("UpsertFeed: %v", err)
	}

	got, err := store.GetFeed(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetFeed: %v", err)
	}
	if got == nil || got.URL != created.URL {
		t.Fatalf("GetFeed = %+v, want %+v", got, created)
	}

	missing, err := store.GetFeed(ctx, 99999)
	if err != nil {
		t.Fatalf("GetFeed(missing): %v", err)
	}
	if missing != nil {
		t.Errorf("GetFeed(missing) = %+v, want nil", missing)
	}
}

func TestSetFeedIcon(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)

	feed, err := store.UpsertFeed(ctx, "https://example.com/feed.xml", "Example", api.FeedKindArticle)
	if err != nil {
		t.Fatalf("UpsertFeed: %v", err)
	}

	icon := []byte{0x89, 0x50, 0x4e, 0x47} // fake PNG magic bytes, contents don't matter here
	if err := store.SetFeedIcon(ctx, feed.ID, icon, "image/png"); err != nil {
		t.Fatalf("SetFeedIcon: %v", err)
	}

	feeds, err := store.ListFeeds(ctx)
	if err != nil {
		t.Fatalf("ListFeeds: %v", err)
	}
	if len(feeds) != 1 || !bytes.Equal(feeds[0].Icon, icon) {
		t.Fatalf("ListFeeds()[0].Icon = %v, want %v", feeds[0].Icon, icon)
	}
	if feeds[0].IconMime != "image/png" {
		t.Errorf("IconMime = %q, want %q", feeds[0].IconMime, "image/png")
	}

	// Re-upserting (e.g. a title/kind refresh) must not wipe the icon that
	// was set separately.
	if _, err := store.UpsertFeed(ctx, "https://example.com/feed.xml", "Example Renamed", api.FeedKindArticle); err != nil {
		t.Fatalf("UpsertFeed (refresh): %v", err)
	}
	feeds, err = store.ListFeeds(ctx)
	if err != nil {
		t.Fatalf("ListFeeds: %v", err)
	}
	if !bytes.Equal(feeds[0].Icon, icon) {
		t.Errorf("icon was cleared by an unrelated UpsertFeed: got %v, want %v", feeds[0].Icon, icon)
	}
}

func TestUpsertItemsAndList(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)

	feed, err := store.UpsertFeed(ctx, "https://example.com/feed.xml", "Example", api.FeedKindArticle)
	if err != nil {
		t.Fatalf("UpsertFeed: %v", err)
	}

	items := []api.Item{
		{GUID: "guid-1", Title: "First", Link: "https://example.com/1", PubDate: "2026-01-01T00:00:00Z", Description: "one"},
		{GUID: "guid-2", Title: "Second", Link: "https://example.com/2", PubDate: "2026-01-02T00:00:00Z", Description: "two"},
	}
	if err := store.UpsertItems(ctx, feed.ID, items); err != nil {
		t.Fatalf("UpsertItems: %v", err)
	}

	// Refetching the same feed updates existing items by guid rather than
	// duplicating them.
	updated := []api.Item{
		{GUID: "guid-1", Title: "First (edited)", Link: "https://example.com/1", PubDate: "2026-01-01T00:00:00Z", Description: "one edited"},
	}
	if err := store.UpsertItems(ctx, feed.ID, updated); err != nil {
		t.Fatalf("UpsertItems (update): %v", err)
	}

	got, err := store.ListItems(ctx, &feed.ID)
	if err != nil {
		t.Fatalf("ListItems: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(ListItems) = %d, want 2 (no duplicate from re-upsert)", len(got))
	}
	// Newest first.
	if got[0].Title != "Second" {
		t.Errorf("ListItems[0].Title = %q, want %q (newest first)", got[0].Title, "Second")
	}
	if got[1].Title != "First (edited)" {
		t.Errorf("ListItems[1].Title = %q, want the edited title", got[1].Title)
	}
	for _, it := range got {
		if it.FeedID != feed.ID {
			t.Errorf("item %q FeedID = %d, want %d", it.Title, it.FeedID, feed.ID)
		}
		if it.ID == 0 {
			t.Errorf("item %q has zero ID", it.Title)
		}
	}

	all, err := store.ListItems(ctx, nil)
	if err != nil {
		t.Fatalf("ListItems(nil): %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("len(ListItems(nil)) = %d, want 2", len(all))
	}

	one, err := store.GetItem(ctx, got[0].ID)
	if err != nil {
		t.Fatalf("GetItem: %v", err)
	}
	if one == nil || one.Title != got[0].Title {
		t.Errorf("GetItem = %+v, want %+v", one, got[0])
	}

	missing, err := store.GetItem(ctx, 99999)
	if err != nil {
		t.Fatalf("GetItem(missing): %v", err)
	}
	if missing != nil {
		t.Errorf("GetItem(missing) = %+v, want nil", missing)
	}
}

func TestDeleteFeedCascadesItems(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)

	feed, err := store.UpsertFeed(ctx, "https://example.com/feed.xml", "Example", api.FeedKindArticle)
	if err != nil {
		t.Fatalf("UpsertFeed: %v", err)
	}
	err = store.UpsertItems(ctx, feed.ID, []api.Item{
		{Title: "First", Link: "https://example.com/1", PubDate: "2026-01-01T00:00:00Z"},
	})
	if err != nil {
		t.Fatalf("UpsertItems: %v", err)
	}

	if err := store.DeleteFeed(ctx, feed.ID); err != nil {
		t.Fatalf("DeleteFeed: %v", err)
	}

	items, err := store.ListItems(ctx, nil)
	if err != nil {
		t.Fatalf("ListItems: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("len(ListItems) = %d after DeleteFeed, want 0 (cascade)", len(items))
	}
}

func TestUpdateFeed(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)

	feed, err := store.UpsertFeed(ctx, "https://example.com/feed.xml", "Example", api.FeedKindArticle)
	if err != nil {
		t.Fatalf("UpsertFeed: %v", err)
	}

	renamed, err := store.UpdateFeed(ctx, feed.ID, "My Name", "https://example.com/new-feed.xml")
	if err != nil {
		t.Fatalf("UpdateFeed: %v", err)
	}
	if renamed.Title != "My Name" || renamed.URL != "https://example.com/new-feed.xml" {
		t.Errorf("UpdateFeed = %+v, want title/URL to take", renamed)
	}

	// A refresh's UpsertFeed writes the feed's own title but must not
	// clobber the override.
	refreshed, err := store.UpsertFeed(ctx, renamed.URL, "Feed-Supplied Title", api.FeedKindArticle)
	if err != nil {
		t.Fatalf("UpsertFeed (refresh): %v", err)
	}
	if refreshed.Title != "My Name" {
		t.Errorf("Title after refresh = %q, want override %q to survive", refreshed.Title, "My Name")
	}

	// Clearing the override reverts to the feed-supplied title.
	cleared, err := store.UpdateFeed(ctx, feed.ID, "", renamed.URL)
	if err != nil {
		t.Fatalf("UpdateFeed (clear): %v", err)
	}
	if cleared.Title != "Feed-Supplied Title" {
		t.Errorf("Title after clearing override = %q, want feed-supplied %q", cleared.Title, "Feed-Supplied Title")
	}
}

func TestOpenIsReopenable(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "wisp.db")

	store1, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if _, err := store1.UpsertFeed(ctx, "https://example.com/feed.xml", "Example", api.FeedKindArticle); err != nil {
		t.Fatalf("UpsertFeed: %v", err)
	}
	store1.Close()

	store2, err := Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer store2.Close()

	feeds, err := store2.ListFeeds(ctx)
	if err != nil {
		t.Fatalf("ListFeeds after reopen: %v", err)
	}
	if len(feeds) != 1 {
		t.Fatalf("len(ListFeeds) after reopen = %d, want 1 (data should persist)", len(feeds))
	}
}
