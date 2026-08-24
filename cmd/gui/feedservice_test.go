package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"wisp/internal/api"
	"wisp/internal/db"
)

const fixtureFeed = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
<channel>
	<title>Fixture Feed</title>
	<item>
		<title>Episode One</title>
		<link>https://example.com/ep1</link>
		<description>An episode</description>
		<enclosure url="https://example.com/ep1.mp3" type="audio/mpeg" length="123"/>
	</item>
	<item>
		<title>Episode Two</title>
		<link>https://example.com/ep2</link>
		<description>Another episode</description>
		<enclosure url="https://example.com/ep2.mp3" type="audio/mpeg" length="123"/>
	</item>
</channel>
</rss>`

func newTestFeedService(t *testing.T) (*FeedService, *httptest.Server) {
	t.Helper()

	store, err := db.Open(filepath.Join(t.TempDir(), "wisp.db"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		w.Write([]byte(fixtureFeed))
	}))
	t.Cleanup(srv.Close)

	return &FeedService{store: store}, srv
}

func TestFeedServiceAddFeed(t *testing.T) {
	ctx := context.Background()
	svc, srv := newTestFeedService(t)

	feed, err := svc.AddFeed(ctx, srv.URL)
	if err != nil {
		t.Fatalf("AddFeed: %v", err)
	}
	if feed.Title != "Fixture Feed" {
		t.Errorf("Title = %q, want %q", feed.Title, "Fixture Feed")
	}
	if feed.Kind != api.FeedKindPodcast {
		t.Errorf("Kind = %v, want FeedKindPodcast", feed.Kind)
	}

	count, err := svc.ItemCount(ctx, feed.ID)
	if err != nil {
		t.Fatalf("ItemCount: %v", err)
	}
	if count != 2 {
		t.Errorf("ItemCount = %d, want 2", count)
	}
}

// TestFeedServiceAddFeedIconFailureIsNonFatal points the feed's site link
// at a host that refuses connections, forcing feed.FetchIcon to fail
// outright (not just return no icon) — AddFeed must still succeed.
func TestFeedServiceAddFeedIconFailureIsNonFatal(t *testing.T) {
	ctx := context.Background()

	store, err := db.Open(filepath.Join(t.TempDir(), "wisp.db"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	svc := &FeedService{store: store}

	const feedWithDeadSiteLink = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
<channel>
	<title>Fixture Feed</title>
	<link>http://127.0.0.1:1</link>
	<item>
		<title>Article One</title>
		<link>https://example.com/a1</link>
		<description>An article</description>
	</item>
</channel>
</rss>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		w.Write([]byte(feedWithDeadSiteLink))
	}))
	t.Cleanup(srv.Close)

	feed, err := svc.AddFeed(ctx, srv.URL)
	if err != nil {
		t.Fatalf("AddFeed: %v (icon fetch failure should not be fatal)", err)
	}
	if len(feed.Icon) != 0 {
		t.Errorf("Icon = %v, want empty (the site link is unreachable)", feed.Icon)
	}
}

func TestFeedServiceAddFeedIsIdempotent(t *testing.T) {
	ctx := context.Background()
	svc, srv := newTestFeedService(t)

	first, err := svc.AddFeed(ctx, srv.URL)
	if err != nil {
		t.Fatalf("AddFeed (first): %v", err)
	}
	second, err := svc.AddFeed(ctx, srv.URL)
	if err != nil {
		t.Fatalf("AddFeed (second): %v", err)
	}
	if first.ID != second.ID {
		t.Errorf("re-adding the same URL produced a new feed: %d -> %d", first.ID, second.ID)
	}

	feeds, err := svc.ListFeeds(ctx)
	if err != nil {
		t.Fatalf("ListFeeds: %v", err)
	}
	if len(feeds) != 1 {
		t.Fatalf("len(ListFeeds) = %d, want 1 (no duplicate feed)", len(feeds))
	}

	count, err := svc.ItemCount(ctx, first.ID)
	if err != nil {
		t.Fatalf("ItemCount: %v", err)
	}
	if count != 2 {
		t.Errorf("ItemCount after re-add = %d, want 2 (no duplicate items)", count)
	}
}

// TestFeedServiceRefreshFeed asserts RefreshFeed's actual contract: it
// returns before the network fetch completes (queuing, not doing, the
// work), and the result shows up later via the "feed-refreshed" event.
func TestFeedServiceRefreshFeed(t *testing.T) {
	ctx := context.Background()

	store, err := db.Open(filepath.Join(t.TempDir(), "wisp.db"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	events := make(chan FeedRefreshResult, 1)
	svc := &FeedService{
		store: store,
		emit: func(event string, data ...any) {
			if event != eventFeedRefreshed {
				return
			}
			result, ok := data[0].(FeedRefreshResult)
			if !ok {
				t.Errorf("emit(%q, ...) payload was %T, want FeedRefreshResult", event, data[0])
				return
			}
			events <- result
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		w.Write([]byte(fixtureFeed))
	}))
	t.Cleanup(srv.Close)

	added, err := svc.AddFeed(ctx, srv.URL)
	if err != nil {
		t.Fatalf("AddFeed: %v", err)
	}

	if err := svc.RefreshFeed(ctx, added.ID); err != nil {
		t.Fatalf("RefreshFeed: %v", err)
	}

	select {
	case result := <-events:
		if result.Error != "" {
			t.Fatalf("feed-refreshed reported an error: %s", result.Error)
		}
		if result.FeedID != added.ID {
			t.Errorf("result.FeedID = %d, want %d", result.FeedID, added.ID)
		}
		if result.Feed.Title != "Fixture Feed" {
			t.Errorf("result.Feed.Title = %q, want %q", result.Feed.Title, "Fixture Feed")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the feed-refreshed event")
	}

	// The refresh must not have duplicated the feed's items.
	count, err := svc.ItemCount(ctx, added.ID)
	if err != nil {
		t.Fatalf("ItemCount: %v", err)
	}
	if count != 2 {
		t.Errorf("ItemCount after refresh = %d, want 2 (no duplicates)", count)
	}
}

// TestFeedServiceRefreshAllFeeds adds two feeds, then kills one server
// before refreshing — asserting each feed reports its own independent
// feed-refreshed event (one success, one error) rather than one feed's
// failure blocking or dropping the other's result.
func TestFeedServiceRefreshAllFeeds(t *testing.T) {
	ctx := context.Background()

	store, err := db.Open(filepath.Join(t.TempDir(), "wisp.db"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	events := make(chan FeedRefreshResult, 2)
	svc := &FeedService{
		store: store,
		emit: func(event string, data ...any) {
			if event != eventFeedRefreshed {
				return
			}
			result, ok := data[0].(FeedRefreshResult)
			if !ok {
				t.Errorf("emit(%q, ...) payload was %T, want FeedRefreshResult", event, data[0])
				return
			}
			events <- result
		},
	}

	srvA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		w.Write([]byte(fixtureFeed))
	}))
	t.Cleanup(srvA.Close)
	srvB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		w.Write([]byte(fixtureFeed))
	}))

	feedA, err := svc.AddFeed(ctx, srvA.URL)
	if err != nil {
		t.Fatalf("AddFeed A: %v", err)
	}
	feedB, err := svc.AddFeed(ctx, srvB.URL)
	if err != nil {
		t.Fatalf("AddFeed B: %v", err)
	}

	srvB.Close() // B will fail to refresh; A must still succeed independently

	if err := svc.RefreshAllFeeds(ctx); err != nil {
		t.Fatalf("RefreshAllFeeds: %v", err)
	}

	results := map[int64]FeedRefreshResult{}
	for len(results) < 2 {
		select {
		case result := <-events:
			results[result.FeedID] = result
		case <-time.After(5 * time.Second):
			t.Fatalf("timed out waiting for both feed-refreshed events, got %d/2", len(results))
		}
	}

	if results[feedA.ID].Error != "" {
		t.Errorf("feed A reported an error: %s", results[feedA.ID].Error)
	}
	if results[feedB.ID].Error == "" {
		t.Error("feed B (server closed) should have reported an error, got none")
	}
}

func TestFeedServiceRefreshFeedRejectsUnknownID(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestFeedService(t)

	if err := svc.RefreshFeed(ctx, 99999); err == nil {
		t.Fatal("RefreshFeed(99999): expected an error for an unknown feed id")
	}
}

func TestFeedServiceDeleteFeed(t *testing.T) {
	ctx := context.Background()
	svc, srv := newTestFeedService(t)

	feed, err := svc.AddFeed(ctx, srv.URL)
	if err != nil {
		t.Fatalf("AddFeed: %v", err)
	}

	if err := svc.DeleteFeed(ctx, feed.ID); err != nil {
		t.Fatalf("DeleteFeed: %v", err)
	}

	feeds, err := svc.ListFeeds(ctx)
	if err != nil {
		t.Fatalf("ListFeeds: %v", err)
	}
	if len(feeds) != 0 {
		t.Errorf("len(ListFeeds) after DeleteFeed = %d, want 0", len(feeds))
	}
}

func TestFeedServiceListItems(t *testing.T) {
	ctx := context.Background()
	svc, srv := newTestFeedService(t)

	feed, err := svc.AddFeed(ctx, srv.URL)
	if err != nil {
		t.Fatalf("AddFeed: %v", err)
	}

	items, err := svc.ListItems(ctx, feed.ID)
	if err != nil {
		t.Fatalf("ListItems: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("len(ListItems) = %d, want 2", len(items))
	}
	titles := map[string]bool{items[0].Title: true, items[1].Title: true}
	if !titles["Episode One"] || !titles["Episode Two"] {
		t.Errorf("ListItems titles = %v, want Episode One and Episode Two", titles)
	}
	for _, it := range items {
		if it.FeedID != feed.ID {
			t.Errorf("item %q FeedID = %d, want %d", it.Title, it.FeedID, feed.ID)
		}
		if it.AudioURL == "" {
			t.Errorf("item %q AudioURL is empty, want an enclosure URL", it.Title)
		}
	}
}

// TestFeedServiceAddFeedFailsOnPlainWebpage pastes a homepage URL, not a
// feed URL — AddFeed no longer auto-discovers a feed from it (that choice
// now belongs to the caller via DiscoverFeeds, tested below), so it should
// fail rather than silently guessing.
func TestFeedServiceAddFeedFailsOnPlainWebpage(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestFeedService(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<!DOCTYPE html><html><head>
			<link rel="alternate" type="application/rss+xml" href="/feed.xml">
		</head><body></body></html>`))
	}))
	defer srv.Close()

	if _, err := svc.AddFeed(ctx, srv.URL); err == nil {
		t.Fatal("AddFeed on a plain webpage: expected an error, got none")
	}
}

