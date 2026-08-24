import { FeedService } from "../bindings/wisp/cmd/gui";
import type { DiscoveredFeed } from "../bindings/wisp/internal/api";
import { el, requireEl } from "./dom";
import { setStatus } from "./status";
import { loadFeeds } from "./feedList";

const form = requireEl<HTMLFormElement>("add-feed-form");
const urlInput = requireEl<HTMLInputElement>("feed-url");
const submitBtn = form.querySelector<HTMLButtonElement>("button");
if (!submitBtn) throw new Error("missing submit button in #add-feed-form");
const feedPickerEl = requireEl<HTMLUListElement>("feed-picker");

form.addEventListener("submit", async (e) => {
    e.preventDefault();
    const url = urlInput.value.trim();
    if (!url) return;

    submitBtn.disabled = true;
    feedPickerEl.hidden = true;
    feedPickerEl.replaceChildren();
    setStatus("Adding…", false);

    try {
        const feed = await FeedService.AddFeed(url);
        urlInput.value = "";
        setStatus(`Added "${feed.title || feed.url}".`, false);
        await loadFeeds();
        return;
    } catch {
        // Fall through to feed-link discovery below.
    } finally {
        submitBtn.disabled = false;
    }

    let found: DiscoveredFeed[];
    try {
        found = await FeedService.DiscoverFeeds(url);
    } catch (err) {
        setStatus(`Couldn't add feed: ${err}`, true);
        return;
    }
    if (!found || found.length === 0) {
        setStatus("Couldn't find a feed at that URL.", true);
        return;
    }

    if (found.length === 1) {
        setStatus("Adding…", false);
        try {
            const feed = await FeedService.AddFeed(found[0].url);
            urlInput.value = "";
            setStatus(`Added "${feed.title || feed.url}".`, false);
            await loadFeeds();
        } catch (err) {
            setStatus(`Couldn't add feed: ${err}`, true);
        }
        return;
    }

    setStatus(`This page has ${found.length} feeds — pick one:`, false);
    feedPickerEl.replaceChildren(
        ...found.map((candidate) => {
            const addBtn = el("button", { type: "button", className: "discover-result-add", textContent: "Add" });
            addBtn.addEventListener("click", async () => {
                addBtn.disabled = true;
                try {
                    const feed = await FeedService.AddFeed(candidate.url);
                    addBtn.textContent = "Added";
                    setStatus(`Added "${feed.title || feed.url}".`, false);
                    await loadFeeds();
                } catch (err) {
                    addBtn.disabled = false;
                    setStatus(`Couldn't add feed: ${err}`, true);
                }
            });
            return el("li", { className: "discover-result" }, [
                el("div", { className: "discover-result-main" }, [
                    el("div", { className: "item-row-title", textContent: candidate.title }),
                    el("div", { className: "item-row-meta", textContent: candidate.url }),
                ]),
                addBtn,
            ]);
        }),
    );
    feedPickerEl.hidden = false;
});
