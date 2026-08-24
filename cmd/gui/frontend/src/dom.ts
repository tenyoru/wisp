export function requireEl<T extends HTMLElement>(id: string): T {
    const found = document.getElementById(id);
    if (!found) throw new Error(`missing #${id}`);
    return found as T;
}

export function el<K extends keyof HTMLElementTagNameMap>(
    tag: K,
    props: Partial<HTMLElementTagNameMap[K]> = {},
    children: Node[] = [],
): HTMLElementTagNameMap[K] {
    const node = document.createElement(tag);
    Object.assign(node, props);
    for (const child of children) node.append(child);
    return node;
}
