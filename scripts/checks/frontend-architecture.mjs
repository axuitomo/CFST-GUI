#!/usr/bin/env node
// Frontend Monorepo & Adapter Architecture Guard.
//
// Enforces the strict unidirectional dependency hierarchy under frontend/src:
//   views / components / composables -> lib/bridge.ts (facade)
//     -> lib/client/* (platform-agnostic RPC/event/file clients) & lib/bridge/* (pure contracts & codecs)
//       -> lib/adapters/{registry,desktop,webui,android}
//         -> lib/bridge/* (contracts/codecs only)
//
// Also enforces cross-adapter isolation (desktop / webui / android never cross-import
// each other) and zero module-level import cycles across frontend/src/lib/**.
//
// Invoked by frontend-boundary.{sh,ps1}, fast.{sh,ps1}, check.{sh,ps1}, and .githooks/pre-commit.

import { existsSync, readFileSync, readdirSync, statSync } from "node:fs";
import { dirname, join, relative, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));
const root = resolve(here, "..", "..");
const srcDir = join(root, "frontend", "src");

const errors = [];
const rel = (p) => relative(root, p).split("\\").join("/");
const add = (file, message) => errors.push({ file, message });

function walk(dir, cb) {
  if (!existsSync(dir)) return;
  for (const name of readdirSync(dir)) {
    if (name === "node_modules" || name === "dist" || name.startsWith(".")) continue;
    const p = join(dir, name);
    if (statSync(p).isDirectory()) walk(p, cb);
    else cb(p);
  }
}

const isTestFile = (repoPath) => /\.(test|browser\.test|spec)\.(ts|js|mjs)$/.test(repoPath);

