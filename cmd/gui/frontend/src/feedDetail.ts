import { Events, Browser, Clipboard } from "@wailsio/runtime";
import { FeedService } from "../bindings/wisp/cmd/gui";
import { FeedKind, type Feed, type Item } from "../bindings/wisp/internal/api";
import { el, requireEl, REFRESH_CHANGED } from "./dom";
import { renderFeedIcon } from "./avatar";
import { loadItems, formatPubDate } from "./items";
import { renderMarkdown, type TocHeading } from "./markdown";
import { deleteFeed, refreshFeed, refreshAllFeeds, loadFeeds, isRefreshing } from "./feedList";
import { createViewGroup } from "./views";
import { setStatus } from "./status";
import * as player from "./player";

const postPanel = requireEl<HTMLDivElement>("post-detail-panel");
const feedViews = createViewGroup([
    requireEl<HTMLDivElement>("feed-list-panel"),
    requireEl<HTMLDivElement>("feed-detail-panel"),
    postPanel,
]);
const iconSlot = requireEl<HTMLDivElement>("feed-detail-icon");
const titleEl = requireEl<HTMLHeadingElement>("feed-detail-title");
const metaEl = requireEl<HTMLParagraphElement>("feed-detail-meta");
const deleteBtn = requireEl<HTMLButtonElement>("feed-detail-delete");
const editForm = requireEl<HTMLFormElement>("feed-edit-form");
const editTitleInput = requireEl<HTMLInputElement>("feed-edit-title");
const editUrlInput = requireEl<HTMLInputElement>("feed-edit-url");
const editStatusEl = requireEl<HTMLParagraphElement>("feed-edit-status");
const itemsEl = requireEl<HTMLUListElement>("feed-detail-items");

const postNavEl = requireEl<HTMLDivElement>("post-nav");
const postBackBtn = requireEl<HTMLButtonElement>("post-nav-back");
const postTocBtn = requireEl<HTMLButtonElement>("post-nav-toc");
const navSpin = requireEl<HTMLElement>("nav-refresh-spin");
const postCopyBtn = requireEl<HTMLButtonElement>("post-nav-copy");
const postEditBtn = requireEl<HTMLButtonElement>("post-nav-edit");
const postShareEl = requireEl<HTMLDivElement>("post-share-menu");
const postTocBackdrop = requireEl<HTMLDivElement>("post-toc-backdrop");
const postTitleEl = requireEl<HTMLHeadingElement>("post-detail-title");
const postMetaEl = requireEl<HTMLParagraphElement>("post-detail-meta");
const postBodyEl = requireEl<HTMLDivElement>("post-detail-body");
const postTocEl = requireEl<HTMLElement>("post-detail-toc");

let currentFeedId: number | null = null;
let currentFeed: Feed | null = null;

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
    currentFeed = feed;
    iconSlot.replaceChildren(renderFeedIcon(feed));
    titleEl.textContent = feed.title || feed.url;
    const kindLabel = feed.kind === FeedKind.FeedKindPodcast ? "Podcast" : "Article";
    metaEl.textContent = `${kindLabel} · ${feed.url}`;
    editTitleInput.value = feed.title;
    editUrlInput.value = feed.url;
}

function showFeedNav(feed: Feed): void {
    postNavEl.hidden = false;
    postCopyBtn.hidden = false;
    postTocBtn.textContent = displayUrl(feed.url);
    postTocBtn.setAttribute("aria-disabled", "true");
    postEditBtn.hidden = false;
    postBackBtn.setAttribute("aria-label", "Back");
    closeOverlays();
    postTocEl.replaceChildren();
    syncNavSpin();
}

function syncNavSpin(): void {
    navSpin.hidden = currentFeedId === null || !isRefreshing(currentFeedId);
}

export async function openFeedDetail(feedId: number): Promise<void> {
    currentFeedId = feedId;
    closeOverlays();
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
    showFeedNav(feed);
    await loadItems(itemsEl, feedId);
}

