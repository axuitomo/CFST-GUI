// Copyright (C) 2026 axuitomo
// SPDX-License-Identifier: GPL-3.0-only
import assert from "node:assert/strict";
import { existsSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";
import test from "node:test";
import { signingCertificateDigest } from "./verify-android-release.mjs";

const verifier = fileURLToPath(new URL("./verify-android-release.mjs", import.meta.url));
const debugApk = fileURLToPath(new URL("../../apps/android/app/build/outputs/apk/debug/app-arm64-v8a-debug.apk", import.meta.url));
const sdk = process.env.ANDROID_SDK_ROOT || process.env.ANDROID_HOME;
const skip = !sdk || !existsSync(debugApk);

function verify(apk) {
  return spawnSync(process.execPath, [verifier, apk], { encoding: "utf8" });
}

test("accept legacy signer output with CRLF", () => {
  const digest = "ab".repeat(32);
  assert.equal(signingCertificateDigest(`Number of signers: 1\r\nSigner #1 certificate SHA-256 digest: ${digest}\r\n`), digest);
});

test("accept SDK 37 per-scheme output and deduplicate the same signing certificate", () => {
  const digest = "ab".repeat(32);
  const output = `Number of signers: 1\nV2 Signer: certificate SHA-256 digest: ${digest}\nV3 Signer: certificate SHA-256 digest: ${digest.toUpperCase()}\nSource Stamp Signer: certificate SHA-256 digest: ${"cd".repeat(32)}\n`;
  assert.equal(signingCertificateDigest(output), digest);
});

test("reject multiple signers, conflicting certificates, and missing digests", () => {
  assert.throws(() => signingCertificateDigest(`Number of signers: 2\nSigner #1 certificate SHA-256 digest: ${"ab".repeat(32)}\n`), /exactly one/);
  assert.throws(() => signingCertificateDigest(`Number of signers: 1\nV2 Signer: certificate SHA-256 digest: ${"ab".repeat(32)}\nV3 Signer: certificate SHA-256 digest: ${"cd".repeat(32)}\n`), /exactly one/);
  assert.throws(() => signingCertificateDigest("Number of signers: 1\n"), /exactly one/);
});

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
