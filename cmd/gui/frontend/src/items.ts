import { FeedService } from "../bindings/wisp/cmd/gui";
import type { Item } from "../bindings/wisp/internal/api";
import { el, requireEl } from "./dom";
import { openPostDetail } from "./feedDetail";

const PAGE_SIZE = 50;

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

// Guards a previous loadItems call's in-flight fetch/observer from acting
// after a newer call (new feed, or a refresh) has taken over the list.
let activeObserver: IntersectionObserver | null = null;
let loadToken = 0;

export async function loadItems(itemsEl: HTMLUListElement, feedId: number): Promise<void> {
    const token = ++loadToken;
    activeObserver?.disconnect();

    itemsEl.replaceChildren(el("li", { className: "item-row-status", textContent: "Loading…" }));

    let offset = 0;
    let isLoading = false;
    const sentinel = el("li", { className: "item-row-status", textContent: "Loading…" });

    const observer = new IntersectionObserver(
        (entries) => {
            if (entries[0].isIntersecting) void loadPage();
        },
        { root: requireEl<HTMLElement>("main-scroll") },
    );
    activeObserver = observer;

    async function loadPage(): Promise<void> {
        if (isLoading || token !== loadToken) return;
        isLoading = true;
        let page: Item[];
        try {
            page = (await FeedService.ListItems(feedId, PAGE_SIZE, offset)) ?? [];
        } catch (err) {
            isLoading = false;
            if (token !== loadToken) return;
            observer.disconnect();
            if (offset === 0) {
                itemsEl.replaceChildren(el("li", { className: "item-row-status", textContent: `Failed to load items: ${err}` }));
            } else {
                sentinel.remove();
            }
            return;
        }
        isLoading = false;
        if (token !== loadToken) return;

        if (offset === 0 && page.length === 0) {
            observer.disconnect();
            itemsEl.replaceChildren(el("li", { className: "item-row-status", textContent: "No items yet." }));
            return;
        }

        if (offset === 0) itemsEl.replaceChildren();
        sentinel.remove();
        itemsEl.append(...page.map(buildItemRow));
        offset += page.length;

        if (page.length < PAGE_SIZE) {
            observer.disconnect();
        } else {
            itemsEl.append(sentinel);
            observer.observe(sentinel);
        }
    }

    await loadPage();
}
