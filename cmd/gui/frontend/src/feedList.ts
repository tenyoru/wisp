import { Events } from "@wailsio/runtime";
import { FeedService } from "../bindings/wisp/cmd/gui";
import { FeedKind, type Feed } from "../bindings/wisp/internal/api";
import { el, requireEl } from "./dom";
import { setStatus } from "./status";
import { renderFeedIcon } from "./avatar";
import { openFeedDetail } from "./feedDetail";

// Ids with a refresh in flight — RefreshFeed only queues it; completion
// arrives later via "feed-refreshed".
const refreshingIds = new Set<number>();

const listEl = requireEl<HTMLUListElement>("feed-list");
const refreshAllBtn = requireEl<HTMLButtonElement>("refresh-all-btn");
const feedFilterEl = requireEl<HTMLDivElement>("feed-filter");
const feedFilterButtons = [...feedFilterEl.querySelectorAll<HTMLButtonElement>(".feed-filter-btn")];

interface RowEntry {
    li: HTMLLIElement;
    iconEl: HTMLElement;
    titleEl: HTMLDivElement;
    metaEl: HTMLDivElement;
    refreshBtn: HTMLButtonElement;
    // Mirrors feed.kind so the filter can check it without re-fetching.
    kind: FeedKind;
    // Change-detection key for iconEl, swapped via replaceWith when it
    // changes. No wrapper span: WebKitGTK doesn't box a `display: contents`
    // child of the row, which silently hid every icon.
    lastIconKey: string;
}

// Persistent per-feed <li>, updated in place — a full teardown/rebuild on
// every refresh made the whole list flash on every click.
const rowElements = new Map<number, RowEntry>();

function iconKeyFor(feed: Feed): string {
    return feed.icon ? `${feed.iconMime}:${feed.icon}` : "";
}

function buildFeedRow(feed: Feed): RowEntry {
    const iconEl = renderFeedIcon(feed, (next) => { entry.iconEl = next; });
    const titleEl = el("div", { className: "feed-row-title" });
    const metaEl = el("div", { className: "feed-row-meta" });

    const refreshBtn = el("button", {
        className: "feed-row-refresh",
        type: "button",
        textContent: "↻",
        title: "Refresh feed",
    });
    // Buttons sit inside the row; without this a click also opens the feed detail page.
    refreshBtn.addEventListener("click", (e) => {
        e.stopPropagation();
        refreshFeed(feed.id);
    });

    const deleteBtn = el("button", {
        className: "feed-row-delete",
        type: "button",
        textContent: "×",
        title: "Remove feed",
    });
    deleteBtn.addEventListener("click", (e) => {
        e.stopPropagation();
        deleteFeed(feed.id);
    });

    const summary = el(
        "div",
        { className: "feed-row-summary", tabIndex: 0, role: "button" },
        [iconEl, el("div", { className: "feed-row-main" }, [titleEl, metaEl]), refreshBtn, deleteBtn],
    );
    summary.addEventListener("click", () => openFeedDetail(feed.id));
    summary.addEventListener("keydown", (e) => {
        if (e.key === "Enter" || e.key === " ") {
            e.preventDefault();
            openFeedDetail(feed.id);
        }
    });

    const li = el("li", { className: "feed-row" }, [summary]);

    const entry: RowEntry = {
        li,
        iconEl,
        titleEl,
        metaEl,
        refreshBtn,
        kind: feed.kind,
        lastIconKey: iconKeyFor(feed),
    };
    return entry;
}

async function updateFeedRow(entry: RowEntry, feed: Feed): Promise<void> {
    const iconKey = iconKeyFor(feed);
    if (iconKey !== entry.lastIconKey) {
        const nextIcon = renderFeedIcon(feed, (next) => { entry.iconEl = next; });
        entry.iconEl.replaceWith(nextIcon);
        entry.iconEl = nextIcon;
        entry.lastIconKey = iconKey;
    }
    entry.titleEl.textContent = feed.title || feed.url;
    entry.kind = feed.kind;

    const kindLabel = feed.kind === FeedKind.FeedKindPodcast ? "Podcast" : "Article";
    let itemCount = 0;
    try {
        itemCount = await FeedService.ItemCount(feed.id);
    } catch {
        // Non-fatal — the row still shows without a count.
    }
    entry.metaEl.textContent = `${kindLabel} · ${itemCount} item${itemCount === 1 ? "" : "s"}`;

    const isRefreshing = refreshingIds.has(feed.id);
    entry.refreshBtn.disabled = isRefreshing;
    entry.refreshBtn.classList.toggle("is-spinning", isRefreshing);
}

// Pure visibility toggle over already-rendered rows — never re-fetches.
// Persisted to localStorage so it survives a restart.
type FeedFilterValue = "all" | "article" | "podcast";
const FEED_FILTER_STORAGE_KEY = "wisp:feedFilter";