function stripCommentsAndStringsForImports(source) {
  // Replace /* ... */ and // ... comments with spaces while keeping line count aligned.
  return source
    .replace(/\/\*[\s\S]*?\*\//g, (block) => block.replace(/[^\n]/g, " "))
    .replace(/(^|[^:])\/\/.*$/gm, "$1");
}

function extractScriptText(filePath, content) {
  if (!filePath.endsWith(".vue")) {
    return content;
  }
  const chunks = [];
  for (const match of content.matchAll(/<script\b[^>]*>([\s\S]*?)<\/script>/gi)) {
    chunks.push(match[1] || "");
  }
  return chunks.join("\n");
}

function extractRelativeSpecifiers(source) {
  const clean = stripCommentsAndStringsForImports(source);
  const specs = new Set();
  const fromRe = /\b(?:import|export)\s+(?:type\s+)?(?:[\s\S]*?\bfrom\s+)?["']([^"']+)["']/g;
  for (const match of clean.matchAll(fromRe)) {
    const spec = match[1];
    if (spec.startsWith(".")) specs.add(spec);
  }
  const dynRe = /\bimport\s*\(\s*["']([^"']+)["']\s*\)/g;
  for (const match of clean.matchAll(dynRe)) {
    const spec = match[1];
    if (spec.startsWith(".")) specs.add(spec);
  }
  return [...specs];
}

function resolveImport(importerAbs, spec) {
  const base = resolve(dirname(importerAbs), spec);
  const candidates = [
    base,
    `${base}.ts`,
    `${base}.js`,
    `${base}.vue`,
    join(base, "index.ts"),
    join(base, "index.js"),
  ];
  for (const candidate of candidates) {
    if (existsSync(candidate) && statSync(candidate).isFile()) {
      return rel(candidate);
    }
  }
  return null;
}

const allProdFiles = [];
walk(srcDir, (abs) => {
  const r = rel(abs);
  if (!/\.(ts|js|vue)$/.test(r) || r.endsWith(".d.ts") || isTestFile(r)) {
    return;
  }
  allProdFiles.push({ abs, rel: r });
});

const graph = new Map();
for (const entry of allProdFiles) {
  const raw = readFileSync(entry.abs, "utf8");
  const script = extractScriptText(entry.rel, raw);
  const deps = [];
  for (const spec of extractRelativeSpecifiers(script)) {
    const resolved = resolveImport(entry.abs, spec);
    if (resolved) {
      deps.push(resolved);
    }
  }
  graph.set(entry.rel, deps);
}

const adapterPlatformOf = (p) => {
  const match = /^frontend\/src\/lib\/adapters\/(desktop|webui|android)\//.exec(p);
  return match ? match[1] : null;
};

// --- Layer rules -------------------------------------------------------------
for (const [file, deps] of graph.entries()) {
  const isAdapter = file.startsWith("frontend/src/lib/adapters/");
  const isClient = file.startsWith("frontend/src/lib/client/");
  const isBridgeContracts = file.startsWith("frontend/src/lib/bridge/");
  const isFacade = file === "frontend/src/lib/bridge.ts";
  const isUILayer =
    file.startsWith("frontend/src/views/") ||
    file.startsWith("frontend/src/components/") ||
    file.startsWith("frontend/src/composables/");

  for (const dep of deps) {
    // 1. Adapters and clients may only be imported from approved internal layers.
    if ((dep.startsWith("frontend/src/lib/adapters/") || dep.startsWith("frontend/src/lib/client/")) && !isAdapter && !isClient && !isFacade) {
      add(file, `forbidden direct import of ${dep}; route through frontend/src/lib/bridge.ts`);
    }

    // 2. Facade bridge.ts routes via lib/client/* and lib/bridge/*, never direct adapters.
    if (isFacade && dep.startsWith("frontend/src/lib/adapters/")) {
      add(file, `bridge.ts facade must import from ./client/* or ./bridge/* instead of ${dep}`);
    }

    // 3. Pure bridge contracts & codecs cannot depend on runtime clients, adapters, or facade.
    if (isBridgeContracts && (dep.startsWith("frontend/src/lib/adapters/") || dep.startsWith("frontend/src/lib/client/") || dep === "frontend/src/lib/bridge.ts")) {
      add(file, `pure bridge contract module must not import ${dep}`);
    }

    // 4. Shared helpers under lib/ used outside client/adapters must not pull in adapters or clients.
    if (file.startsWith("frontend/src/lib/") && !isAdapter && !isClient && !isFacade && (dep.startsWith("frontend/src/lib/adapters/") || dep.startsWith("frontend/src/lib/client/"))) {
      add(file, `shared lib helper must not import ${dep}`);
    }

    // 5. Adapters must stay decoupled from client API wrappers, facade, and UI layers.
    if (isAdapter) {
      if (dep.startsWith("frontend/src/lib/client/") || dep === "frontend/src/lib/bridge.ts" || dep.startsWith("frontend/src/views/") || dep.startsWith("frontend/src/components/") || dep.startsWith("frontend/src/composables/")) {
        add(file, `adapter module must not depend upward on ${dep}`);
      }
      const myPlatform = adapterPlatformOf(file);
      const depPlatform = adapterPlatformOf(dep);
      if (myPlatform && depPlatform && myPlatform !== depPlatform) {
        add(file, `cross-adapter import forbidden: ${myPlatform} adapter imported ${depPlatform} (${dep})`);
      }
    }

    // 6. Client layer must not reach upward into facade or UI layers.
    if (isClient && (dep === "frontend/src/lib/bridge.ts" || dep.startsWith("frontend/src/views/") || dep.startsWith("frontend/src/components/") || dep.startsWith("frontend/src/composables/"))) {
      add(file, `client module must not import upward module ${dep}`);
    }

    // 7. UI layers go through lib/bridge.ts (or pure bridge contracts).
    if (isUILayer && (dep.startsWith("frontend/src/lib/adapters/") || dep.startsWith("frontend/src/lib/client/"))) {
      add(file, `UI layer must import from lib/bridge.ts instead of ${dep}`);
    }
  }
}

// --- Pure contract reachability check ---------------------------------------
// Ensure modules transitively imported by frontend/src/lib/adapters/** never loop back to facade/client.
const visitedFromAdapters = new Set();
const stack = [...graph.keys()].filter((k) => k.startsWith("frontend/src/lib/adapters/"));
while (stack.length > 0) {
  const current = stack.pop();
  if (visitedFromAdapters.has(current)) continue;
  visitedFromAdapters.add(current);
  for (const dep of graph.get(current) ?? []) {
    if (dep === "frontend/src/lib/bridge.ts" || dep.startsWith("frontend/src/lib/client/")) {
      add(current, `adapter dependency graph reaches upward module ${dep}`);
    } else if (!visitedFromAdapters.has(dep)) {
      stack.push(dep);
    }
  }
}

// --- Zero import cycle check across frontend/src/lib/** ----------------------
const libNodes = [...graph.keys()].filter((k) => k.startsWith("frontend/src/lib/")).sort();
const color = new Map(); // 0/undefined=unvisited, 1=visiting, 2=done
const trail = [];
const reportedCycles = new Set();

function dfs(node) {
  color.set(node, 1);
  trail.push(node);
  for (const dep of graph.get(node) ?? []) {
    if (!dep.startsWith("frontend/src/lib/")) continue;
    const state = color.get(dep) ?? 0;
    if (state === 1) {
      const start = trail.indexOf(dep);
      const cycle = [...trail.slice(start), dep];
      const key = [...cycle].sort().join("|");
      if (!reportedCycles.has(key)) {
        reportedCycles.add(key);
        add(node, `import cycle detected: ${cycle.join(" -> ")}`);
      }
    } else if (state === 0) {
      dfs(dep);
    }
  }
  trail.pop();
  color.set(node, 2);
}

for (const node of libNodes) {
  if ((color.get(node) ?? 0) === 0) {
    dfs(node);
  }
}

if (errors.length > 0) {
  console.error("Frontend architecture check failed:");
  for (const { file, message } of errors) {
    console.error(`  ${file}: ${message}`);
  }
  process.exit(1);
}

console.log(`Frontend architecture check passed (${libNodes.length} lib modules verified, 0 cycles).`);
