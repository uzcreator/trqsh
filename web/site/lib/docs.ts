import fs from "node:fs";
import path from "node:path";
import { marked } from "marked";
import { docsContentDir } from "./paths";
import type { TocItem } from "./docs-nav";

// Server-only Markdown pipeline. Re-exports the client-safe nav data so server
// components can import slugs, entries, and the renderer from one place; client
// components must import nav data from ./docs-nav directly (this file pulls in
// node:fs and would break the browser bundle).
export * from "./docs-nav";

export interface RenderedDoc {
  html: string;
  toc: TocItem[];
}

function makeSlugger() {
  const seen = new Map<string, number>();
  return (text: string): string => {
    const base =
      text
        .toLowerCase()
        .replace(/[^\w\s-]/g, "")
        .trim()
        .replace(/\s+/g, "-") || "section";
    const n = seen.get(base) ?? 0;
    seen.set(base, n + 1);
    return n === 0 ? base : `${base}-${n}`;
  };
}

/** Render Markdown to HTML, adding stable ids + anchor links to h2/h3 and a TOC. */
export function renderMarkdown(md: string): RenderedDoc {
  const raw = marked.parse(md, { async: false, gfm: true }) as string;
  const slug = makeSlugger();
  const toc: TocItem[] = [];
  const html = raw.replace(/<(h[23])>([\s\S]*?)<\/\1>/g, (_m, tag: string, inner: string) => {
    const text = inner.replace(/<[^>]+>/g, "").trim();
    const id = slug(text);
    toc.push({ level: tag === "h2" ? 2 : 3, text, id });
    return `<${tag} id="${id}">${inner}<a class="anchor" href="#${id}" aria-hidden="true">#</a></${tag}>`;
  });
  return { html, toc };
}

/** Read + render a docs Markdown file. Returns null when the file is absent. */
export function loadDoc(slug: string): RenderedDoc | null {
  const file = path.join(docsContentDir, `${slug}.md`);
  let md: string;
  try {
    md = fs.readFileSync(file, "utf8");
  } catch {
    return null;
  }
  return renderMarkdown(md);
}
