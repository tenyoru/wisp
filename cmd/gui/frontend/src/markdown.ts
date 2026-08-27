import { marked } from "marked";
import { markedHighlight } from "marked-highlight";
import hljs from "highlight.js/lib/common";
import "highlight.js/styles/github-dark.css";

function escapeHtml(s: string): string {
    return s
        .replace(/&/g, "&amp;")
        .replace(/</g, "&lt;")
        .replace(/>/g, "&gt;")
        .replace(/"/g, "&quot;");
}

const SAFE_PROTOCOL = /^(?:https?:|mailto:)/i;
const HAS_PROTOCOL = /^[a-z][a-z0-9+.-]*:/i;

function safeHref(href: string): string | null {
    if (href.startsWith("#")) return null;
    return HAS_PROTOCOL.test(href) && !SAFE_PROTOCOL.test(href) ? null : href;
}

export interface TocHeading {
    level: number;
    text: string;
    id: string;
}

let toc: TocHeading[] = [];
const slugCounts = new Map<string, number>();

function slugify(text: string): string {
    return text.toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/^-+|-+$/g, "") || "section";
}

function uniqueSlug(base: string): string {
    const count = slugCounts.get(base) ?? 0;
    slugCounts.set(base, count + 1);
    return count === 0 ? base : `${base}-${count}`;
}

marked.use({
    renderer: {
        html: ({ text }) => escapeHtml(text),
        link({ href, title, tokens }) {
            const text = this.parser.parseInline(tokens);
            const safe = safeHref(href);
            if (!safe) return text;
            const titleAttr = title ? ` title="${escapeHtml(title)}"` : "";
            return `<a href="${escapeHtml(safe)}"${titleAttr}>${text}</a>`;
        },
        image({ href, title, text }) {
            const safe = safeHref(href);
            if (!safe) return escapeHtml(text);
            const titleAttr = title ? ` title="${escapeHtml(title)}"` : "";
            return `<img src="${escapeHtml(safe)}" alt="${escapeHtml(text)}"${titleAttr}>`;
        },
        heading({ tokens, depth }) {
            const html = this.parser.parseInline(tokens);
            const scratch = document.createElement("div");
            scratch.innerHTML = html;
            const text = scratch.textContent || "";
            const id = uniqueSlug(slugify(text));
            toc.push({ level: depth, text, id });
            return `<h${depth} id="${id}">${html}</h${depth}>`;
        },
    },
});
marked.use(
    markedHighlight({
        langPrefix: "hljs language-",
        highlight(code, lang) {
            if (lang && hljs.getLanguage(lang)) return hljs.highlight(code, { language: lang }).value;
            return hljs.highlightAuto(code).value;
        },
    }),
);

export function renderMarkdown(md: string): { html: string; toc: TocHeading[] } {
    toc = [];
    slugCounts.clear();
    const html = marked.parse(md, { async: false });
    return { html, toc };
}
