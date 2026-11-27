// Copyright (C) 2026 axuitomo
// SPDX-License-Identifier: GPL-3.0-only

import { spawnSync } from "node:child_process";
import { createHash } from "node:crypto";
import {
  existsSync,
  mkdirSync,
  mkdtempSync,
  readFileSync,
  readdirSync,
  rmSync,
  writeFileSync,
} from "node:fs";
import os from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { collectInlineLicenseNotices } from "./inline-license-notices.mjs";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../..");
const frontend = path.join(root, "packages/frontend");
const output = path.join(root, "THIRD-PARTY-NOTICES.md");
const androidReportPath = path.join(root, "scripts/build/licenses/android-release-runtime.json");
const apacheLicensePath = path.join(root, "scripts/build/licenses/apache-2.0.txt");
const mode = process.argv[2] ?? "--stdout";
const compare = (a, b) => a < b ? -1 : a > b ? 1 : 0;
const textGroups = new Map();
const packageRows = [];
const goModules = new Map();
let goToolchainVersion = "";

function run(command, args, cwd, env = process.env) {
  const result = spawnSync(command, args, { cwd, env, encoding: "utf8" });
  if (result.error) throw result.error;
  if (result.status !== 0) {
    throw new Error(`${command} ${args.join(" ")} failed:\n${result.stderr}\n${result.stdout}`);
  }
  return result.stdout;
}

function runPnpmLicenseReport() {
  if (process.platform === "win32") {
    return run("pwsh", ["-NoProfile", "-Command", "pnpm licenses list --prod --json"], frontend);
  }
  return run("pnpm", ["licenses", "list", "--prod", "--json"], frontend);
}

function addText(license, text, source) {
  const content = text.replace(/\r\n/g, "\n").trimEnd();
  const digest = createHash("sha256").update(content).digest("hex");
  const group = textGroups.get(digest) ?? { content, licenses: new Set(), sources: new Set() };
  group.licenses.add(license);
  group.sources.add(source);
  textGroups.set(digest, group);
}


function addPackageFiles(entry, scope) {
  const packageDirs = entry.paths?.length ? entry.paths : [entry.packageDir];
  if (packageDirs.some((directory) => !directory)) throw new Error(`No installed path for ${entry.name}`);
  const foundVersions = new Set();
  for (const packageDir of new Set(packageDirs)) {
    const manifest = JSON.parse(readFileSync(path.join(packageDir, "package.json"), "utf8"));
    if (manifest.name !== entry.name) throw new Error(`Package path does not match ${entry.name}`);
    const version = manifest.version;
    foundVersions.add(version);
    const files = walkLegalFiles(packageDir);
    const legalPaths = files.length > 0
      ? files
      : entry.name === "@wailsio/runtime" && version === "3.2.0-nightly.20261103"
        ? [{ relativePath: "Wails LICENSE", path: path.join(root, "scripts/build/licenses/wails-runtime-MIT.txt") }]
        : [];
    if (legalPaths.length === 0) throw new Error(`No license or notice file found for ${entry.name}@${version}`);
    const author = typeof manifest.author === "string" ? manifest.author : manifest.author?.name ?? "Not specified";
    packageRows.push({ name: entry.name, version, license: entry.license, author, scope });
    for (const file of legalPaths) {
      addText(entry.license, readFileSync(file.path, "utf8"), `${entry.name}@${version} (${file.relativePath})`);
    }
  }
  for (const version of entry.versions ?? []) {
    if (!foundVersions.has(version)) throw new Error(`No installed legal files for ${entry.name}@${version}`);
  }
}

function walkLegalFiles(directory, relative = "") {
  const result = [];
  for (const entry of readdirSync(directory, { withFileTypes: true }).sort((a, b) => compare(a.name, b.name))) {
    const relativePath = relative ? `${relative}/${entry.name}` : entry.name;
    const fullPath = path.join(directory, entry.name);
    if (entry.isDirectory() && entry.name !== "node_modules" && entry.name !== ".git") result.push(...walkLegalFiles(fullPath, relativePath));
    else if (/^(?:license|licence|copying|notice)(?:[._-].*)?$/i.test(entry.name)) {
      result.push({ path: fullPath, relativePath });
    }
  }
  return result;
}

function androidLicenseIds(dependency) {
  const ids = new Set();
  for (const license of dependency.moduleLicenses ?? []) {
    const name = String(license.moduleLicense ?? "").toLowerCase();
    const url = String(license.moduleLicenseUrl ?? "").toLowerCase();
    if (name.includes("apache") || url.includes("apache.org/licenses/license-2.0")) {
      ids.add("Apache-2.0");
    } else {
      throw new Error(`Unmapped Android license for ${dependency.moduleName}@${dependency.moduleVersion}: ${license.moduleLicense ?? license.moduleLicenseUrl}`);
    }
  }
  if (ids.size === 0) throw new Error(`No Android POM license declared for ${dependency.moduleName}@${dependency.moduleVersion}`);
  return [...ids].sort(compare);
}

