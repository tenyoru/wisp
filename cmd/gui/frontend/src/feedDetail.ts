import { Events } from "@wailsio/runtime";
import { FeedService } from "../bindings/wisp/cmd/gui";
import { FeedKind, type Feed, type Item } from "../bindings/wisp/internal/api";
import { el, requireEl } from "./dom";
import { renderFeedIcon } from "./avatar";
import { loadItems, formatPubDate } from "./items";
import { renderMarkdown } from "./markdown";
import { deleteFeed, refreshFeed, loadFeeds } from "./feedList";
import { createViewGroup } from "./views";

const feedViews = createViewGroup([
    requireEl<HTMLDivElement>("feed-list-panel"),
    requireEl<HTMLDivElement>("feed-detail-panel"),
    requireEl<HTMLDivElement>("post-detail-panel"),
]);
const backBtn = requireEl<HTMLButtonElement>("feed-detail-back");
const iconSlot = requireEl<HTMLDivElement>("feed-detail-icon");
const titleEl = requireEl<HTMLHeadingElement>("feed-detail-title");
const metaEl = requireEl<HTMLParagraphElement>("feed-detail-meta");
const refreshBtn = requireEl<HTMLButtonElement>("feed-detail-refresh");
const deleteBtn = requireEl<HTMLButtonElement>("feed-detail-delete");
const editForm = requireEl<HTMLFormElement>("feed-edit-form");
const editTitleInput = requireEl<HTMLInputElement>("feed-edit-title");
const editUrlInput = requireEl<HTMLInputElement>("feed-edit-url");
const editStatusEl = requireEl<HTMLParagraphElement>("feed-edit-status");
const itemsEl = requireEl<HTMLUListElement>("feed-detail-items");

const postBackBtn = requireEl<HTMLButtonElement>("post-detail-back");
const postTitleEl = requireEl<HTMLHeadingElement>("post-detail-title");
const postMetaEl = requireEl<HTMLParagraphElement>("post-detail-meta");
const postBodyEl = requireEl<HTMLDivElement>("post-detail-body");

let currentFeedId: number | null = null;

function setEditStatus(message: string, isError: boolean): void {
    if (!message) {
        editStatusEl.hidden = true;
        return;
    }
    editStatusEl.textContent = message;
    editStatusEl.classList.toggle("is-error", isError);
    editStatusEl.hidden = false;
}

function renderFeed(feed: Feed): void {
    iconSlot.replaceChildren(renderFeedIcon(feed));
    titleEl.textContent = feed.title || feed.url;
    const kindLabel = feed.kind === FeedKind.FeedKindPodcast ? "Podcast" : "Article";
    metaEl.textContent = `${kindLabel} · ${feed.url}`;
    editTitleInput.value = feed.title;
    editUrlInput.value = feed.url;
}

export async function openFeedDetail(feedId: number): Promise<void> {
    currentFeedId = feedId;
    feedViews.show("detail");
    setEditStatus("", false);

    let feed: Feed;
    try {
        feed = await FeedService.GetFeed(feedId);
    } catch (err) {
        setEditStatus(`Failed to load feed: ${err}`, true);
        return;
    }
    renderFeed(feed);
    await loadItems(itemsEl, feedId);
}

function closeFeedDetail(): void {
    currentFeedId = null;
    feedViews.show("list");
}

backBtn.addEventListener("click", closeFeedDetail);

refreshBtn.addEventListener("click", () => {
    if (currentFeedId !== null) refreshFeed(currentFeedId);
});

deleteBtn.addEventListener("click", async () => {
    if (currentFeedId === null) return;
    await deleteFeed(currentFeedId);
    closeFeedDetail();
});

editForm.addEventListener("submit", async (e) => {
    e.preventDefault();
    if (currentFeedId === null) return;
    try {
        const updated = await FeedService.UpdateFeed(currentFeedId, editTitleInput.value.trim(), editUrlInput.value.trim());
        renderFeed(updated);
        setEditStatus("Saved.", false);
        await loadFeeds();
    } catch (err) {
        setEditStatus(`Failed to save: ${err}`, true);
    }
});

// Keeps the open detail page (header + item list) in sync with a refresh
// that was kicked off from here or from the feed list row.
Events.On("feed-refreshed", async (evt) => {
    const result = evt.data;
    if (currentFeedId === null || result.feedId !== currentFeedId || result.error) return;
    if (result.feed) renderFeed(result.feed);
    await loadItems(itemsEl, currentFeedId);
});

// Rapid open/close can start a second load before the first resolves;
// requestId drops a stale response so it can't clobber a fresher one.
let postRequestId = 0;

export async function openPostDetail(item: Item): Promise<void> {
    feedViews.show("post");
    const requestId = ++postRequestId;

    postTitleEl.textContent = item.title || item.link;
    const kindLabel = item.audioUrl ? "Podcast" : "Article";
    postMetaEl.textContent = [kindLabel, formatPubDate(item.pubDate)].filter(Boolean).join(" · ");

    if (item.audioUrl) {
        postBodyEl.replaceChildren(
            el("p", { className: "item-row-status", textContent: "Podcast playback isn't implemented yet." }),
        );
        return;
    }

    postBodyEl.replaceChildren(el("p", { className: "item-row-status", textContent: "Loading…" }));
    let md: string;
    try {
        md = await FeedService.ItemMarkdown(item.id);
    } catch (err) {
        if (requestId === postRequestId) {
            postBodyEl.replaceChildren(el("p", { className: "item-row-status", textContent: `Failed to load article: ${err}` }));
        }
        return;
    }
    if (requestId !== postRequestId) return;
    if (!md.trim()) {
        postBodyEl.replaceChildren(el("p", { className: "item-row-status", textContent: "No content available." }));
        return;
    }
    postBodyEl.innerHTML = renderMarkdown(md);
}

postBackBtn.addEventListener("click", () => feedViews.show("detail"));
