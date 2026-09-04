import { requireEl } from "./dom";

const scroller = requireEl<HTMLElement>("main-scroll");
const thumb = requireEl<HTMLElement>("scroll-thumb");
const nav = requireEl<HTMLElement>("post-nav");
const player = requireEl<HTMLElement>("now-playing-bar");

function inset(): { top: number; bottom: number } {
    return {
        top: nav.hidden ? 8 : nav.offsetHeight + 4,
        bottom: player.hidden ? 8 : player.offsetHeight + 4,
    };
}

function sync(): void {
    const { scrollTop, scrollHeight, clientHeight } = scroller;
    const overflow = scrollHeight - clientHeight;
    if (overflow <= 0) {
        thumb.hidden = true;
        return;
    }
    const { top, bottom } = inset();
    const trackH = clientHeight - top - bottom;
    const thumbH = Math.max(24, (clientHeight / scrollHeight) * trackH);
    thumb.hidden = false;
    thumb.style.height = `${thumbH}px`;
    thumb.style.top = `${top + (scrollTop / overflow) * (trackH - thumbH)}px`;
}

scroller.addEventListener("scroll", sync, { passive: true });
window.addEventListener("resize", sync);
new ResizeObserver(sync).observe(scroller);
new MutationObserver(sync).observe(nav, { attributes: true, attributeFilter: ["hidden"] });
new MutationObserver(sync).observe(player, { attributes: true, attributeFilter: ["hidden"] });

let drag: { y: number; scroll: number } | null = null;
thumb.addEventListener("pointerdown", (e) => {
    e.preventDefault();
    window.getSelection()?.removeAllRanges();
    document.documentElement.classList.add("is-dragging-scroll");
    drag = { y: e.clientY, scroll: scroller.scrollTop };
    thumb.setPointerCapture(e.pointerId);
});
thumb.addEventListener("pointermove", (e) => {
    if (!drag) return;
    const { scrollHeight, clientHeight } = scroller;
    const overflow = scrollHeight - clientHeight;
    const { top, bottom } = inset();
    const maxThumb = clientHeight - top - bottom - thumb.offsetHeight;
    if (maxThumb <= 0) return;
    scroller.scrollTop = drag.scroll + ((e.clientY - drag.y) / maxThumb) * overflow;
});
thumb.addEventListener("pointerup", () => {
    drag = null;
    document.documentElement.classList.remove("is-dragging-scroll");
});

sync();
