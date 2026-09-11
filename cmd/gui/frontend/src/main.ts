import "./style.scss";
import "./scrollbar";
import "./linkDialog";
import "./sidebar";
import "./addFeed";
import "./discoverPanel";
import "./settings";
import "./visualizer";
import { loadFeeds } from "./feedList";

const BASE = 20;
let scale = Number(localStorage.getItem("wisp-font-scale")) || 1;
let saveTimer = 0;

// The font-size write is left alone: the browser already batches the relayout
// (measured: 2 layouts for a 30-event zoom burst). Only the save is held back,
// because setItem is synchronous and WebKit backs localStorage with SQLite.
function apply() {
    document.documentElement.style.fontSize = `${BASE * scale}px`;
    window.clearTimeout(saveTimer);
    saveTimer = window.setTimeout(() => localStorage.setItem("wisp-font-scale", String(scale)), 250);
}

function bump(delta: number) {
    scale = Math.min(2, Math.max(0.7, Math.round((scale + delta) * 20) / 20));
    apply();
}

apply();

// Must stay attached: trackpad pinch arrives as a ctrlKey wheel event with no Control
// keydown, so gating this on modifier keys breaks pinch zoom.
window.addEventListener("wheel", (e) => {
    if (!e.ctrlKey && !e.metaKey) return;
    e.preventDefault();
    bump(e.deltaY < 0 ? 0.05 : -0.05);
}, { passive: false });

window.addEventListener("keydown", (e) => {
    if (!e.ctrlKey && !e.metaKey) return;
    if (e.key === "=" || e.key === "+") { e.preventDefault(); bump(0.1); }
    else if (e.key === "-") { e.preventDefault(); bump(-0.1); }
    else if (e.key === "0") { e.preventDefault(); scale = 1; apply(); }
});

loadFeeds();