function loadAndroidReport() {
  const report = JSON.parse(readFileSync(androidReportPath, "utf8"));
  if (!Array.isArray(report.dependencies) || report.dependencies.length === 0) {
    throw new Error(`No Android runtime dependencies in ${androidReportPath}`);
  }
  const dependencies = [...report.dependencies].sort((a, b) =>
    compare(a.moduleName, b.moduleName) || compare(a.moduleVersion, b.moduleVersion),
  );
  for (const dependency of dependencies) {
    const licenses = androidLicenseIds(dependency);
    packageRows.push({
      name: dependency.moduleName,
      version: dependency.moduleVersion,
      license: licenses.join(" OR "),
      author: dependency.moduleUrls?.[0] ?? "Not specified",
      scope: "Android release runtime",
    });
  }
  addText("Apache-2.0", readFileSync(apacheLicensePath, "utf8"), "Android Release runtime (Apache-2.0)");
  if (!Array.isArray(report.artifactLegalTexts) || report.artifactLegalTexts.length === 0) {
    throw new Error("Android archive license/notice texts are missing; regenerate the Gradle report");
  }
  for (const notice of report.artifactLegalTexts) {
    if (!notice.text?.trim() || !notice.sources?.length) throw new Error("Invalid Android archive legal text");
    for (const source of notice.sources) addText("Android archive license/notice", notice.text, source);
  }
}

function moduleTableData(env) {
  const lines = run("go", ["list", "-m", "-f", "{{.Path}}|{{.Version}}|{{.Dir}}", "all"], root, env)
    .trim()
    .split(/\r?\n/);
  return lines.map((line) => {
    const [modulePath, version, directory] = line.split("|");
    return { path: modulePath, version, directory };
  }).filter((module) => module.path && module.version && module.path !== "github.com/axuitomo/CFST-GUI")
    .sort((a, b) => b.path.length - a.path.length || compare(a.path, b.path));
}

function collectGoToolchainNotices(env) {
  const goRoot = run("go", ["env", "GOROOT"], root, env).trim();
  goToolchainVersion = run("go", ["env", "GOVERSION"], root, env).trim();
  addText("BSD-3-Clause (Go Authors)", readFileSync(path.join(goRoot, "LICENSE"), "utf8"), `${goToolchainVersion} Go standard library/runtime (GOROOT/LICENSE)`);
  const patentsPath = path.join(goRoot, "PATENTS");
  if (existsSync(patentsPath)) addText("Go toolchain patent notice", readFileSync(patentsPath, "utf8"), `${goToolchainVersion} GOROOT/PATENTS`);
  const vendorRoot = path.join(goRoot, "src/vendor");
  if (existsSync(vendorRoot)) {
    for (const file of walkLegalFiles(vendorRoot)) {
      const relativePath = path.relative(vendorRoot, file.path).replaceAll("\\", "/");
      addText("Go standard library vendored license", readFileSync(file.path, "utf8"), `${goToolchainVersion} src/vendor/${relativePath}`);
    }
  }
  for (const notice of collectInlineLicenseNotices(path.join(goRoot, "src"))) {
    addText("Go standard library inline license", notice.text, `${goToolchainVersion} src/${notice.relativePath}`);
  }
}

function addGoLicenseFile(filePath, relativePath, target, modules) {
  const sourcePath = relativePath.replaceAll("\\", "/");
  const module = modules.find((item) => sourcePath === item.path || sourcePath.startsWith(`${item.path}/`));
  if (!module) throw new Error(`Could not map Go license file to a module: ${relativePath}`);
  const row = goModules.get(module.path) ?? { version: module.version, targets: new Set(), files: new Set() };
  row.targets.add(target);
  row.files.add(sourcePath);
  goModules.set(module.path, row);
  addText("Go module license", readFileSync(filePath, "utf8"), `${module.path}@${module.version} (${sourcePath}; ${target})`);
}

