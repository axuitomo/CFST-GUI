// Copyright (C) 2026 axuitomo
// SPDX-License-Identifier: GPL-3.0-only

import assert from "node:assert/strict";
import { mkdtempSync, mkdirSync, rmSync, writeFileSync } from "node:fs";
import os from "node:os";
import path from "node:path";
import test from "node:test";
import { collectInlineLicenseNotices } from "./inline-license-notices.mjs";

test("collect independent line and block licenses, excluding ordinary headers and tests", () => {
  const dir = mkdtempSync(path.join(os.tmpdir(), "cfst-inline-licenses-"));
  try {
    const lineLicense = "// Copyright Example\n//\n// Permission is hereby granted to copy.\n// THE SOFTWARE IS PROVIDED AS IS.\n";
    const blockLicense = "/* Copyright Other\n * Redistribution and use are permitted.\n * DISCLAIMER\n */\n";
    writeFileSync(path.join(dir, "memmove.s"), `${lineLicense}\n#include \"go_asm.h\"\n`);
    writeFileSync(path.join(dir, "ringqueue.go"), `${blockLicense}\npackage queue\n`);
    writeFileSync(path.join(dir, "ordinary.go"), "// Copyright The Go Authors.\n// Use is governed by the LICENSE file.\npackage ordinary\n");
    writeFileSync(path.join(dir, "ignored_test.go"), lineLicense);
    mkdirSync(path.join(dir, "testdata"));
    writeFileSync(path.join(dir, "testdata/ignored.c"), blockLicense);
    const notices = collectInlineLicenseNotices(dir);
    assert.deepEqual(notices, [
      { relativePath: "memmove.s", text: lineLicense.trim() },
      { relativePath: "ringqueue.go", text: blockLicense.trim() },
    ]);
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
});
