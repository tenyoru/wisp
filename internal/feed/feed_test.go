package feed

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"wisp/internal/api"
	"wisp/internal/httpx"
)

const sampleFeed = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
<channel>
	<title>Sample Feed</title>
	<item>
		<title>Episode One</title>
		<link>https://example.com/ep1</link>
		<description>An episode</description>
		<enclosure url="https://example.com/ep1.mp3" type="audio/mpeg" length="123"/>
	</item>
	<item>
		<title>Article One</title>
		<link>https://example.com/article1</link>
		<description>An article</description>
	</item>
</channel>
</rss>`

func TestFetchAndParse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		w.Write([]byte(sampleFeed))
	}))
	defer srv.Close()

	got, err := FetchAndParse(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("FetchAndParse: %v", err)
	}

	if got.Title != "Sample Feed" {
		t.Errorf("Title = %q, want %q", got.Title, "Sample Feed")
	}
	if got.Kind != api.FeedKindPodcast {
		t.Errorf("Kind = %v, want FeedKindPodcast (item 1 has an audio enclosure)", got.Kind)
	}
	if len(got.Items) != 2 {
		t.Fatalf("len(Items) = %d, want 2", len(got.Items))
	}

	ep := got.Items[0]
	if ep.AudioURL != "https://example.com/ep1.mp3" {
		t.Errorf("Items[0].AudioURL = %q, want the mp3 enclosure URL", ep.AudioURL)
	}
	if ep.Title != "Episode One" {
		t.Errorf("Items[0].Title = %q, want %q", ep.Title, "Episode One")
	}

	article := got.Items[1]
	if article.AudioURL != "" {
		t.Errorf("Items[1].AudioURL = %q, want empty (no enclosure)", article.AudioURL)
	}
	if article.Description != "An article" {
		t.Errorf("Items[1].Description = %q, want %q", article.Description, "An article")
	}
}

func TestFetchAndParse_RespectsCallerTimeout(t *testing.T) {
	unblock := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-unblock // never responds within the test's timeout
	}))
	// Defers run LIFO: unblock the handler before Close() waits on it,
	// otherwise Close() blocks forever on the still-in-flight request.
	defer srv.Close()
	defer close(unblock)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := FetchAndParse(ctx, srv.URL)
	if err == nil {
		t.Fatal("FetchAndParse: expected an error from a server that never responds")
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("FetchAndParse took %v, want it to respect the caller's short timeout, not the internal default", elapsed)
	}
}

func TestFetchAndParse_RejectsOversizedFeed(t *testing.T) {
	huge := "<?xml version=\"1.0\"?><rss version=\"2.0\"><channel><title>" +
		strings.Repeat("x", httpx.MaxBytes+1) + "</title></channel></rss>"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(huge))
	}))
	defer srv.Close()

	_, err := FetchAndParse(context.Background(), srv.URL)
	if err == nil {
		t.Fatal("FetchAndParse: expected an error for a feed exceeding maxFeedBytes")
	}
}