function collectGoLicenseFiles(tool, tempRoot, modules) {

  const targets = [
    { id: "windows", label: "Windows desktop/installer", env: { GOOS: "windows", GOARCH: "amd64", GOFLAGS: "-tags=production,tray" }, packages: ["./apps/desktop", "./apps/installer"] },
    { id: "linux-webui", label: "Linux WebUI", env: { GOOS: "linux", GOARCH: "amd64", GOFLAGS: "" }, packages: ["./apps/webui"] },
    { id: "linux-webui-arm64", label: "Linux WebUI arm64", env: { GOOS: "linux", GOARCH: "arm64", GOFLAGS: "" }, packages: ["./apps/webui"] },
    { id: "macos", label: "macOS desktop", env: { GOOS: "darwin", GOARCH: "amd64", GOFLAGS: "-tags=tray" }, packages: ["./apps/desktop"] },
    { id: "macos-arm64", label: "macOS desktop arm64", env: { GOOS: "darwin", GOARCH: "arm64", GOFLAGS: "-tags=tray" }, packages: ["./apps/desktop"] },
    { id: "android", label: "Android gomobile AAR", env: { GOOS: "android", GOARCH: "arm64", GOFLAGS: "" }, packages: ["./apps/android/bridge/mobileapi"] },
  ];
  for (const target of targets) {
    const savePath = path.join(tempRoot, target.id);
    const env = { ...process.env, ...target.env };
    run(tool, ["save", "--ignore", "github.com/axuitomo/CFST-GUI", ...target.packages, "--save_path", savePath], root, env);
    for (const file of walkLegalFiles(savePath)) {
      addGoLicenseFile(file.path, file.relativePath, target.label, modules);
    }
  }
}

function clean(value) {
  return String(value ?? "").replaceAll("|", "\\|").replaceAll("\n", " ").trim().replace(/https?:\/\/[^\s<>()]+/g, "<$&>") || "Not specified";
}