function closeFeedDetail(): void {
    currentFeedId = null;
    currentFeed = null;
    closeOverlays();
    postNavEl.hidden = true;
    navSpin.hidden = true;
    feedViews.show("list");
}

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
    if (!postPanel.hidden && currentPostItem) await openPostDetail(currentPostItem);
});

let postRequestId = 0;
let currentPostItem: Item | null = null;
let currentDownloadStatusEl: HTMLElement | null = null;
function closeToc(): void {
    postTocBtn.setAttribute("aria-expanded", "false");
    postTocEl.hidden = true;
}

function closeShare(): void {
    postCopyBtn.setAttribute("aria-expanded", "false");
    postShareEl.hidden = true;
}

function closeOverlays(): void {
    closeToc();
    closeShare();
    postTocBackdrop.hidden = true;
}

function itemUrl(item: Item | null): string {
    return item?.link || item?.audioUrl || "";
}

function displayUrl(link: string): string {
    try {
        const u = new URL(link);
        return (u.host + u.pathname).replace(/\/$/, "") || u.host;
    } catch {
        return link;
    }
}

function setToc(headings: TocHeading[]): void {
    closeOverlays();
    const hasToc = headings.length > 1;
    if (hasToc) postTocBtn.removeAttribute("aria-disabled");
    else postTocBtn.setAttribute("aria-disabled", "true");
    if (!hasToc) {
        postTocEl.replaceChildren();
        return;
    }
    postTocEl.replaceChildren(
        ...headings.map((h) => {
            const link = el("a", { href: `#${h.id}`, className: `post-toc-l${h.level}`, textContent: h.text });
            link.addEventListener("click", (e) => {
                e.preventDefault();
                closeOverlays();
                document.getElementById(h.id)?.scrollIntoView({ behavior: "smooth", block: "start" });
            });
            return link;
        }),
    );
}

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
            player.updateSrc(item.id, audioSrc(item));
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
    const playBtn = el("button", { type: "button", className: "podcast-play-btn", textContent: "▶ Play episode" });
    playBtn.addEventListener("click", () => player.play(item, audioSrc(item)));
    currentDownloadStatusEl = el("div", { className: "podcast-download" });
    renderDownloadStatus(item);
    return el("div", { className: "podcast-player" }, [playBtn, currentDownloadStatusEl]);
}

function isParseableTranscript(item: Item): boolean {
    const type = item.transcriptType ?? "";
    return type.includes("vtt") || type.includes("srt") || type.includes("subrip");
}

async function loadShowNotes(item: Item, requestId: number): Promise<void> {
    const notesEl = el("div", { className: "podcast-shownotes" });
    notesEl.classList.toggle("is-transcript", isParseableTranscript(item));
    notesEl.addEventListener("click", (e) => {
        const link = (e.target as HTMLElement).closest<HTMLAnchorElement>("a[data-t]");
        if (!link) return;
        const selection = window.getSelection();
        if (selection && !selection.isCollapsed) return;
        player.play(item, audioSrc(item), Number(link.dataset.t));
    });
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
    setToc(toc);

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
        player.updateSrc(currentPostItem.id, audioSrc(currentPostItem));
        renderDownloadStatus(currentPostItem);
        return;
    }
    const pct = result.total > 0 ? `${Math.round((result.downloaded / result.total) * 100)}%` : formatBytes(result.downloaded);
    currentDownloadStatusEl.replaceChildren(el("span", { textContent: `Downloading… ${pct}` }));
});

export async function openPostDetail(item: Item): Promise<void> {
    feedViews.show("post");
    const requestId = ++postRequestId;

    currentPostItem = item;
    currentDownloadStatusEl = null;
    postNavEl.hidden = false;
    postCopyBtn.hidden = false;
    postTocBtn.textContent = item.link ? displayUrl(item.link) : (item.title || "");
    postTocBtn.setAttribute("aria-disabled", "true");
    postEditBtn.hidden = true;
    syncNavSpin();
    const backTo = titleEl.textContent?.trim();
    postBackBtn.setAttribute("aria-label", backTo ? `Back to ${backTo}` : "Back");
    closeOverlays();
    postTocEl.replaceChildren();

    postTitleEl.textContent = item.title || item.link;
    const kindLabel = item.audioUrl ? "Podcast" : "Article";
    postMetaEl.textContent = [kindLabel, formatPubDate(item.pubDate)].filter(Boolean).join(" · ");

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
    setToc(toc);
}

