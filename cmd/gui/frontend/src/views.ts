export function createViewGroup(panels: HTMLElement[]): { show(name: string): void } {
    return {
        show(name) {
            for (const panel of panels) panel.hidden = panel.dataset.view !== name;
        },
    };
}
