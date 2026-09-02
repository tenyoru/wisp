// Package article resolves an article's readable content and renders it to Markdown.
package article

import (
	"bytes"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	readability "codeberg.org/readeck/go-readability/v2"
	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/base"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/commonmark"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/strikethrough"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/table"
	"github.com/microcosm-cc/bluemonday"

	"wisp/internal/httpx"
)

var sanitizePolicy = bluemonday.UGCPolicy()

// markdownConverter adds table/strikethrough, unlike the package's ConvertString.
var markdownConverter = converter.NewConverter(
	converter.WithPlugins(
		base.NewBasePlugin(),
		commonmark.NewCommonmarkPlugin(),
		table.NewTablePlugin(
			table.WithHeaderPromotion(true), // scraped tables rarely have <th>
			table.WithNewlineBehavior(table.NewlineBehaviorPreserve),
		),
		strikethrough.NewStrikethroughPlugin(),
	),
)

// ResolveArticleMarkdown resolves the best available Markdown for an article.
func ResolveArticleMarkdown(link, contentEncoded, description string) (string, error) {
	if strings.TrimSpace(contentEncoded) != "" {
		return sanitizeAndConvert(contentEncoded, link)
	}
	if md, err := extractViaReadability(link); err == nil && strings.TrimSpace(md) != "" {
		return md, nil
	}
	return sanitizeAndConvert(description, link)
}

// ResolveShowNotes is ResolveArticleMarkdown without the Readability
// fallback: a podcast episode's link is often a generic show page rather
// than that specific episode, so scraping it returns the wrong content.
func ResolveShowNotes(link, contentEncoded, description string) (string, error) {
	if strings.TrimSpace(contentEncoded) != "" {
		return sanitizeAndConvert(contentEncoded, link)
	}
	return sanitizeAndConvert(description, link)
}

func extractViaReadability(link string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, link, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", httpx.UserAgent)

	client := &http.Client{Timeout: httpx.DefaultTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("fetch %s: unexpected status %s", link, resp.Status)
	}

	parser := readability.NewParser()
	article, err := parser.Parse(resp.Body, resp.Request.URL)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := article.RenderHTML(&buf); err != nil {
		return "", err
	}
	return sanitizeAndConvert(buf.String(), resp.Request.URL.String())
}

// Some feeds double-escape <br> tags — unescape just this pattern, not the
// whole input, so genuine escaped-HTML examples in article prose survive.
var doubleEscapedBR = regexp.MustCompile(`&lt;br\s*/?&gt;`)

// sanitizeAndConvert resolves relative URLs in htmlInput against baseURL.
func sanitizeAndConvert(htmlInput, baseURL string) (string, error) {
	htmlInput = doubleEscapedBR.ReplaceAllString(htmlInput, "<br>")
	clean := sanitizePolicy.Sanitize(htmlInput)

	domain := domainOf(baseURL)
	var opts []converter.ConvertOptionFunc
	if domain != "" {
		opts = append(opts, converter.WithDomain(domain))
	}

	md, err := markdownConverter.ConvertString(clean, opts...)
	if err != nil {
		return md, err
	}
	if domain != "" {
		// html-to-markdown resolves a same-page "#fragment" href against
		// domain (scheme://host, no path — WithDomain never sees the
		// article's own path), turning in-page anchors into links to the
		// site's root instead of leaving them alone.
		md = strings.ReplaceAll(md, "]("+domain+"#", "](#")
	}
	// A newline between a source <a> and the punctuation right after it
	// collapses to a space per HTML's whitespace rules; html-to-markdown
	// carries that space through, stranding the punctuation alone when the
	// line wraps there.
	md = trailingPunctAfterLink.ReplaceAllString(md, ")$1")
	return md, nil
}

var trailingPunctAfterLink = regexp.MustCompile(`\)[ \t]+([.,;:!?)\]])`)

func domainOf(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ""
	}
	return u.Scheme + "://" + u.Host
}
