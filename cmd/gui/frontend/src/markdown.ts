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

// Feed content is untrusted; only allow schemes that can't run script or read local files.
const SAFE_PROTOCOL = /^(?:https?:|mailto:)/i;
const HAS_PROTOCOL = /^[a-z][a-z0-9+.-]*:/i;

function safeHref(href: string): string | null {
    return HAS_PROTOCOL.test(href) && !SAFE_PROTOCOL.test(href) ? null : href;
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

export function renderMarkdown(md: string): string {
    return marked.parse(md, { async: false });
}
