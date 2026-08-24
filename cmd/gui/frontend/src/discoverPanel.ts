import { FeedService } from "../bindings/wisp/cmd/gui";
import type { PodcastResult } from "../bindings/wisp/internal/api";
import { el, requireEl } from "./dom";
import { setStatus } from "./status";
import { loadFeeds } from "./feedList";

const discoverForm = requireEl<HTMLFormElement>("discover-form");
const discoverQueryInput = requireEl<HTMLInputElement>("discover-query");
const discoverSubmitBtn = discoverForm.querySelector<HTMLButtonElement>("button");
if (!discoverSubmitBtn) throw new Error("missing submit button in #discover-form");
const discoverResultsEl = requireEl<HTMLUListElement>("discover-results");

function buildDiscoverResultRow(result: PodcastResult): HTMLLIElement {
    const addBtn = el("button", { type: "button", className: "discover-result-add", textContent: "Add" });
    addBtn.addEventListener("click", async () => {
        addBtn.disabled = true;
        try {
            const feed = await FeedService.AddFeedFromSearch(result.feedUrl, result.artworkUrl);
            addBtn.textContent = "Added";
            setStatus(`Added "${feed.title || feed.url}".`, false);
            await loadFeeds();
        } catch (err) {
            addBtn.disabled = false;
            setStatus(`Couldn't add feed: ${err}`, true);
        }
    });

    const artwork = result.artworkUrl
        ? el("img", { className: "discover-result-artwork", src: result.artworkUrl, alt: "", draggable: false })
        : el("div", { className: "discover-result-artwork discover-result-artwork-fallback" });

    return el("li", { className: "discover-result" }, [
        artwork,
        el("div", { className: "discover-result-main" }, [
            el("div", { className: "item-row-title", textContent: result.title }),
            el("div", { className: "item-row-meta", textContent: result.author }),
        ]),
        addBtn,
    ]);
}

discoverForm.addEventListener("submit", async (e) => {
    e.preventDefault();
    const term = discoverQueryInput.value.trim();
    if (!term) return;

    discoverSubmitBtn.disabled = true;
    discoverResultsEl.replaceChildren(el("li", { className: "item-row-status", textContent: "Searching…" }));
    try {
        const results = await FeedService.SearchPodcasts(term);
        if (!results || results.length === 0) {
            discoverResultsEl.replaceChildren(
                el("li", { className: "item-row-status", textContent: "No results." }),
            );
            return;
        }
        discoverResultsEl.replaceChildren(...results.map(buildDiscoverResultRow));
    } catch (err) {
        discoverResultsEl.replaceChildren(
            el("li", { className: "item-row-status", textContent: `Search failed: ${err}` }),
        );
    } finally {
        discoverSubmitBtn.disabled = false;
    }
});
