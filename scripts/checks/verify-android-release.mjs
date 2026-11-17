// Copyright (C) 2026 axuitomo
// SPDX-License-Identifier: GPL-3.0-only
import { createHash } from "node:crypto";
import { existsSync, mkdtempSync, readFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("../../", import.meta.url));

function run(command, args, options = {}) {
  const result = spawnSync(command, args, {
    encoding: "utf8",
    maxBuffer: 8 * 1024 * 1024,
    ...options,
  });
  if (result.error) throw new Error(`Cannot execute ${path.basename(command)}: ${result.error.code}`);
  if (result.status !== 0) {
    throw new Error(`${path.basename(command)} verification failed: ${(result.stderr || result.stdout).trim()}`);
  }
  return result.stdout;
}

export function signingCertificateDigest(output) {
  const digests = new Set([...output.matchAll(/^(?:Signer #\d+|V\d+(?:\.\d+)? Signer).*?certificate SHA-256 digest:[ \t]*([a-f0-9]{64})[ \t]*$/gim)].map((match) => match[1].toLowerCase()));
  if (!/^Number of signers:[ \t]*1[ \t]*\r?$/m.test(output) || digests.size !== 1) {
    throw new Error("Expected exactly one verified APK signing identity.");
  }
  return [...digests][0];
}

function main() {
  if (process.argv.length !== 3) throw new Error("Usage: node scripts/checks/verify-android-release.mjs <release.apk>");
  const apk = path.resolve(process.argv[2]);
  const sdk = process.env.ANDROID_SDK_ROOT || process.env.ANDROID_HOME;
  if (!sdk) throw new Error("Set ANDROID_SDK_ROOT or ANDROID_HOME to verify the Release APK.");
  const buildTools = path.join(sdk, "build-tools", "37.0.0");
  const signerJar = path.join(buildTools, "lib", "apksigner.jar");
  if (!existsSync(apk) || !existsSync(signerJar)) throw new Error("Release APK or pinned Android signing tools are missing.");

  const signature = run("java", ["-jar", signerJar, "verify", "--verbose", "--print-certs", apk]);
  const digest = signingCertificateDigest(signature);
  if (/certificate DN:.*\bCN\s*=\s*Android Debug\b/i.test(signature)) {
    throw new Error("A debug signing certificate cannot be used for the Release APK.");
  }
  const aapt = path.join(buildTools, process.platform === "win32" ? "aapt.exe" : "aapt");
  const manifest = run(aapt, ["dump", "badging", apk]);
  if (!/^package: name='io\.github\.axuitomo\.cfstgui'/m.test(manifest) || /^application-debuggable\b/m.test(manifest)) {
    throw new Error("The Release APK has an unexpected package identity or is debuggable.");
  }

  for (const name of ["CFST_ANDROID_KEYSTORE", "CFST_ANDROID_KEYSTORE_PASSWORD", "CFST_ANDROID_KEY_ALIAS"]) {
    if (!process.env[name]?.trim()) throw new Error(`Missing signing identity configuration: ${name}`);
  }
  const work = mkdtempSync(path.join(tmpdir(), "cfst-android-signature-"));
  try {
    const certificate = path.join(work, "signer.der");
    run("keytool", [
      "-exportcert", "-keystore", process.env.CFST_ANDROID_KEYSTORE,
      "-alias", process.env.CFST_ANDROID_KEY_ALIAS,
      "-storepass:env", "CFST_ANDROID_KEYSTORE_PASSWORD", "-file", certificate,
    ]);
    const expected = createHash("sha256").update(readFileSync(certificate)).digest("hex");
    if (digest !== expected) throw new Error("The APK signer does not match the configured Release key.");
    run("jar", ["xf", apk, "assets/public/LICENSE", "assets/public/THIRD-PARTY-NOTICES.md"], { cwd: work });
    for (const name of ["LICENSE", "THIRD-PARTY-NOTICES.md"]) {
      const bundled = readFileSync(path.join(work, "assets", "public", name));
      if (!bundled.equals(readFileSync(path.join(root, name)))) {
        throw new Error(`Signed APK legal text differs from the canonical file: ${name}`);
      }
    }
    console.log("Verified Android Release signature, configured signing identity, non-debug manifest, and canonical legal assets.");
  } finally {
    rmSync(work, { recursive: true, force: true });
  }
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  try {
    main();
  } catch (error) {
    console.error(error.message);
    process.exitCode = 1;
  }
}