// TestFeedServiceDiscoverFeeds_SingleMatch covers the common case: a page
// advertising exactly one feed.
func TestFeedServiceDiscoverFeeds_SingleMatch(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestFeedService(t)

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<!DOCTYPE html><html><head>
			<link rel="alternate" type="application/rss+xml" href="/feed.xml">
		</head><body></body></html>`))
	})
	mux.HandleFunc("/feed.xml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		w.Write([]byte(fixtureFeed))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	found, err := svc.DiscoverFeeds(ctx, srv.URL)
	if err != nil {
		t.Fatalf("DiscoverFeeds: %v", err)
	}
	if len(found) != 1 {
		t.Fatalf("len(DiscoverFeeds) = %d, want 1", len(found))
	}
	if found[0].URL != srv.URL+"/feed.xml" {
		t.Errorf("found[0].URL = %q, want %q", found[0].URL, srv.URL+"/feed.xml")
	}

	// The discovered URL is a real feed URL, so AddFeed on it (the second
	// step the frontend takes) must succeed.
	feed, err := svc.AddFeed(ctx, found[0].URL)
	if err != nil {
		t.Fatalf("AddFeed(%s): %v", found[0].URL, err)
	}
	if feed.Title != "Fixture Feed" {
		t.Errorf("Title = %q, want %q", feed.Title, "Fixture Feed")
	}
}

// TestFeedServiceDiscoverFeeds_MultipleMatches covers a page advertising
// more than one feed (e.g. separate blog/photo feeds) — DiscoverFeeds must
// return every one, with its title="..." attribute preserved, rather than
// picking one on the caller's behalf.
func TestFeedServiceDiscoverFeeds_MultipleMatches(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestFeedService(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<!DOCTYPE html><html><head>
			<link rel="alternate" type="application/rss+xml" title="Blog" href="/blog.xml">
			<link rel="alternate" type="application/rss+xml" title="Photos" href="/photo.xml">
		</head><body></body></html>`))
	}))
	defer srv.Close()

	found, err := svc.DiscoverFeeds(ctx, srv.URL)
	if err != nil {
		t.Fatalf("DiscoverFeeds: %v", err)
	}
	if len(found) != 2 {
		t.Fatalf("len(DiscoverFeeds) = %d, want 2", len(found))
	}
	if found[0].Title != "Blog" || found[0].URL != srv.URL+"/blog.xml" {
		t.Errorf("found[0] = %+v, unexpected", found[0])
	}
	if found[1].Title != "Photos" || found[1].URL != srv.URL+"/photo.xml" {
		t.Errorf("found[1] = %+v, unexpected", found[1])
	}
}

