import { FeedService } from "../bindings/wisp/cmd/gui";
import type { Item } from "../bindings/wisp/internal/api";
import { el } from "./dom";
import { renderMarkdown } from "./markdown";

function formatPubDate(pubDate: string): string {
    if (!pubDate) return "";
    const d = new Date(pubDate);
    return Number.isNaN(d.getTime()) ? pubDate : d.toLocaleDateString();
}

function buildItemRow(item: Item): HTMLLIElement {
    const kindLabel = item.audioUrl ? "Podcast" : "Article";
    const metaParts = [kindLabel, formatPubDate(item.pubDate)].filter(Boolean);

    const summary = el("summary", { className: "item-row-summary" }, [
        el("div", { className: "item-row-title", textContent: item.title || item.link }),
        el("div", { className: "item-row-meta", textContent: metaParts.join(" · ") }),
        el("p", { className: "item-row-desc", textContent: item.description }),
    ]);

    const previewEl = el("div", { className: "item-preview" });
    const details = el("details", { className: "item-row" }, [summary, previewEl]);

    // Rapid toggle can start a second load before the first resolves;
    // isCurrent() drops a stale response so it can't clobber a fresher one.
    let requestId = 0;
    details.addEventListener("toggle", () => {
        if (!details.open) return;
        const id = ++requestId;
        loadPreview(previewEl, item.id, () => id === requestId);
    });

    return el("li", {}, [details]);
}

function setPreviewStatus(previewEl: HTMLDivElement, text: string): void {
    previewEl.replaceChildren(el("p", { className: "item-row-status", textContent: text }));
}

async function loadPreview(previewEl: HTMLDivElement, itemId: number, isCurrent: () => boolean): Promise<void> {
    setPreviewStatus(previewEl, "Loading…");
    let md: string;
    try {
        md = await FeedService.ItemMarkdown(itemId);
    } catch (err) {
        if (isCurrent()) setPreviewStatus(previewEl, `Failed to load article: ${err}`);
        return;
    }
    if (!isCurrent()) return;
    if (!md.trim()) {
        setPreviewStatus(previewEl, "No content available.");
        return;
    }
    previewEl.innerHTML = renderMarkdown(md);
}

export async function loadItems(itemsEl: HTMLUListElement, feedId: number): Promise<void> {
    itemsEl.replaceChildren(el("li", { className: "item-row-status", textContent: "Loading…" }));
    let items: Item[];
    try {
        items = await FeedService.ListItems(feedId);
    } catch (err) {
        itemsEl.replaceChildren(
            el("li", { className: "item-row-status", textContent: `Failed to load items: ${err}` }),
        );
        return;
    }
    if (!items || items.length === 0) {
        itemsEl.replaceChildren(el("li", { className: "item-row-status", textContent: "No items yet." }));
        return;
    }
    itemsEl.replaceChildren(...items.map(buildItemRow));
}
