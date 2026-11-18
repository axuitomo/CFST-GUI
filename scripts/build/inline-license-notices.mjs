// Copyright (C) 2026 axuitomo
// SPDX-License-Identifier: GPL-3.0-only

import { readFileSync, readdirSync } from "node:fs";
import path from "node:path";

// Some Go/assembly/C sources carry independent grants only in comments.
export function collectInlineLicenseNotices(directory, relative = "") {
  const notices = [];
  for (const entry of readdirSync(directory, { withFileTypes: true }).sort((a, b) => a.name < b.name ? -1 : a.name > b.name ? 1 : 0)) {
    const relativePath = relative ? `${relative}/${entry.name}` : entry.name;
    const fullPath = path.join(directory, entry.name);
    if (entry.isDirectory()) {
      if (![".git", "node_modules", "cmd", "testdata", "tests", "examples"].includes(entry.name)) {
        notices.push(...collectInlineLicenseNotices(fullPath, relativePath));
      }
    } else if (entry.isFile() && /\.(?:go|s|c|h|m|mm|cc|cpp)$/i.test(entry.name) && !entry.name.endsWith("_test.go")) {
      const text = readFileSync(fullPath, "utf8").replace(/\r\n/g, "\n");
      const blocks = text.match(/(?:(?:^[ \t]*\/\/[^\n]*(?:\n|$))|(?:^[ \t]*\/\*[\s\S]*?\*\/[ \t]*(?:\n|$))|(?:^[ \t]*\n))+/gm) ?? [];
      for (const block of blocks) {
        if (/copyright/i.test(block) && /permission[\s\S]*?(?:grant|use|copy)|redistribution and use|SPDX-License-Identifier:/i.test(block)) {
          notices.push({ relativePath, text: block.trim() });
        }
      }
    }
  }
  return notices;
}
