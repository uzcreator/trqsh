import path from "node:path";

// Build-time filesystem anchors. During `next dev` and `next build`, process.cwd()
// is the site root (web/site), so the repo `docs/` folder is two levels up. These
// are read only in Server Components at build time — never ship to the client.
const repoRoot = path.join(process.cwd(), "..", "..");

export const docsDir = path.join(repoRoot, "docs");
export const docsContentDir = path.join(docsDir, "content");
export const openapiPath = path.join(docsDir, "openapi.yaml");
