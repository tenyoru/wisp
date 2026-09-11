import { Browser } from "@wailsio/runtime";
import { requireEl } from "./dom";
import { setStatus } from "./status";

const dialog = requireEl<HTMLDivElement>("link-dialog");
const urlEl = requireEl<HTMLParagraphElement>("link-dialog-url");
const cancelBtn = requireEl<HTMLButtonElement>("link-dialog-cancel");
const openBtn = requireEl<HTMLButtonElement>("link-dialog-open");

let pending: URL | null = null;
let promptOpen = false;

function close(): void {
    pending = null;
    promptOpen = false;
    dialog.hidden = true;
}

function ask(url: URL): void {
    pending = url;
    promptOpen = true;
    urlEl.textContent = (url.host + url.pathname).replace(/\/$/, "") || url.host;
    dialog.hidden = false;
    queueMicrotask(() => openBtn.focus());
}

dialog.addEventListener("click", (e) => {
    if (e.target === dialog) close();
});
cancelBtn.addEventListener("click", close);
openBtn.addEventListener("click", () => {
    const url = pending;
    close();
    if (!url) return;
    void Browser.OpenURL(url).catch((err) => setStatus(`Couldn't open link: ${err}`, true));
});

document.addEventListener("click", (e) => {
    if (e.button !== 0) return;
    const a = (e.target as Element | null)?.closest("a[href]");
    if (!(a instanceof HTMLAnchorElement)) return;
    const raw = a.getAttribute("href") || "";
    if (raw.startsWith("#")) return;
    e.preventDefault();
    let url: URL;
    try { url = new URL(raw); } catch { return; }
    if (url.protocol !== "http:" && url.protocol !== "https:") return;
    ask(url);
});

document.addEventListener("keydown", (e) => {
    if (!promptOpen) return;
    if (e.key === "Enter" || e.key === " ") return;
    e.stopPropagation();
    if (e.key === "Escape") {
        e.preventDefault();
        close();
        return;
    }
    if (e.key === "Tab" || e.key.startsWith("Arrow")) {
        e.preventDefault();
        const toOpen = e.key === "Tab"
            ? document.activeElement !== openBtn
            : e.key === "ArrowRight" || e.key === "ArrowDown";
        (toOpen ? openBtn : cancelBtn).focus();
    }
}, true);
