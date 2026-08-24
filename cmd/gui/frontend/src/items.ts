import { FeedService } from "../bindings/wisp/cmd/gui";
import type { Item } from "../bindings/wisp/internal/api";
import { el } from "./dom";

function formatPubDate(pubDate: string): string {
    if (!pubDate) return "";
    const d = new Date(pubDate);
    return Number.isNaN(d.getTime()) ? pubDate : d.toLocaleDateString();
}

function buildItemRow(item: Item): HTMLLIElement {
    const kindLabel = item.audioUrl ? "Podcast" : "Article";
    const metaParts = [kindLabel, formatPubDate(item.pubDate)].filter(Boolean);
    return el("li", { className: "item-row" }, [
        el("div", { className: "item-row-title", textContent: item.title || item.link }),
        el("div", { className: "item-row-meta", textContent: metaParts.join(" · ") }),
        el("p", { className: "item-row-desc", textContent: item.description }),
    ]);
}

// Rebuilds fully on every expand rather than diffing (unlike feedList's
// row reuse) — this only runs once per click, not on frequent re-renders.
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
