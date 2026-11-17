// Copyright (C) 2026 axuitomo
// SPDX-License-Identifier: GPL-3.0-only

import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { readFileSync } from "node:fs";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../..");

test("frontend build embeds the canonical legal files", () => {
  for (const name of ["LICENSE", "THIRD-PARTY-NOTICES.md"]) {
    assert.equal(
      readFileSync(path.join(root, "frontend/dist", name), "utf8"),
      readFileSync(path.join(root, name), "utf8"),
      `${name} is missing or stale; run the frontend build`,
    );
  }
});

test("notices include every resolved production npm package version", () => {
  const command = process.platform === "win32" ? "pwsh" : "pnpm";
  const args = process.platform === "win32"
    ? ["-NoProfile", "-Command", "pnpm licenses list --prod --json"]
    : ["licenses", "list", "--prod", "--json"];
  const result = spawnSync(command, args, { cwd: path.join(root, "frontend"), encoding: "utf8" });
  assert.ifError(result.error);
  assert.equal(result.status, 0, result.stderr);
  const notices = readFileSync(path.join(root, "THIRD-PARTY-NOTICES.md"), "utf8");
  const entries = Object.values(JSON.parse(result.stdout)).flat();
  assert.ok(entries.length > 0);
  for (const entry of entries) {
    for (const version of entry.versions) {
      assert.ok(notices.includes(`| ${entry.name} | ${version} |`), `${entry.name}@${version} is missing`);
    }
  }
});

test("notices retain bundled CSS and inline source attributions", () => {
  const notices = readFileSync(path.join(root, "THIRD-PARTY-NOTICES.md"), "utf8");
  for (const author of ["Tailwind Labs", "Erik Dubbelboer", "Lucent Technologies", "Vita Nuova"]) {
    assert.ok(notices.includes(author), `${author} attribution is missing`);
  }
});

test("built Go binary embeds the canonical legal files", { skip: !process.env.CFST_LEGAL_BINARY }, () => {
  const binary = readFileSync(process.env.CFST_LEGAL_BINARY);
  for (const name of ["LICENSE", "THIRD-PARTY-NOTICES.md"]) {
    assert.ok(binary.includes(readFileSync(path.join(root, name))), `${name} is missing from the Go binary`);
  }
});
