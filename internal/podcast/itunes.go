// Package podcast handles podcast-specific lookup — currently iTunes
// search — as distinct from feed-level fetch/parse/discovery.
package podcast

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"wisp/internal/api"
	"wisp/internal/httpx"
)

// itunesSearchURL is a var so tests can point it at a local httptest.Server.
var itunesSearchURL = "https://itunes.apple.com/search"

const itunesResultLimit = 25

type itunesSearchResponse struct {
	Results []struct {
		CollectionName string `json:"collectionName"`
		ArtistName     string `json:"artistName"`
		FeedURL        string `json:"feedUrl"`
		ArtworkURL100  string `json:"artworkUrl100"`
	} `json:"results"`
}

// SearchPodcasts queries the iTunes Search API for podcasts matching term.
func SearchPodcasts(ctx context.Context, term string) ([]api.PodcastResult, error) {
	ctx, cancel := context.WithTimeout(ctx, httpx.DefaultTimeout)
	defer cancel()

	q := url.Values{
		"media":  {"podcast"},
		"entity": {"podcast"},
		"term":   {term},
		"limit":  {fmt.Sprint(itunesResultLimit)},
	}
	body, _, err := httpx.FetchCapped(ctx, itunesSearchURL+"?"+q.Encode(), httpx.MaxBytes)
	if err != nil {
		return nil, fmt.Errorf("itunes search %q: %w", term, err)
	}

	var parsed itunesSearchResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("itunes search %q: decode response: %w", term, err)
	}

	results := make([]api.PodcastResult, 0, len(parsed.Results))
	for _, r := range parsed.Results {
		if r.FeedURL == "" {
			continue
		}
		results = append(results, api.PodcastResult{
			Title:      r.CollectionName,
			Author:     r.ArtistName,
			FeedURL:    r.FeedURL,
			ArtworkURL: r.ArtworkURL100,
		})
	}
	return results, nil
}
