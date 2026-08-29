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
- [ ] `Item.Saved`/`Item.Liked` (bool) + `Item.LikedAt` — liked sorts by like recency, not pub date, so "most recently liked" surfaces first

## internal/feed
Feed-level only: fetch, parse, discover, icon. Article/podcast-specific
logic lives in their own packages (below) — split out so "rendering an
article" and "finding a podcast" don't grow inside the feed package.
- [x] `FetchAndParse` — RSS/Atom via `gofeed`, podcast/article kind inferred once, in one place
- [x] `FetchIcon`/`FetchDirectIcon` — site favicon discovery + direct URL fetch (iTunes artwork)
- [x] `DiscoverFeedURLs` — scans a page's `<link rel="alternate">` tags, returns every feed it advertises
- [ ] OPML import/export
- [ ] `audioEnclosure` only matches `type` prefixed `audio` — a video-only podcast enclosure (`video/mp4` etc.) currently has no `AudioURL`, so the whole item is miscategorized as an article with no player, download, or transcript

## internal/article
- [x] `ResolveArticleMarkdown` — content:encoded → Readability extraction → sanitized teaser fallback, converted to Markdown (`go-readability` + `html-to-markdown` + `bluemonday`)
- [x] Markdown polish: GFM tables, strikethrough, relative image/link URL resolution

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
- [ ] Item status column
- [ ] Local Markdown cache on disk, not a DB column — opened articles land under an XDG cache dir, capped at the last 100 opened (LRU eviction); saved/liked items are cached indefinitely, cleared only when the user explicitly clears the cache
- [ ] `saved`/`liked`/`liked_at` columns on items + `SetItemSaved`/`SetItemLiked` + a liked-items query ordered by `liked_at DESC` (separate from `ListItems`' `pub_date DESC`)

## internal/media
- [ ] Everything — empty stub. Need to pick Go libraries for:
  - episode download
  - ffmpeg convert
  - whisper transcription (local whisper.cpp exec, or cloud API)
  - audio playback

## MCP server
- [ ] Everything — not started. Expose feeds/items to AI agents (list feeds, search/list items, fetch `ItemMarkdown`, mark saved/liked) as MCP tools, reusing `internal/db`/`FeedService` logic directly rather than duplicating it
- [ ] Pick a Go MCP SDK, and a transport (stdio for local Claude Desktop/Code config vs. HTTP/SSE)
- [ ] Decide where it lives: new `cmd/mcp`, or a mode/flag on an existing binary
- [ ] Don't hand an agent `ItemMarkdown`'s raw output for a podcast transcript as-is — it wraps every cue in its own `[text](#t=seconds)` seek-link (`internal/podcast/subtitles.go`, for the GUI's click-to-seek), which is unclickable noise to an agent and burns context for nothing. Either add a plain-text transcript resolver without cue-links, or strip markdown links before returning from the MCP tool.

## Sync server
- [ ] Everything — not started. Today wisp is fully local: `cmd/cli` and `cmd/gui` each open the same SQLite file directly, no daemon, no IPC (see `internal/db`'s package doc)
- [ ] Purpose: multi-device sync of feed subscriptions + read/saved/liked state
- [ ] Open questions: self-hosted vs. none/local-only fallback, sync protocol (simple push/pull REST vs. CRDT-style conflict-free merge), auth model
- [ ] Storage backend for the server side — same SQLite schema, or something else once it's multi-tenant

## Wiring
- [x] `cmd/gui`: feed list (add/refresh/delete, icon, All/Articles/Podcasts filter), item listing per feed, iTunes search panel, site feed-link discovery+picker
- [x] `cmd/gui`: call `feed.FetchAndParse` → `db.UpsertFeed`/`UpsertItems` on subscribe/refresh
- [x] `cmd/gui`: article reading view — click an item to expand its rendered article (`FeedService.ItemMarkdown` → `marked` + `highlight.js`, no caching yet, resolves fresh on every expand)
- [ ] `cmd/gui`: `glamour` added to go.mod but unused — no CLI/TUI to render into yet
- [x] `cmd/gui`: podcast playback — in-app `<audio>` player, on-demand episode download, show notes/transcript below the player (prefers `podcast:transcript` when a feed publishes one, falls back to show notes)
- [ ] `cmd/cli`: actual TUI/subcommands — currently a one-line placeholder `main()`
- [ ] `cmd/gui`: save/like buttons on item rows + a Saved and a Liked view (Liked ordered by most-recently-liked, not pub date)
- [ ] `cmd/gui` (desktop): local Markdown preview API — serve a cached article's `.md` file as rendered HTML over HTTP, so an external editor/tool can point at it and preview the file directly (likely piggybacks on the existing `-tags server` HTTP mode rather than a new server)
- [ ] `cmd/gui`: Table of Contents for the article preview — walk the rendered Markdown's headings and build a jump-to-heading nav; applies to both the in-app preview and the preview-API HTML output above

## Design

No real design pass yet on any target — the desktop UI is whatever CSS each
feature needed as it was built, TUI doesn't exist, mobile has never been
opened. Scoping only below, nothing implemented.

### Desktop (`cmd/gui`, current webview UI)
- [ ] Typography scale — font-sizes are ad-hoc per component right now (12px/12.5px/13.5px/14px/14.5px/15px/18px scattered across `styles/*.scss`), no defined scale
- [ ] Spacing scale — margins/paddings/gaps are hand-picked per component, same problem
- [ ] Keyboard nav / focus-visible states — only `:hover` is styled anywhere; no focus ring, no full keyboard-only flow
- [ ] Keybindings for common actions (refresh, add feed, navigate items, play/pause podcast, etc.) — none exist yet, everything is mouse/tap-only
- [ ] Consistent loading/empty/error visual language — every feature invented its own inline text pattern as it was built (`.item-row-status`, `.status`, `.empty-state`, ad-hoc `textContent` in a few places)
- [ ] Dedicated article reading view — currently an inline expand under the item row; no reading-width constraint, font-size control, or "close/back" affordance
- [ ] Settings/preferences UI — doesn't exist; blocks on `api.Settings` (not yet defined either)
- [ ] Real app icon/branding — `cmd/gui/build/appicon.png` etc. are still Wails' default template assets
- [ ] Light theme / theme toggle — `color-scheme: dark` is hardcoded, no light palette exists

### TUI (`cmd/cli`)
- [ ] Doesn't exist yet — `main()` is a one-line placeholder. First design decision: navigation model (vim-style modal nav like lazygit/k9s, vs. simple list+enter)
- [ ] Keybinding scheme
- [ ] Screen inventory: feed list, item list, article reader (via `glamour`, already a dependency but unused), add-feed/search input, status/error line
- [ ] Bubble Tea component picks per screen (list, viewport, spinner, textinput)
- [ ] Narrow-terminal layout behavior (min width before things break)

### Mobile (phone — Wails already scaffolds `cmd/gui/build/{android,ios}`, same webview UI, not a separate app)
- [ ] Touch targets — current buttons (e.g. 18px refresh/delete icons with 4-6px padding) are well under the ~44px minimum tap-target guidance
- [ ] Every interactive affordance right now is hover-only (refresh/delete buttons, feed-filter, item-row-summary) — none of it exists on a device with no hover
- [ ] Layout was never tested at a phone viewport — fixed `max-width: 480px` single-column container, large fixed top padding (`4rem`), no safe-area-inset handling for notches
- [ ] No mobile-appropriate navigation pattern — it's a single continuously-scrolling page; typical mobile RSS readers use a tab bar or drill-down nav instead
- [ ] Common mobile RSS-reader gestures (swipe-to-refresh, swipe-to-delete) — not present, current delete/refresh are tap-only buttons

## Verification habits established this session
- `go build ./cmd/cli ./cmd/gui ./internal/...` + `go vet` (same set) — don't blanket `go build ./...`, `cmd/gui/build/{ios,android}` are Wails' cross-compile targets, not plain-buildable
- New deps only land in `go.mod` once real code imports them (`go mod tidy` strips unused ones — unlike Cargo)
- `git add -A` needed before any `nix build` (local flakes only see staged content)
