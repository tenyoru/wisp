// Shared by the sidebar's top-level tabs and any drill-down (e.g. the feed
// list/detail split) — one place that knows how to show a named panel.
export function createViewGroup(panels: HTMLElement[]): { show(name: string): void } {
    return {
        show(name) {
            for (const panel of panels) panel.hidden = panel.dataset.view !== name;
        },
    };
}
