import { requireEl } from "./dom";
import { audioEl } from "./player";

const canvas = requireEl<HTMLCanvasElement>("now-playing-viz");
const barEl = requireEl<HTMLElement>("now-playing-bar");
const playBtn = requireEl<HTMLElement>("now-playing-play");
const g = canvas.getContext("2d");

const POINTS = 32;
const SMOOTH = 0.25;
const BIN0 = 1;
const BIN1 = 80;
const LINE = 2;

let analyser: AnalyserNode | null = null;
let bins = new Uint8Array(0);
let levels = new Float32Array(POINTS);
let cx = 0;
let cy = 0;
let baseR = 18;
let spanR = 14;
let running = false;
let alpha = 0;
let theme = "";
let rgb = "212, 188, 139";

function resize(): void {
    const w = barEl.clientWidth;
    const h = barEl.clientHeight;
    if (!w || !h) return;

    const dpr = window.devicePixelRatio || 1;
    canvas.width = Math.round(w * dpr);
    canvas.height = Math.round(h * dpr);
    g?.setTransform(dpr, 0, 0, dpr, 0, 0);

    const c = canvas.getBoundingClientRect();
    const b = playBtn.getBoundingClientRect();
    cx = b.left + b.width / 2 - c.left;
    cy = b.top + b.height / 2 - c.top;
    baseR = b.width / 2 + LINE / 2 + 2;
    spanR = 14;
}

function paintRgb(): string {
    const current = document.documentElement.dataset.theme ?? "";
    if (current === theme && rgb) return rgb;
    theme = current;
    const hex = getComputedStyle(document.documentElement).getPropertyValue("--accent").trim() || "#d4bc8b";
    const n = parseInt(hex.slice(1), 16);
    rgb = `${(n >> 16) & 255}, ${(n >> 8) & 255}, ${n & 255}`;
    return rgb;
}

function sample(): void {
    if (!analyser) return;
    analyser.getByteFrequencyData(bins);
    const hi = Math.min(BIN1, bins.length);
    const span = Math.max(1, hi - BIN0);
    for (let i = 0; i < POINTS; i++) {
        const a = BIN0 + Math.floor(i * span / POINTS);
        const b = BIN0 + Math.floor((i + 1) * span / POINTS);
        let peak = 0;
        for (let k = a; k < Math.max(a + 1, b) && k < bins.length; k++) {
            if (bins[k] > peak) peak = bins[k];
        }
        levels[i] += (peak / 255 - levels[i]) * SMOOTH;
    }
}

function draw(): void {
    if (!g) return;
    const dpr = window.devicePixelRatio || 1;
    const w = canvas.width / dpr;
    const h = canvas.height / dpr;
    g.clearRect(0, 0, w, h);

    g.beginPath();
    for (let i = 0; i < POINTS; i++) {
        const a = (i / POINTS) * Math.PI * 2 - Math.PI / 2;
        const dx = Math.cos(a);
        const dy = Math.sin(a);
        const tx = dx > 0 ? (w - cx) / dx : dx < 0 ? cx / -dx : Infinity;
        const ty = dy > 0 ? (h - cy) / dy : dy < 0 ? cy / -dy : Infinity;
        const r = Math.min(baseR + levels[i] * spanR, Math.min(tx, ty) - LINE);
        const x = cx + dx * r;
        const y = cy + dy * r;
        if (i === 0) g.moveTo(x, y);
        else g.lineTo(x, y);
    }
    g.closePath();
    g.strokeStyle = `rgba(${paintRgb()}, ${0.9 * alpha})`;
    g.lineWidth = LINE;
    g.lineJoin = "round";
    g.stroke();
}

function frame(): void {
    if (!running) return;
    if (audioEl.paused) {
        alpha *= 0.94;
        for (let i = 0; i < POINTS; i++) levels[i] *= 0.97;
    } else {
        alpha += (1 - alpha) * 0.25;
        sample();
    }

    draw();
    if (audioEl.paused && alpha < 0.02) {
        running = false;
        alpha = 0;
        levels.fill(0);
        g?.clearRect(0, 0, canvas.width, canvas.height);
        return;
    }
    requestAnimationFrame(frame);
}

const LOCAL = "http://127.0.0.1:9246";
const tapEl = new Audio();
tapEl.muted = true;
tapEl.crossOrigin = "anonymous";
tapEl.preload = "auto";

function localSrc(): string | null {
    const src = audioEl.currentSrc || audioEl.src;
    return src.startsWith(LOCAL) ? src : null;
}

function syncTap(): void {
    const src = localSrc();
    if (!src) {
        tapEl.pause();
        return;
    }
    if (tapEl.src !== src) tapEl.src = src;
    if (Number.isFinite(audioEl.currentTime) && Math.abs(tapEl.currentTime - audioEl.currentTime) > 0.3) {
        tapEl.currentTime = audioEl.currentTime;
    }
    if (audioEl.paused) tapEl.pause();
    else void tapEl.play().catch(() => {});
}

let broken = false;
let ctx: AudioContext | null = null;

function openContext(): void {
    if (broken) return;
    if (!ctx) {
        try {
            const Ctor = window.AudioContext
                ?? (window as { webkitAudioContext?: typeof AudioContext }).webkitAudioContext;
            if (!Ctor) throw new Error("no AudioContext");
            ctx = new Ctor();
            ctx.addEventListener("statechange", () => {
                if (ctx?.state === "suspended" && !tapEl.paused) void ctx.resume();
            });
        } catch {
            broken = true;
            return;
        }
    }
    void ctx.resume();
}
document.addEventListener("pointerdown", openContext, { capture: true });
document.addEventListener("keydown", openContext, { capture: true });

function attach(): boolean {
    if (analyser) return true;
    if (!ctx || broken || !localSrc()) return false;
    try {
        const node = ctx.createAnalyser();
        node.fftSize = 512;
        node.smoothingTimeConstant = 0.75;
        ctx.createMediaElementSource(tapEl).connect(node);
        bins = new Uint8Array(node.frequencyBinCount);
        analyser = node;
        resize();
        return true;
    } catch {
        broken = true;
        return false;
    }
}

audioEl.addEventListener("playing", () => {
    void ctx?.resume();
    syncTap();
    if (!localSrc()) return;
    if (!attach()) return;
    if (!running) {
        running = true;
        requestAnimationFrame(frame);
    }
});
audioEl.addEventListener("pause", () => tapEl.pause());
audioEl.addEventListener("seeked", syncTap);
audioEl.addEventListener("timeupdate", () => {
    if (!localSrc() || tapEl.paused) return;
    if (Math.abs(tapEl.currentTime - audioEl.currentTime) > 0.5) tapEl.currentTime = audioEl.currentTime;
});

new ResizeObserver(resize).observe(barEl);
