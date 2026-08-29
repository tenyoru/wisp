import { FeedService } from "../bindings/wisp/cmd/gui";
import type { Item } from "../bindings/wisp/internal/api";
import { el } from "./dom";
import { openPostDetail } from "./feedDetail";

export function formatPubDate(pubDate: string): string {
    if (!pubDate) return "";
    const d = new Date(pubDate);
    return Number.isNaN(d.getTime()) ? pubDate : d.toLocaleDateString();
}

// DOMParser output is never attached to the page, so this can't execute embedded scripts.
function stripHtml(html: string): string {
    return new DOMParser().parseFromString(html, "text/html").body.textContent ?? "";
}

function buildItemRow(item: Item): HTMLLIElement {
    const kindLabel = item.audioUrl ? "Podcast" : "Article";
    const metaParts = [kindLabel, formatPubDate(item.pubDate)].filter(Boolean);

    const summary = el(
        "div",
        { className: "item-row-summary", tabIndex: 0, role: "button" },
        [
            el("div", { className: "item-row-title", textContent: item.title || item.link }),
            el("div", { className: "item-row-meta", textContent: metaParts.join(" · ") }),
            el("p", { className: "item-row-desc", textContent: stripHtml(item.description) }),
        ],
    );
    summary.addEventListener("click", () => openPostDetail(item));
    summary.addEventListener("keydown", (e) => {
        if (e.key === "Enter" || e.key === " ") {
            e.preventDefault();
            openPostDetail(item);
        }
    });

    return el("li", { className: "item-row" }, [summary]);
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
