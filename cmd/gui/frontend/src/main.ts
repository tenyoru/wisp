import "./style.scss";
import "./scrollbar";
import "./sidebar";
import "./addFeed";
import "./discoverPanel";
import "./settings";
import { loadFeeds } from "./feedList";

const BASE = 20;
let scale = Number(localStorage.getItem("wisp-font-scale")) || 1;

function apply() {
    document.documentElement.style.fontSize = `${BASE * scale}px`;
    localStorage.setItem("wisp-font-scale", String(scale));
}

function bump(delta: number) {
    scale = Math.min(2, Math.max(0.7, Math.round((scale + delta) * 20) / 20));
    apply();
}

apply();
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
