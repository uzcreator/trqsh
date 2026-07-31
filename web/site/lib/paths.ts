import path from "node:path";

// Build-time filesystem anchors. Read only in Server Components at build time
// — never ship to the client.
//
// docsContentDir still reaches two levels up into the (for now, still
// monorepo) backend repo's docs/ folder — process.cwd() is the site root
// (web/site) during `next dev`/`next build`. This will need to move into the
// site's own repo when it's split out (a separate, later concern).
//
// openapiPath, unlike docsContentDir, does NOT reach outside web/site: it's a
// local copy fetched from the live control API by scripts/gen-openapi.mjs
// (see that script — this mirrors how scripts/genplans.mjs already generates
// lib/catalog.generated.ts instead of reading backend source directly).
const repoRoot = path.join(process.cwd(), "..", "..");

export const docsDir = path.join(repoRoot, "docs");
export const docsContentDir = path.join(docsDir, "content");
export const openapiPath = path.join(process.cwd(), "lib", "openapi.generated.yaml");
