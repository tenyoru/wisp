import { marked } from "marked";
import { markedHighlight } from "marked-highlight";
import hljs from "highlight.js/lib/common";
import "highlight.js/styles/github-dark.css";

function escapeHtml(s: string): string {
    return s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
}

marked.use({ renderer: { html: ({ text }) => escapeHtml(text) } });
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
