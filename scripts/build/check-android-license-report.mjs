// Copyright (C) 2026 axuitomo
// SPDX-License-Identifier: GPL-3.0-only

import { readFileSync, writeFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../..");
const trackedPath = path.join(root, "scripts/build/licenses/android-release-runtime.json");
const generatedPath = path.join(root, "apps/android/app/build/reports/dependency-license/android-release-runtime.json");
const mode = process.argv[2] ?? "--check";
const compare = (a, b) => a < b ? -1 : a > b ? 1 : 0;

function canonicalReport(filePath) {
  const report = JSON.parse(readFileSync(filePath, "utf8"));
  if (!Array.isArray(report.dependencies)) throw new Error(`No dependency list in ${filePath}`);
  const dependencies = report.dependencies.map((item) => ({
    moduleName: item.moduleName,
    moduleVersion: item.moduleVersion,
    moduleUrls: [...(item.moduleUrls ?? [])].sort(compare),
    moduleLicenses: (item.moduleLicenses ?? [])
      .map((license) => ({
        moduleLicense: license.moduleLicense ?? null,
        moduleLicenseUrl: license.moduleLicenseUrl ?? null,
      }))
      .sort((a, b) => compare(`${a.moduleLicense}|${a.moduleLicenseUrl}`, `${b.moduleLicense}|${b.moduleLicenseUrl}`)),
  })).sort((a, b) => compare(a.moduleName, b.moduleName) || compare(a.moduleVersion, b.moduleVersion));
  if (!Array.isArray(report.artifactLegalTexts) || report.artifactLegalTexts.length === 0) {
    throw new Error(`No archive license/notice texts in ${filePath}; regenerate the Gradle report`);
  }
  const artifactLegalTexts = report.artifactLegalTexts.map((item) => {
    if (!item.text?.trim() || !item.sources?.length) throw new Error(`Invalid archive legal text in ${filePath}`);
    return { sources: [...item.sources].sort(compare), text: item.text.replace(/\r\n/g, "\n").trimEnd() };
  }).sort((a, b) => compare(a.sources.join("|"), b.sources.join("|")));
  return `${JSON.stringify({ dependencies, artifactLegalTexts }, null, 2)}\n`;
}

const generated = canonicalReport(generatedPath);
if (mode === "--update") {
  writeFileSync(trackedPath, generated);
} else if (mode === "--check") {
  const tracked = canonicalReport(trackedPath);
  if (tracked !== generated) {
    console.error("Android releaseRuntimeClasspath changed; refresh scripts/build/licenses/android-release-runtime.json and regenerate THIRD-PARTY-NOTICES.md.");
    process.exitCode = 1;
  }
} else {
  throw new Error(`Unknown mode: ${mode}`);
}
