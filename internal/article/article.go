// Package article resolves an article's readable content and renders it to Markdown.
package article

import (
	"bytes"
	"net/http"
	"net/url"
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

func extractViaReadability(link string) (string, error) {
	article, err := readability.FromURL(link, httpx.DefaultTimeout, func(r *http.Request) {
		r.Header.Set("User-Agent", httpx.UserAgent)
	})
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := article.RenderHTML(&buf); err != nil {
		return "", err
	}
	return sanitizeAndConvert(buf.String(), link)
}

// sanitizeAndConvert resolves relative URLs in htmlInput against baseURL.
func sanitizeAndConvert(htmlInput, baseURL string) (string, error) {
	clean := sanitizePolicy.Sanitize(htmlInput)

	var opts []converter.ConvertOptionFunc
	if domain := domainOf(baseURL); domain != "" {
		opts = append(opts, converter.WithDomain(domain))
	}
	return markdownConverter.ConvertString(clean, opts...)
}

func domainOf(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ""
	}
	return u.Scheme + "://" + u.Host
}
