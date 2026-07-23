#!/usr/bin/env node
"use strict";

// Thin launcher: exec the vendored trqsh binary, forwarding args, stdio and the
// exit code. If the binary is missing (e.g. `npm install --ignore-scripts`), it
// is downloaded on first run before executing.

const fs = require("fs");
const { spawnSync } = require("child_process");
const C = require("../lib/common");

async function main() {
  if (!fs.existsSync(C.binPath)) {
    await require("../lib/install").install();
  }
  const result = spawnSync(C.binPath, process.argv.slice(2), { stdio: "inherit" });
  if (result.error) {
    process.stderr.write(`trqsh: ${result.error.message}\n`);
    process.exit(1);
  }
  process.exit(result.status === null ? 1 : result.status);
}

main().catch((err) => {
  process.stderr.write(`trqsh: ${err.message}\n`);
  process.exit(1);
});