func TestNormalizeURL(t *testing.T) {
	cases := map[string]string{
		"example.com/feed.xml":     "https://example.com/feed.xml",
		"example.com":              "https://example.com",
		"http://example.com/feed":  "http://example.com/feed",
		"https://example.com/feed": "https://example.com/feed",
		"  example.com/feed.xml  ": "https://example.com/feed.xml",
	}
	for in, want := range cases {
		if got := normalizeURL(in); got != want {
			t.Errorf("normalizeURL(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestFeedServiceAddFeedFromSearchUsesArtwork asserts the icon comes from
// artworkURL directly, not from a favicon-discovery scan of the feed's own
// site (which the test deliberately leaves with no icon to find).
func TestFeedServiceAddFeedFromSearchUsesArtwork(t *testing.T) {
	ctx := context.Background()

	store, err := db.Open(filepath.Join(t.TempDir(), "wisp.db"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	svc := &FeedService{store: store}

	const artwork = "fake-artwork-bytes"
	mux := http.NewServeMux()
	mux.HandleFunc("/feed.xml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		w.Write([]byte(fixtureFeed))
	})
	mux.HandleFunc("/artwork.png", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write([]byte(artwork))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	feedURL := srv.URL + "/feed.xml"
	artworkURL := srv.URL + "/artwork.png"

	feed, err := svc.AddFeedFromSearch(ctx, feedURL, artworkURL)
	if err != nil {
		t.Fatalf("AddFeedFromSearch: %v", err)
	}
	if string(feed.Icon) != artwork {
		t.Errorf("Icon = %q, want the artwork bytes %q", feed.Icon, artwork)
	}
	if feed.IconMime != "image/png" {
		t.Errorf("IconMime = %q, want %q", feed.IconMime, "image/png")
	}
}

func TestFeedServiceAddFeedRejectsBadURL(t *testing.T) {
	ctx := context.Background()
	svc, _ := newTestFeedService(t)

	if _, err := svc.AddFeed(ctx, "not a url"); err == nil {
		t.Fatal("AddFeed(\"not a url\"): expected an error")
	}

	feeds, err := svc.ListFeeds(ctx)
	if err != nil {
		t.Fatalf("ListFeeds: %v", err)
	}
	if len(feeds) != 0 {
		t.Errorf("len(ListFeeds) = %d after a failed AddFeed, want 0", len(feeds))
	}
}
