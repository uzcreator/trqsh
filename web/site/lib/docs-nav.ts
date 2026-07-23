// Client-safe docs navigation data + types. This file must NOT import node:fs /
// node:path / marked, because it's pulled into the browser bundle by the sidebar
// and TOC. The server-only Markdown pipeline (fs, marked) lives in ./docs.ts,
// which re-exports everything here.

export interface DocEntry {
  slug: string;
  title: string;
  description?: string;
  type: "md" | "route";
}
export interface DocCategory {
  title: string;
  items: DocEntry[];
}
export interface TocItem {
  level: 2 | 3;
  text: string;
  id: string;
}

export const docsNav: DocCategory[] = [
  {
    title: "Getting started",
    items: [
      { slug: "quickstart", title: "Quickstart", description: "A live public URL in under a minute.", type: "md" },
      { slug: "installation", title: "Installation", description: "Install the CLI and desktop app on any OS.", type: "md" },
      { slug: "authentication", title: "Authentication", description: "Log in and manage API keys.", type: "md" },
    ],
  },
  {
    title: "Tunnels",
    items: [
      { slug: "http-tunnels", title: "HTTP tunnels", description: "Expose web apps and APIs.", type: "md" },
      { slug: "tcp-udp-tunnels", title: "TCP & UDP tunnels", description: "SSH, databases, game servers, DNS.", type: "md" },
      { slug: "reserved-subdomains", title: "Reserved subdomains", description: "Keep the same URL every time.", type: "md" },
      { slug: "custom-domains", title: "Custom domains", description: "Bring your own domain with DNS setup.", type: "md" },
    ],
  },
  {
    title: "Tools",
    items: [
      { slug: "inspector", title: "Request inspector", description: "Inspect and replay traffic locally.", type: "md" },
      { slug: "desktop-gui", title: "Desktop app", description: "The cross-platform trqsh GUI.", type: "md" },
      { slug: "configuration", title: "Configuration", description: "The ~/.trqsh-uz/trqsh.yml schema.", type: "md" },
    ],
  },
  {
    title: "Guides",
    items: [
      { slug: "webhooks-ci", title: "Webhooks & CI", description: "Receive webhooks and share dev builds.", type: "md" },
      { slug: "self-hosting", title: "Self-hosting", description: "Run the open-source agent your way.", type: "md" },
    ],
  },
  {
    title: "Reference",
    items: [
      { slug: "api", title: "API reference", description: "The Control API, from OpenAPI.", type: "route" },
      { slug: "errors", title: "Error reference", description: "Every error code and its fix.", type: "route" },
      { slug: "security", title: "Security & abuse", description: "Our security posture and reporting.", type: "md" },
    ],
  },
];

const allEntries = docsNav.flatMap((c) => c.items);

/** Slugs backed by a Markdown file — the only ones the [slug] route renders. */
export const mdSlugs = allEntries.filter((e) => e.type === "md").map((e) => e.slug);

export function docEntry(slug: string): DocEntry | undefined {
  return allEntries.find((e) => e.slug === slug);
}

/** The first Markdown doc — used as the "Start here" target. */
export const firstDocSlug = mdSlugs[0];
