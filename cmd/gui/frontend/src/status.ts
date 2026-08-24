import { requireEl } from "./dom";

const statusEl = requireEl<HTMLParagraphElement>("status");

export function setStatus(message: string, isError: boolean): void {
    if (!message) {
        statusEl.hidden = true;
        return;
    }
    statusEl.textContent = message;
    statusEl.classList.toggle("is-error", isError);
    statusEl.hidden = false;
}
