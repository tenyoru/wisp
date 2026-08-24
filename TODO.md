# wisp (Go + Wails v3) — TODO

## Scaffolding
- [x] `go.mod` + `internal/{api,article,db,feed,httpx,media,podcast}` + `cmd/{cli,gui}` layout
- [x] `flake.nix` devShell (go, gtk4, webkitgtk_6_0, wails3 build deps, ffmpeg, whisper-cpp, alsa-lib)
- [x] `cmd/gui` — real Wails v3 app, builds and runs (still the default template UI)

## internal/api
- [x] `FeedKind`, `Feed`, `Item`, `ParsedFeed`
- [x] `PodcastResult` (iTunes search), `DiscoveredFeed` (site feed-link discovery)
- [ ] `Settings` (theme, playback speed, whisper model size, cloud-transcription toggle)
- [ ] `ItemStatus` (new/downloading/transcribing/ready) — needed once `internal/media` exists
- [ ] `Segment` (transcript segment) — needed once transcription exists

## internal/feed
Feed-level only: fetch, parse, discover, icon. Article/podcast-specific
logic lives in their own packages (below) — split out so "rendering an
article" and "finding a podcast" don't grow inside the feed package.
- [x] `FetchAndParse` — RSS/Atom via `gofeed`, podcast/article kind inferred once, in one place
- [x] `FetchIcon`/`FetchDirectIcon` — site favicon discovery + direct URL fetch (iTunes artwork)
- [x] `DiscoverFeedURLs` — scans a page's `<link rel="alternate">` tags, returns every feed it advertises
- [ ] OPML import/export

## internal/article
- [x] `ResolveArticleMarkdown` — content:encoded → Readability extraction → sanitized teaser fallback, converted to Markdown (`go-readability` + `html-to-markdown` + `bluemonday`)
- [ ] Polish pass on Markdown generation itself before wiring up rendering — current focus (see below)

## internal/podcast
- [x] `SearchPodcasts` — iTunes Search API
- [ ] Additional search providers beyond iTunes (e.g. PodcastIndex)

## internal/httpx
- [x] `FetchCapped` — shared capped/timed-out HTTP fetch used by feed, article, and podcast

## internal/db
- [x] `Store` interface + `SQLiteStore` (`modernc.org/sqlite`, no cgo)
- [x] Feeds/items schema, upsert-by-url / upsert-by-(feed_id,link), cascade delete
- [x] `DefaultPath` — XDG data dir (`wisp-go/wisp.db`, deliberately separate from the old Rust build's incompatible schema)
- [ ] Settings table
- [ ] Item status + cached article HTML/Markdown column (for offline reopen)

## internal/media
- [ ] Everything — empty stub. Need to pick Go libraries for:
  - episode download
  - ffmpeg convert
  - whisper transcription (local whisper.cpp exec, or cloud API)
  - audio playback

## Wiring
- [x] `cmd/gui`: feed list (add/refresh/delete, icon, All/Articles/Podcasts filter), item listing per feed, iTunes search panel, site feed-link discovery+picker
- [x] `cmd/gui`: call `feed.FetchAndParse` → `db.UpsertFeed`/`UpsertItems` on subscribe/refresh
- [ ] `cmd/gui`: article reading view — items currently only show a teaser; nothing calls `article.ResolveArticleMarkdown` or renders/caches full content yet
- [ ] `cmd/gui`: Markdown → HTML rendering on the frontend for the article view above (deliberately after the Markdown-generation polish pass, not before)
- [ ] `cmd/gui`: podcast playback — no player, `internal/media` is still an empty stub
- [ ] `cmd/cli`: actual TUI/subcommands — currently a one-line placeholder `main()`

## Verification habits established this session
- `go build ./cmd/cli ./cmd/gui ./internal/...` + `go vet` (same set) — don't blanket `go build ./...`, `cmd/gui/build/{ios,android}` are Wails' cross-compile targets, not plain-buildable
- New deps only land in `go.mod` once real code imports them (`go mod tidy` strips unused ones — unlike Cargo)
- `git add -A` needed before any `nix build` (local flakes only see staged content)