function closePostDetail(): void {
    closeOverlays();
    feedViews.show("detail");
    if (currentFeed) showFeedNav(currentFeed);
}

postBackBtn.addEventListener("click", () => {
    if (postPanel.hidden) closeFeedDetail();
    else closePostDetail();
});

postTocBtn.addEventListener("click", () => {
    if (!postTocEl.children.length) return;
    if (postTocEl.hidden) {
        closeShare();
        postTocBtn.setAttribute("aria-expanded", "true");
        postTocBackdrop.hidden = false;
        postTocEl.hidden = false;
    } else {
        closeOverlays();
    }
});

function navShare(): { title: string; url: string } | null {
    if (!postPanel.hidden) {
        const url = itemUrl(currentPostItem);
        if (currentPostItem && url) return { title: currentPostItem.title || url, url };
        return null;
    }
    if (currentFeed) return { title: currentFeed.title || currentFeed.url, url: currentFeed.url };
    return null;
}

function refreshCurrent(): void {
    if (document.getElementById("view-feeds")?.hidden) return;
    if (currentFeedId !== null) void refreshFeed(currentFeedId);
    else void refreshAllFeeds();
}

function openEditFeed(): void {
    if (!postPanel.hidden) closePostDetail();
    const panel = editForm.closest("details");
    if (panel) panel.open = true;
    editTitleInput.focus();
}

postCopyBtn.addEventListener("click", () => {
    if (!currentFeed && !currentPostItem) return;
    if (postShareEl.hidden) {
        closeToc();
        postCopyBtn.setAttribute("aria-expanded", "true");
        postTocBackdrop.hidden = false;
        postShareEl.hidden = false;
    } else {
        closeOverlays();
    }
});

postShareEl.addEventListener("click", async (e) => {
    const action = (e.target as HTMLElement).closest<HTMLElement>("[data-share]")?.dataset.share;
    if (!action) return;
    if (action === "refresh") {
        closeOverlays();
        refreshCurrent();
        return;
    }
    if (action === "edit") {
        closeOverlays();
        openEditFeed();
        return;
    }
    const target = navShare();
    if (!target) return;
    closeOverlays();
    if (action === "copy") {
        try {
            await Clipboard.SetText(target.url);
        } catch {
            setStatus("Couldn't copy link.", true);
        }
        return;
    }
    if (action === "open") {
        try {
            const parsed = new URL(target.url);
            if (parsed.protocol !== "http:" && parsed.protocol !== "https:") return;
            await Browser.OpenURL(parsed);
        } catch (err) {
            setStatus(`Couldn't open link: ${err}`, true);
        }
        return;
    }
    if (action === "share") {
        try {
            if (navigator.share) await navigator.share({ title: target.title, url: target.url });
            else await Clipboard.SetText(target.url);
        } catch (err) {
            if (err instanceof Error && err.name === "AbortError") return;
            setStatus(`Couldn't share: ${err}`, true);
        }
    }
});

postTocBackdrop.addEventListener("click", closeOverlays);

document.addEventListener("keydown", (e) => {
    if (e.key === "Escape") closeOverlays();
    if (e.key !== "r" && e.key !== "R") return;
    const inField = e.target instanceof HTMLElement && !!e.target.closest("input, textarea, select, [contenteditable]");
    if (e.ctrlKey || e.metaKey) {
        e.preventDefault();
        refreshCurrent();
        return;
    }
    if (inField || e.altKey) return;
    e.preventDefault();
    refreshCurrent();
});

document.addEventListener(REFRESH_CHANGED, syncNavSpin);

player.setNavigationHandlers({
    openItem: (item) => openPostDetail(item),
    openFeed: (feedId) => openFeedDetail(feedId),
});
