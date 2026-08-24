package article

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestResolveArticleMarkdown_ContentEncoded(t *testing.T) {
	md, err := ResolveArticleMarkdown(
		"https://example.com/article",
		"<p>full body</p><script>evil()</script>",
		"teaser",
	)
	if err != nil {
		t.Fatalf("ResolveArticleMarkdown: %v", err)
	}
	if !strings.Contains(md, "full body") {
		t.Errorf("output %q missing expected body text", md)
	}
	if strings.Contains(md, "evil") || strings.Contains(md, "script") {
		t.Errorf("output %q still contains the script tag", md)
	}
}

const articlePage = `<!DOCTYPE html>
<html>
<head><title>A Real Article</title></head>
<body>
<nav><a href="/">Home</a> <a href="/about">About</a></nav>
<article>
<h1>A Real Article</h1>
<p>This is the opening paragraph of a fairly substantial article about
something worth reading. It needs to be long enough that Readability's
content-density heuristics actually pick it as the main content block
instead of the surrounding navigation chrome.</p>
<p>Here is a second paragraph continuing the same thought, adding more
detail and more sentences so that the paragraph-to-boilerplate ratio in
this document clearly favors the article body over the nav and footer
elements that surround it on the page.</p>
<p>And a third paragraph, just to be safe, since Mozilla's Readability
algorithm scores nodes by text length and link density, and short test
fixtures sometimes fall under its detection threshold entirely.</p>
</article>
<footer>Copyright 2026</footer>
</body>
</html>`

func TestResolveArticleMarkdown_ReadabilityExtraction(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(articlePage))
	}))
	defer srv.Close()

	md, err := ResolveArticleMarkdown(srv.URL, "", "fallback teaser")
	if err != nil {
		t.Fatalf("ResolveArticleMarkdown: %v", err)
	}
	if !strings.Contains(md, "opening paragraph") {
		t.Errorf("output %q missing expected extracted body text", md)
	}
	if strings.Contains(md, "About") {
		t.Errorf("output %q still contains nav chrome", md)
	}
}

func TestResolveArticleMarkdown_Table(t *testing.T) {
	md, err := ResolveArticleMarkdown(
		"https://example.com/article",
		"<table><tr><td>Name</td><td>Age</td></tr><tr><td>Alice</td><td>30</td></tr></table>",
		"teaser",
	)
	if err != nil {
		t.Fatalf("ResolveArticleMarkdown: %v", err)
	}
	if !strings.Contains(md, "Alice") || !strings.Contains(md, "30") {
		t.Errorf("output %q missing table cell content", md)
	}
	if !strings.Contains(md, "---") {
		t.Errorf("output %q missing a Markdown table header separator", md)
	}
}

func TestResolveArticleMarkdown_Strikethrough(t *testing.T) {
	md, err := ResolveArticleMarkdown(
		"https://example.com/article",
		"<p>Was <del>$10</del> now $5.</p>",
		"teaser",
	)
	if err != nil {
		t.Fatalf("ResolveArticleMarkdown: %v", err)
	}
	if !strings.Contains(md, "~~$10~~") {
		t.Errorf("output %q missing strikethrough markup around the old price", md)
	}
}

func TestResolveArticleMarkdown_ResolvesRelativeImageURL(t *testing.T) {
	md, err := ResolveArticleMarkdown(
		"https://example.com/posts/hello",
		`<p>look <img src="/assets/photo.png" alt="a photo"></p>`,
		"teaser",
	)
	if err != nil {
		t.Fatalf("ResolveArticleMarkdown: %v", err)
	}
	if !strings.Contains(md, "https://example.com/assets/photo.png") {
		t.Errorf("output %q still has an unresolved relative image URL", md)
	}
}

func TestResolveArticleMarkdown_FallbackToTeaser(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	md, err := ResolveArticleMarkdown(srv.URL, "", "<b>teaser</b><script>x</script>")
	if err != nil {
		t.Fatalf("ResolveArticleMarkdown: %v", err)
	}
	if !strings.Contains(md, "teaser") {
		t.Errorf("output %q missing the fallback teaser text", md)
	}
	if strings.Contains(md, "script") {
		t.Errorf("output %q still contains the script tag", md)
	}
}
