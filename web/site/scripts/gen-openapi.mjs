#!/usr/bin/env node
// Fetches the control API's public OpenAPI spec (GET /openapi.yaml —
// internal/api/docs.go) and saves it locally as
// web/site/lib/openapi.generated.yaml, so the /docs/api reference page
// (web/site/lib/openapi.ts) can render it without reaching into the backend
// repo's docs/ folder — which won't exist in this checkout once the site is
// split into its own repo.
//
// Regenerate: node scripts/gen-openapi.mjs   (or: make site-openapi)
// Like genplans.mjs, this is NOT part of a normal `pnpm build` — only CI's
// drift check and explicit local regeneration invoke it.

import { writeFile } from "node:fs/promises";

const apiUrl = process.env.TRQSH_API_URL || "https://api.trqsh.uz";

async function main() {
  const url = `${apiUrl}/openapi.yaml`;
  const res = await fetch(url);
  if (!res.ok) {
    throw new Error(`fetch ${url}: HTTP ${res.status}`);
  }
  const text = await res.text();

  const dest = new URL("../lib/openapi.generated.yaml", import.meta.url);
  await writeFile(dest, text);
  console.log(`wrote ${dest.pathname}`);
}

main().catch((err) => {
  console.error("gen-openapi:", err.message);
  process.exit(1);
});
