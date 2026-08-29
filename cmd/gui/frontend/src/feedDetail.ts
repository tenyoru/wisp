import { Events } from "@wailsio/runtime";
import { FeedService } from "../bindings/wisp/cmd/gui";
import { FeedKind, type Feed, type Item } from "../bindings/wisp/internal/api";
import { el, requireEl } from "./dom";
import { renderFeedIcon } from "./avatar";
import { loadItems, formatPubDate } from "./items";
import { renderMarkdown } from "./markdown";
import { deleteFeed, refreshFeed, loadFeeds } from "./feedList";
import { createViewGroup } from "./views";
import { setStatus } from "./status";

const containerEl = requireEl<HTMLDivElement>("app-container");

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
const postTocEl = requireEl<HTMLElement>("post-detail-toc");

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

Events.On("feed-refreshed", async (evt) => {
    const result = evt.data;
    if (currentFeedId === null || result.feedId !== currentFeedId || result.error) return;
    if (result.feed) renderFeed(result.feed);
    await loadItems(itemsEl, currentFeedId);
});

let postRequestId = 0;
let currentPostItemId: number | null = null;
let currentPostItem: Item | null = null;
let currentDownloadStatusEl: HTMLElement | null = null;
let currentAudioEl: HTMLAudioElement | null = null;

function audioSrc(item: Item): string {
    if (!item.downloadFilename) return item.audioUrl;
    const encodedPath = item.downloadFilename.split("/").map(encodeURIComponent).join("/");
    return `/episodes/${encodedPath}`;
}

function formatBytes(n: number): string {
    return n >= 1024 * 1024 ? `${(n / (1024 * 1024)).toFixed(1)} MB` : `${Math.round(n / 1024)} KB`;
}

function renderDownloadStatus(item: Item): void {
    if (!currentDownloadStatusEl) return;
    const statusEl = currentDownloadStatusEl;

    if (item.downloadFilename) {
        const deleteBtn = el("button", { type: "button", className: "link-btn", textContent: "Delete download" });
        deleteBtn.addEventListener("click", async () => {
            try {
                await FeedService.DeleteDownload(item.id);
            } catch (err) {
                setStatus(`Couldn't delete download: ${err}`, true);
                return;
            }
            item.downloadFilename = "";
            if (currentAudioEl) currentAudioEl.src = audioSrc(item);
            renderDownloadStatus(item);
        });
        statusEl.replaceChildren(el("span", { textContent: "Downloaded" }), deleteBtn);
        return;
    }

    const downloadBtn = el("button", { type: "button", className: "link-btn", textContent: "Download" });
    downloadBtn.addEventListener("click", async () => {
        statusEl.replaceChildren(el("span", { textContent: "Downloading…" }));
        try {
            await FeedService.DownloadEpisode(item.id);
        } catch (err) {
            setStatus(`Couldn't start download: ${err}`, true);
            renderDownloadStatus(item);
        }
    });
    statusEl.replaceChildren(downloadBtn);
}

function renderPodcastPlayer(item: Item): HTMLElement {
    currentPostItem = item;
    const audioEl = el("audio", { className: "podcast-audio", controls: true, src: audioSrc(item) });
    currentAudioEl = audioEl;
    currentDownloadStatusEl = el("div", { className: "podcast-download" });
    renderDownloadStatus(item);
    return el("div", { className: "podcast-player" }, [audioEl, currentDownloadStatusEl]);
}

// Transcript paragraphs are prefixed with a [MM:SS](#t=seconds) link
// (see internal/podcast/subtitles.go) — clicking one seeks the player
// instead of navigating.
function seekOnTimeLinkClick(e: MouseEvent): void {
    const link = (e.target as HTMLElement).closest<HTMLAnchorElement>('a[href^="#t="]');
    if (!link || !currentAudioEl) return;
    e.preventDefault();
    currentAudioEl.currentTime = Number(link.hash.slice("#t=".length));
}

