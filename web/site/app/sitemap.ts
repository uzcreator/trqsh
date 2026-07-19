import type { MetadataRoute } from "next";
import { site } from "@/lib/site";
import { mdSlugs } from "@/lib/docs";

export const dynamic = "force-static";

export default function sitemap(): MetadataRoute.Sitemap {
  const base = site.siteUrl.replace(/\/$/, "");
  const staticPaths = ["", "/pricing", "/download", "/docs", "/docs/api", "/docs/errors", "/terms", "/privacy"];
  const docPaths = mdSlugs.map((s) => `/docs/${s}`);
  const now = new Date();
  return [...staticPaths, ...docPaths].map((path) => ({
    url: `${base}${path}`,
    lastModified: now,
    changeFrequency: "weekly",
    priority: path === "" ? 1 : path.startsWith("/docs") ? 0.6 : 0.8,
  }));
}
