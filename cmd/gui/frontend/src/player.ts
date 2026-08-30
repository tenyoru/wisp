import { requireEl } from "./dom";
import { renderFeedIcon } from "./avatar";
import { setStatus } from "./status";
import { FeedService } from "../bindings/wisp/cmd/gui";
import type { Item } from "../bindings/wisp/internal/api";

const barEl = requireEl<HTMLDivElement>("now-playing-bar");
const artworkEl = requireEl<HTMLDivElement>("now-playing-artwork");
const titleEl = requireEl<HTMLDivElement>("now-playing-title");
const showEl = requireEl<HTMLDivElement>("now-playing-show");
const playBtn = requireEl<HTMLButtonElement>("now-playing-play");
const backBtn = requireEl<HTMLButtonElement>("now-playing-back");
const forwardBtn = requireEl<HTMLButtonElement>("now-playing-forward");
const closeBtn = requireEl<HTMLButtonElement>("now-playing-close");
const seekEl = requireEl<HTMLInputElement>("now-playing-seek");
const currentTimeEl = requireEl<HTMLSpanElement>("now-playing-current");
const durationEl = requireEl<HTMLSpanElement>("now-playing-duration");

export const audioEl = new Audio();

const PLAY_ICON = '<svg width="14" height="14" viewBox="0 0 20 20" fill="none"><path d="M6 4L16 10L6 16V4Z" fill="currentColor"/></svg>';
const PAUSE_ICON = '<svg width="14" height="14" viewBox="0 0 20 20" fill="none"><rect x="5" y="4" width="4" height="12" rx="1" fill="currentColor"/><rect x="11" y="4" width="4" height="12" rx="1" fill="currentColor"/></svg>';

// Set by feedDetail.ts after both modules are loaded, breaking the import
// cycle (feedDetail imports player to control playback, so player can't
// import feedDetail's openPostDetail/openFeedDetail back).
let onOpenItem: ((item: Item) => void) | null = null;
let onOpenFeed: ((feedId: number) => void) | null = null;

export function setNavigationHandlers(handlers: { openItem: (item: Item) => void; openFeed: (feedId: number) => void }): void {
    onOpenItem = handlers.openItem;
    onOpenFeed = handlers.openFeed;
}

let currentItem: Item | null = null;
let currentFeedId: number | null = null;
let seeking = false;

function formatTime(seconds: number): string {
    if (!Number.isFinite(seconds)) return "0:00";
    const total = Math.floor(seconds);
    return `${Math.floor(total / 60)}:${(total % 60).toString().padStart(2, "0")}`;
}

async function loadArtwork(feedId: number): Promise<void> {
    if (feedId === currentFeedId) return;
    currentFeedId = feedId;
    try {
        const feed = await FeedService.GetFeed(feedId);
        if (feedId !== currentFeedId) return;
        artworkEl.replaceChildren(renderFeedIcon(feed));
        showEl.textContent = feed.title || feed.url;
    } catch {
        // artwork/show name are cosmetic — leave the placeholder on failure
    }
}

// Loads item if it isn't already the one playing (a src change resets
// playback position), then plays — optionally seeking first.
export function play(item: Item, src: string, atSeconds?: number): void {
    const isNewItem = currentItem?.id !== item.id;
    if (isNewItem) {
        audioEl.src = src;
        currentItem = item;
        titleEl.textContent = item.title || item.link;
        void loadArtwork(item.feedId);
        barEl.hidden = false;
    }
    if (atSeconds !== undefined) {
        // A src just set has no metadata yet — WebKit can drop a currentTime
        // assigned before loadedmetadata fires, so defer it in that case.
        if (isNewItem) audioEl.addEventListener("loadedmetadata", () => { audioEl.currentTime = atSeconds; }, { once: true });
        else audioEl.currentTime = atSeconds;
    }
    audioEl.play().catch((err) => setStatus(`Couldn't play episode: ${err}`, true));
}

// A download finishing updates the source out from under a possibly-
// playing episode; preserve position instead of restarting from 0.
export function updateSrc(itemId: number, src: string): void {
    if (currentItem?.id !== itemId) return;
    const wasPlaying = !audioEl.paused;
    const time = audioEl.currentTime;
    audioEl.src = src;
    audioEl.currentTime = time;
    if (wasPlaying) audioEl.play();
}

playBtn.addEventListener("click", () => {
    if (audioEl.paused) audioEl.play().catch((err) => setStatus(`Couldn't play episode: ${err}`, true));
    else audioEl.pause();
});

backBtn.addEventListener("click", () => {
    audioEl.currentTime = Math.max(0, audioEl.currentTime - 15);
});
forwardBtn.addEventListener("click", () => {
    audioEl.currentTime = Math.min(audioEl.duration || Infinity, audioEl.currentTime + 15);
});

closeBtn.addEventListener("click", () => {
    audioEl.pause();
    audioEl.removeAttribute("src");
    currentItem = null;
    currentFeedId = null;
    barEl.hidden = true;
});

titleEl.addEventListener("click", () => {
    if (currentItem) onOpenItem?.(currentItem);
});
artworkEl.addEventListener("click", () => {
    if (currentItem) onOpenFeed?.(currentItem.feedId);
});
showEl.addEventListener("click", () => {
    if (currentItem) onOpenFeed?.(currentItem.feedId);
});

audioEl.addEventListener("play", () => { playBtn.innerHTML = PAUSE_ICON; });
audioEl.addEventListener("pause", () => { playBtn.innerHTML = PLAY_ICON; });

audioEl.addEventListener("timeupdate", () => {
    // audioEl.seeking: mid-seek, timeupdate can fire stale values here.
    if (!seeking && !audioEl.seeking) currentTimeEl.textContent = formatTime(audioEl.currentTime);
    if (!seeking && audioEl.duration) {
        seekEl.value = String((audioEl.currentTime / audioEl.duration) * 1000);
    }
});

audioEl.addEventListener("seeked", () => {
    currentTimeEl.textContent = formatTime(audioEl.currentTime);
});

audioEl.addEventListener("loadedmetadata", () => {
    durationEl.textContent = formatTime(audioEl.duration);
});

seekEl.addEventListener("input", () => {
    seeking = true;
    if (audioEl.duration) currentTimeEl.textContent = formatTime((Number(seekEl.value) / 1000) * audioEl.duration);
});
seekEl.addEventListener("change", () => {
    if (audioEl.duration) audioEl.currentTime = (Number(seekEl.value) / 1000) * audioEl.duration;
    seeking = false;
});
