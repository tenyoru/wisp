const items = document.querySelectorAll<HTMLButtonElement>(".sidebar-item");

for (const item of items) {
    item.addEventListener("click", () => {
        for (const other of items) other.classList.toggle("is-active", other === item);
    });
}
