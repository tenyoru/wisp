import type { Feed } from "../bindings/wisp/internal/api";
import { el } from "./dom";

const AVATAR_COLORS = ["#e07a5f", "#81b29a", "#f2cc8f", "#3d5a80", "#9b5de5", "#00b4d8", "#ef476f"];

function avatarColorFor(name: string): string {
    let hash = 0;
    for (let i = 0; i < name.length; i++) hash = (hash * 31 + name.charCodeAt(i)) | 0;
    return AVATAR_COLORS[Math.abs(hash) % AVATAR_COLORS.length];
}

export function renderFallbackAvatar(feed: Feed): HTMLDivElement {
    const label = (feed.title || feed.url || "?").trim().charAt(0).toUpperCase() || "?";
    const div = el("div", {
        className: "feed-row-icon feed-row-icon-fallback",
        textContent: label,
    });
    div.style.background = avatarColorFor(feed.title || feed.url);
    return div;
}

// onReplaced: lets the caller keep its live icon-node reference in sync
// when a corrupt icon swaps itself for the fallback after load fails.
export function renderFeedIcon(feed: Feed, onReplaced?: (next: HTMLElement) => void): HTMLElement {
    if (!feed.icon) return renderFallbackAvatar(feed);

    const img = el("img", {
        className: "feed-row-icon",
        src: `data:${feed.iconMime || "image/x-icon"};base64,${feed.icon}`,
        alt: "",
        draggable: false,
    });
    img.addEventListener(
        "error",
        () => {
            const fallback = renderFallbackAvatar(feed);
            img.replaceWith(fallback);
            onReplaced?.(fallback);
        },
        { once: true },
    );
    return img;
}