function renderNotice() {
  const lines = [
    "# Third-Party Notices",
    "",
    "CFST-GUI is licensed under GPL-3.0-only. This notice covers frontend production npm packages, bundled Tailwind CSS, Capacitor Android, resolved Go package graphs for release targets, Android releaseRuntimeClasspath, and the Go standard library/runtime bundled in Go executables. OS-provided libraries are excluded.",
    "",
    "Android graph regeneration (from `apps/android`): `./gradlew -I ../../scripts/build/android-license-report.init.gradle --no-configuration-cache :app:generateLicenseReport`, then `node scripts/build/check-android-license-report.mjs --update`; regenerate the combined notice with `node scripts/build/generate-third-party-notices.mjs --write`. Android release/debug builds rerun the Gradle report and check the snapshot.",
    "",
    "## Frontend and Capacitor Packages",
    "",
    "| Package | Version | License | Scope | Author metadata |",
    "| --- | --- | --- | --- | --- |",
  ];
  packageRows.sort((a, b) => compare(a.scope, b.scope) || compare(a.name, b.name) || compare(a.version, b.version));
  for (const entry of packageRows.filter((row) => row.scope !== "Android release runtime")) {
    lines.push(`| ${clean(entry.name)} | ${clean(entry.version)} | ${clean(entry.license)} | ${clean(entry.scope)} | ${clean(entry.author)} |`);
  }
  lines.push("", "## Android Release Runtime (Maven)", "", "Coordinates and declared licenses below come from resolved `releaseRuntimeClasspath` POM metadata. Legal texts found inside the resolved AAR/JAR archives (including nested JARs and local libraries) are also reproduced below.", "", "| Module | Version | License | POM project URL |", "| --- | --- | --- | --- |");
  for (const entry of packageRows.filter((row) => row.scope === "Android release runtime")) {
    lines.push(`| ${clean(entry.name)} | ${clean(entry.version)} | ${clean(entry.license)} | ${clean(entry.author)} |`);
  }
  lines.push("", "## Go Toolchain Runtime", "", `${goToolchainVersion} standard library/runtime code is bundled in the Go executables; its BSD license, patent notice, and vendored-source licenses are reproduced in the license-text section below.`);
  lines.push("", "## Go Dependencies by Release Target", "", "Go module license files were collected with google/go-licenses for Windows desktop/installer, Linux WebUI, macOS desktop, and the Android gomobile AAR, including both amd64 and arm64 Linux/macOS targets. The generated gomobile binding runtime is attributed from the pinned golang.org/x/mobile module. Supplemental legal files and inline grants from the standard library and selected module source trees are preserved as well; these can include files not linked on every target.", "", "| Module | Version | Targets | License files |", "| --- | --- | --- | --- |");
  const moduleEntries = [...goModules.entries()].sort(([a], [b]) => compare(a, b));
  for (const [modulePath, entry] of moduleEntries) {
    lines.push(`| ${clean(modulePath)} | ${clean(entry.version)} | ${clean([...entry.targets].sort(compare).join(", "))} | ${clean([...entry.files].sort(compare).join(", "))} |`);
  }
  lines.push("", "## Upstream Source Acknowledgment", "", "The project README identifies [XIU2/CloudflareSpeedTest](https://github.com/XIU2/CloudflareSpeedTest) as the basis of the initial core speed-testing logic. The retained upstream history credits xiu2, mz, mazhuang, Spedoske, mikuru996, CrazyBoyFeng, Mac_Zhou, charSLee013, kaka, zhangsean, and other contributors. The upstream `LICENSE` is GNU GPL Version 3; the complete GPLv3 text is included in this distribution's [LICENSE](LICENSE). The source history confirms lineage, but this notice does not claim that every current source file is verbatim upstream code.", "", "## License and Notice Texts", "");
  lines.push("<!-- markdownlint-disable MD010 -->", "");
  const groups = [...textGroups.values()].sort((a, b) => compare([...a.sources].sort(compare)[0], [...b.sources].sort(compare)[0]));
  for (const group of groups) {
    const licenses = [...group.licenses].sort(compare).join(", ");
    const sources = [...group.sources].sort(compare);
    lines.push(`### ${licenses}`, "", `Source files: ${sources.map((item) => `\`${item}\``).join(", ")}`, "", "````text", group.content, "````", "");
  }
  lines.push("<!-- markdownlint-enable MD010 -->");
  return `${lines.join("\n").replace(/\n+$/, "")}\n`;
}

const tempRoot = mkdtempSync(path.join(os.tmpdir(), "cfst-third-party-"));
try {
  const pnpmReport = JSON.parse(runPnpmLicenseReport());
  for (const [license, entries] of Object.entries(pnpmReport)) {
    for (const entry of entries) addPackageFiles({ ...entry, license }, "Frontend runtime");
  }
  for (const [name, scope] of [["@capacitor/android", "Android runtime"], ["tailwindcss", "Bundled CSS"]]) {
    const packageDir = path.join(frontend, "node_modules", name);
    const manifest = JSON.parse(readFileSync(path.join(packageDir, "package.json"), "utf8"));
    addPackageFiles({ name: manifest.name, license: manifest.license, packageDir }, scope);
  }
  loadAndroidReport();

  const goEnv = { ...process.env };
  for (const key of ["GOOS", "GOARCH", "GOFLAGS", "CGO_ENABLED"]) delete goEnv[key];
  const modules = moduleTableData(goEnv);
  collectGoToolchainNotices(goEnv);
  const goBin = path.join(tempRoot, "bin");
  mkdirSync(goBin, { recursive: true });
  run("go", ["install", "github.com/google/go-licenses/v2@v2.0.1"], root, { ...goEnv, GOBIN: goBin });
  const goLicenseTool = path.join(goBin, process.platform === "win32" ? "go-licenses.exe" : "go-licenses");
  collectGoLicenseFiles(goLicenseTool, tempRoot, modules);
  // Generated bindings import x/mobile runtime code; the bind package itself is a build tool.
  const mobileRuntime = modules.find((module) => module.path === "golang.org/x/mobile");
  if (!mobileRuntime) throw new Error("Pinned golang.org/x/mobile binding runtime is missing");
  run("go", ["mod", "download", `${mobileRuntime.path}@${mobileRuntime.version}`], root, goEnv);
  goModules.set(mobileRuntime.path, { version: mobileRuntime.version, targets: new Set(["Android gomobile binding runtime"]), files: new Set() });
  for (const module of moduleTableData(goEnv).filter((item) => goModules.has(item.path))) {
    if (!module.directory) throw new Error(`No source directory for ${module.path}@${module.version}`);
    for (const file of walkLegalFiles(module.directory)) {
      const sourcePath = `${module.path}/${file.relativePath}`;
      goModules.get(module.path).files.add(sourcePath);
      addText("Go module supplemental license/notice", readFileSync(file.path, "utf8"), `${module.path}@${module.version} (${sourcePath})`);
    }
    for (const notice of collectInlineLicenseNotices(module.directory)) {
      addText("Go module inline license", notice.text, `${module.path}@${module.version}/${notice.relativePath}`);
    }
  }

  const content = renderNotice();
  if (mode === "--write") {
    writeFileSync(output, content);
  } else if (mode === "--check") {
    let current = "";
    try { current = readFileSync(output, "utf8"); } catch {}
    if (current !== content) {
      console.error("THIRD-PARTY-NOTICES.md is missing or stale; refresh the Android report and run node scripts/build/generate-third-party-notices.mjs --write");
      process.exitCode = 1;
    }
  } else if (mode === "--stdout") {
    process.stdout.write(content);
  } else {
    throw new Error(`Unknown mode: ${mode}`);
  }
} finally {
  rmSync(tempRoot, { recursive: true, force: true });
}