function loadStoredFeedFilter(): FeedFilterValue {
    const stored = localStorage.getItem(FEED_FILTER_STORAGE_KEY);
    return stored === "article" || stored === "podcast" ? stored : "all";
}

let currentFeedFilter: FeedFilterValue = loadStoredFeedFilter();

function feedFilterMatches(kind: FeedKind): boolean {
    if (currentFeedFilter === "article") return kind === FeedKind.FeedKindArticle;
    if (currentFeedFilter === "podcast") return kind === FeedKind.FeedKindPodcast;
    return true;
}

const filterEmptyEl = el("li", { className: "empty-state", textContent: "No feeds match this filter." });

function applyFeedFilter(): void {
    for (const btn of feedFilterButtons) {
        btn.classList.toggle("is-active", btn.dataset.filter === currentFeedFilter);
    }

    let visibleCount = 0;
    for (const entry of rowElements.values()) {
        const matches = feedFilterMatches(entry.kind);
        entry.li.hidden = !matches;
        if (matches) visibleCount++;
    }

    if (rowElements.size > 0 && visibleCount === 0) {
        listEl.append(filterEmptyEl);
    } else {
        filterEmptyEl.remove();
    }
}

for (const btn of feedFilterButtons) {
    btn.addEventListener("click", () => {
        const value = btn.dataset.filter;
        if (value !== "all" && value !== "article" && value !== "podcast") return;
        currentFeedFilter = value;
        localStorage.setItem(FEED_FILTER_STORAGE_KEY, value);
        applyFeedFilter();
    });
}
applyFeedFilter();

// Coalesces overlapping calls (e.g. every feed in "refresh all" firing its
// own event close together) so they don't race into duplicate rows.
let loadFeedsInFlight: Promise<void> | null = null;
let loadFeedsQueued = false;

export function loadFeeds(): Promise<void> {
    if (loadFeedsInFlight) {
        loadFeedsQueued = true;
        return loadFeedsInFlight;
    }
    loadFeedsInFlight = (async () => {
        do {
            loadFeedsQueued = false;
            await loadFeedsOnce();
        } while (loadFeedsQueued);
        loadFeedsInFlight = null;
    })();
    return loadFeedsInFlight;
}

async function loadFeedsOnce(): Promise<void> {
    let feeds: Feed[];
    try {
        feeds = await FeedService.ListFeeds();
    } catch (err) {
        setStatus(`Failed to load feeds: ${err}`, true);
        return;
    }

    if (!feeds || feeds.length === 0) {
        rowElements.clear();
        listEl.replaceChildren(el("li", { className: "empty-state", textContent: "No feeds yet." }));
        return;
    }

    // Rows are appended once and never repositioned — reordering to match
    // the backend's sort on every render made rows visibly jump.
    const seenIds = new Set<number>();
    for (const feed of feeds) {
        seenIds.add(feed.id);
        let entry = rowElements.get(feed.id);
        if (!entry) {
            entry = buildFeedRow(feed);
            rowElements.set(feed.id, entry);
            listEl.append(entry.li);
        }
        await updateFeedRow(entry, feed);
    }

    for (const [id, entry] of rowElements) {
        if (!seenIds.has(id)) {
            entry.li.remove();
            rowElements.delete(id);
        }
    }
    const validLis = new Set([...rowElements.values()].map((e) => e.li));
    for (const child of [...listEl.children]) {
        if (!validLis.has(child as HTMLLIElement)) child.remove();
    }

    applyFeedFilter();
}

export async function deleteFeed(id: number): Promise<void> {
    try {
        await FeedService.DeleteFeed(id);
    } catch (err) {
        setStatus(`Failed to remove feed: ${err}`, true);
        return;
    }
    await loadFeeds();
}

export async function refreshFeed(id: number): Promise<void> {
    refreshingIds.add(id);
    await loadFeeds();
    try {
        await FeedService.RefreshFeed(id);
    } catch (err) {
        refreshingIds.delete(id);
        setStatus(`Couldn't refresh feed: ${err}`, true);
        await loadFeeds();
    }
}

refreshAllBtn.addEventListener("click", async () => {
    let feeds: Feed[];
    try {
        feeds = await FeedService.ListFeeds();
    } catch (err) {
        setStatus(`Failed to load feeds: ${err}`, true);
        return;
    }
    if (!feeds || feeds.length === 0) return;

    for (const feed of feeds) refreshingIds.add(feed.id);
    await loadFeeds();

    try {
        await FeedService.RefreshAllFeeds();
    } catch (err) {
        for (const feed of feeds) refreshingIds.delete(feed.id);
        setStatus(`Couldn't refresh feeds: ${err}`, true);
        await loadFeeds();
    }
});

Events.On("feed-refreshed", async (evt) => {
    const result = evt.data;
    refreshingIds.delete(result.feedId);
    if (result.error) {
        setStatus(`Refresh failed: ${result.error}`, true);
    } else if (result.feed) {
        setStatus(`Refreshed "${result.feed.title || result.feed.url}".`, false);
    }
    await loadFeeds();
});
