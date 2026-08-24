package podcast

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSearchPodcasts(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("term"); got != "hello" {
			t.Errorf("term query param = %q, want %q", got, "hello")
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"resultCount":2,"results":[
			{"collectionName":"Hello Show","artistName":"Alice","feedUrl":"https://example.com/feed.xml","artworkUrl100":"https://example.com/art.png"},
			{"collectionName":"No Feed Show","artistName":"Bob"}
		]}`))
	}))
	defer srv.Close()

	original := itunesSearchURL
	itunesSearchURL = srv.URL
	defer func() { itunesSearchURL = original }()

	results, err := SearchPodcasts(context.Background(), "hello")
	if err != nil {
		t.Fatalf("SearchPodcasts: %v", err)
	}
	// The result with no feedUrl must be skipped — it's not addable.
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}
	got := results[0]
	if got.Title != "Hello Show" || got.Author != "Alice" ||
		got.FeedURL != "https://example.com/feed.xml" || got.ArtworkURL != "https://example.com/art.png" {
		t.Errorf("results[0] = %+v, unexpected", got)
	}
}
