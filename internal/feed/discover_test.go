package feed

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDiscoverFeedURLs_FindsAlternateLink(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<!DOCTYPE html><html><head>
			<link rel="stylesheet" href="/style.css">
			<link rel="alternate" type="application/rss+xml" href="/feed.xml">
		</head><body></body></html>`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	got, err := DiscoverFeedURLs(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("DiscoverFeedURLs: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len(DiscoverFeedURLs) = %d, want 1", len(got))
	}
	if want := srv.URL + "/feed.xml"; got[0].URL != want {
		t.Errorf("got[0].URL = %q, want %q", got[0].URL, want)
	}
}

func TestDiscoverFeedURLs_NoFeedLink(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<!DOCTYPE html><html><head><title>No feed here</title></head><body></body></html>`))
	}))
	defer srv.Close()

	got, err := DiscoverFeedURLs(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("DiscoverFeedURLs: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("len(DiscoverFeedURLs) = %d, want 0", len(got))
	}
}
