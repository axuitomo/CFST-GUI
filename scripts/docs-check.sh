#!/usr/bin/env bash
set -euo pipefail

# shellcheck source=scripts/lib/common.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/common.sh"

node - "$ROOT_DIR" <<'NODE'
const fs = require("fs");
const path = require("path");

const root = process.argv[2];
const markdownFiles = [];

function walk(dir) {
  if (!fs.existsSync(dir)) return;
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    const full = path.join(dir, entry.name);
    if (entry.isDirectory()) walk(full);
    else if (entry.isFile() && entry.name.endsWith(".md")) markdownFiles.push(full);
  }
}

markdownFiles.push(path.join(root, "README.md"));
markdownFiles.push(path.join(root, "介绍产品.md"));
walk(path.join(root, "docs"));

let failures = 0;

function fail(file, line, message) {
  console.error(`${path.relative(root, file)}:${line}: ${message}`);
  failures++;
}

for (const file of markdownFiles) {
  const text = fs.readFileSync(file, "utf8");
  const lines = text.split(/\r?\n/);
  lines.forEach((lineText, index) => {
    const line = index + 1;
    const linkRe = /\[[^\]]+\]\(([^)]+)\)/g;
    for (const match of lineText.matchAll(linkRe)) {
      const raw = match[1].trim();
      if (!raw || raw.startsWith("#")) continue;
      if (/^(https?:|mailto:|tel:)/i.test(raw)) continue;
      if (/^\$\{/.test(raw)) continue;

      const withoutFragment = raw.split("#")[0];
      if (!withoutFragment) continue;
      const decoded = decodeURIComponent(withoutFragment).replace(/:\d+(?::\d+)?$/, "");
      const target = path.isAbsolute(decoded) ? decoded : path.resolve(path.dirname(file), decoded);
      if (!fs.existsSync(target)) {
        fail(file, line, `broken local markdown link: ${raw}`);
      }
    }

    const scriptRe = /(?:^|\s)(?:\.\/)?(scripts\/[A-Za-z0-9_.\/-]+\.sh)\b/g;
    for (const match of lineText.matchAll(scriptRe)) {
      const scriptPath = path.join(root, match[1]);
      if (!fs.existsSync(scriptPath)) {
        fail(file, line, `referenced script does not exist: ${match[1]}`);
      }
    }
  });
}

if (failures > 0) {
  console.error(`Documentation checks failed with ${failures} issue(s).`);
  process.exit(1);
}

console.log(`Documentation checks passed for ${markdownFiles.length} markdown file(s).`);
NODE
