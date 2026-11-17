// Copyright (C) 2026 axuitomo
// SPDX-License-Identifier: GPL-3.0-only
import assert from "node:assert/strict";
import { existsSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";
import test from "node:test";

const verifier = fileURLToPath(new URL("./verify-android-release.mjs", import.meta.url));
const debugApk = fileURLToPath(new URL("../../mobile/android/app/build/outputs/apk/debug/app-arm64-v8a-debug.apk", import.meta.url));
const sdk = process.env.ANDROID_SDK_ROOT || process.env.ANDROID_HOME;
const skip = !sdk || !existsSync(debugApk);

function verify(apk) {
  return spawnSync(process.execPath, [verifier, apk], { encoding: "utf8" });
}

test("Release verification rejects the real Debug APK signing certificate", { skip }, () => {
  const result = verify(debugApk);
  assert.equal(result.status, 1);
  assert.match(result.stderr, /debug signing certificate cannot be used/);
});

test("Release verification rejects APK contents modified after signing", { skip }, () => {
  const work = mkdtempSync(path.join(tmpdir(), "cfst-tampered-apk-"));
  try {
    const bytes = Buffer.from(readFileSync(debugApk));
    assert.equal(bytes.readUInt32LE(0), 0x04034b50);
    const dataOffset = 30 + bytes.readUInt16LE(26) + bytes.readUInt16LE(28);
    bytes[dataOffset] ^= 1;
    const modified = path.join(work, "modified.apk");
    writeFileSync(modified, bytes);
    const result = verify(modified);
    assert.equal(result.status, 1);
    assert.match(result.stderr, /java verification failed:/);
  } finally {
    rmSync(work, { recursive: true, force: true });
  }
});