function isParseableTranscript(item: Item): boolean {
    const type = item.transcriptType;
    return type.includes("vtt") || type.includes("srt") || type.includes("subrip");
}

// Silent on failure/empty — many podcasts have nothing beyond the description already shown.
async function loadShowNotes(item: Item, requestId: number): Promise<void> {
    const notesEl = el("div", { className: "podcast-shownotes" });
    notesEl.classList.toggle("is-transcript", isParseableTranscript(item));
    notesEl.addEventListener("click", seekOnTimeLinkClick);
    postBodyEl.append(el("h2", { className: "podcast-shownotes-label", textContent: "Show notes" }), notesEl);

    let md: string;
    try {
        md = await FeedService.ItemMarkdown(item.id);
    } catch {
        return removeShowNotes();
    }
    if (requestId !== postRequestId) return;
    if (!md.trim()) return removeShowNotes();

    const { html, toc } = renderMarkdown(md);
    notesEl.innerHTML = html;
    if (toc.length > 1) {
        postTocEl.hidden = false;
        containerEl.classList.add("is-wide");
        postTocEl.replaceChildren(
            ...toc.map((h) => {
                const link = el("a", { href: `#${h.id}`, className: `post-toc-l${h.level}`, textContent: h.text });
                link.addEventListener("click", (e) => {
                    e.preventDefault();
                    document.getElementById(h.id)?.scrollIntoView({ behavior: "smooth", block: "start" });
                });
                return link;
            }),
        );
    }

    function removeShowNotes(): void {
        notesEl.previousElementSibling?.remove();
        notesEl.remove();
    }
}

Events.On("episode-download", (evt) => {
    const result = evt.data;
    if (!currentPostItem || result.itemId !== currentPostItem.id || !currentDownloadStatusEl) return;

    if (result.error) {
        setStatus(`Download failed: ${result.error}`, true);
        currentDownloadStatusEl.replaceChildren(el("span", { className: "podcast-download-error", textContent: "Download failed." }));
        return;
    }
    if (result.done) {
        currentPostItem.downloadFilename = result.downloadFilename;
        if (currentAudioEl) currentAudioEl.src = audioSrc(currentPostItem);
        renderDownloadStatus(currentPostItem);
        return;
    }
    const pct = result.total > 0 ? `${Math.round((result.downloaded / result.total) * 100)}%` : formatBytes(result.downloaded);
    currentDownloadStatusEl.replaceChildren(el("span", { textContent: `Downloading… ${pct}` }));
});

export async function openPostDetail(item: Item): Promise<void> {
    feedViews.show("post");
    const requestId = ++postRequestId;

    postTitleEl.textContent = item.title || item.link;
    const kindLabel = item.audioUrl ? "Podcast" : "Article";
    postMetaEl.textContent = [kindLabel, formatPubDate(item.pubDate)].filter(Boolean).join(" · ");
    postTocEl.hidden = true;
    postTocEl.replaceChildren();
    containerEl.classList.remove("is-wide");

    currentPostItem = null;
    currentAudioEl = null;
    currentDownloadStatusEl = null;

    if (item.audioUrl) {
        postBodyEl.replaceChildren(renderPodcastPlayer(item));
        await loadShowNotes(item, requestId);
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
    const { html, toc } = renderMarkdown(md);
    postBodyEl.innerHTML = html;
    if (toc.length > 1) {
        postTocEl.hidden = false;
        containerEl.classList.add("is-wide");
        postTocEl.replaceChildren(
            ...toc.map((h) => {
                const link = el("a", { href: `#${h.id}`, className: `post-toc-l${h.level}`, textContent: h.text });
                link.addEventListener("click", (e) => {
                    e.preventDefault();
                    document.getElementById(h.id)?.scrollIntoView({ behavior: "smooth", block: "start" });
                });
                return link;
            }),
        );
    }
}

postBackBtn.addEventListener("click", () => {
    containerEl.classList.remove("is-wide");
    feedViews.show("detail");
});
