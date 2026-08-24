package feed

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"golang.org/x/net/html"

	"wisp/internal/httpx"
)

// An icon is small; this just bounds a misbehaving server, generous enough
// for any real icon (even a large apple-touch-icon PNG).
const maxIconBytes = 2 << 20 // 2MB

// FetchIcon tries the page's own <link rel="icon"> first, falling back to
// the conventional /favicon.ico path.
func FetchIcon(ctx context.Context, siteURL string) (data []byte, mimeType string, err error) {
	if href, ok := discoverIconHref(ctx, siteURL); ok {
		if data, mimeType, err := httpx.FetchCapped(ctx, href, maxIconBytes); err == nil && len(data) > 0 {
			return data, normalizeMime(mimeType, href), nil
		}
	}

	fallback, err := faviconFallbackURL(siteURL)
	if err != nil {
		return nil, "", fmt.Errorf("icon fallback url for %s: %w", siteURL, err)
	}
	data, mimeType, err = httpx.FetchCapped(ctx, fallback, maxIconBytes)
	if err != nil {
		return nil, "", fmt.Errorf("fetch icon for %s: %w", siteURL, err)
	}
	return data, normalizeMime(mimeType, fallback), nil
}

// FetchDirectIcon downloads imageURL directly as an icon, with no
// discovery step — for when the caller already knows a good icon URL
// (e.g. iTunes artwork from a podcast search result).
func FetchDirectIcon(ctx context.Context, imageURL string) (data []byte, mimeType string, err error) {
	data, mimeType, err = httpx.FetchCapped(ctx, imageURL, maxIconBytes)
	if err != nil {
		return nil, "", fmt.Errorf("fetch icon %s: %w", imageURL, err)
	}
	return data, normalizeMime(mimeType, imageURL), nil
}

// faviconFallbackURL builds the conventional /favicon.ico path — named for
// that literal, standardized filename, not for our own Icon naming.
func faviconFallbackURL(siteURL string) (string, error) {
	u, err := url.Parse(siteURL)
	if err != nil {
		return "", err
	}
	fallback := &url.URL{Scheme: u.Scheme, Host: u.Host, Path: "/favicon.ico"}
	return fallback.String(), nil
}

// discoverIconHref matches rel by substring ("icon", "shortcut icon",
// "apple-touch-icon", ...), not the exact token.
func discoverIconHref(ctx context.Context, siteURL string) (string, bool) {
	matches, err := findAllLinkHrefs(ctx, siteURL, func(attrs map[string]string) bool {
		return strings.Contains(strings.ToLower(attrs["rel"]), "icon")
	})
	if err != nil || len(matches) == 0 {
		return "", false
	}
	return matches[0].Href, true
}

type linkMatch struct {
	Href  string
	Title string // the link's title="..." attribute, if any
}

// findAllLinkHrefs is a best-effort heuristic scan, not a strict
// HTML-spec parse.
func findAllLinkHrefs(ctx context.Context, siteURL string, match func(attrs map[string]string) bool) ([]linkMatch, error) {
	base, err := url.Parse(siteURL)
	if err != nil {
		return nil, err
	}
	body, _, err := httpx.FetchCapped(ctx, siteURL, httpx.MaxBytes)
	if err != nil {
		return nil, err
	}

	var matches []linkMatch
	tokenizer := html.NewTokenizer(strings.NewReader(string(body)))
	for {
		switch tokenizer.Next() {
		case html.ErrorToken:
			return matches, nil
		case html.StartTagToken, html.SelfClosingTagToken:
			tok := tokenizer.Token()
			if tok.Data != "link" {
				continue
			}
			attrs := make(map[string]string, len(tok.Attr))
			for _, a := range tok.Attr {
				attrs[a.Key] = a.Val
			}
			href := attrs["href"]
			if href == "" || !match(attrs) {
				continue
			}
			resolved, err := base.Parse(href)
			if err != nil {
				continue
			}
			matches = append(matches, linkMatch{Href: resolved.String(), Title: attrs["title"]})
		}
	}
}

func normalizeMime(headerMime, resourceURL string) string {
	if headerMime != "" {
		if i := strings.IndexByte(headerMime, ';'); i >= 0 {
			headerMime = headerMime[:i]
		}
		return strings.TrimSpace(headerMime)
	}
	switch {
	case strings.HasSuffix(resourceURL, ".png"):
		return "image/png"
	case strings.HasSuffix(resourceURL, ".svg"):
		return "image/svg+xml"
	default:
		return "image/x-icon"
	}
}
