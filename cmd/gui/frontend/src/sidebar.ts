import { createViewGroup } from "./views";

const items = document.querySelectorAll<HTMLButtonElement>(".sidebar-item");
const views = createViewGroup([...document.querySelectorAll<HTMLElement>(".view")]);

for (const item of items) {
    item.addEventListener("click", () => {
        for (const other of items) other.classList.toggle("is-active", other === item);
        views.show(item.dataset.view!);
    });
}
