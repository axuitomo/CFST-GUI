#!/usr/bin/env node
// Full-tree Vapor Mode guard.
//
// The frontend runs entirely on Vue 3.6 Vapor Mode. This invariant is easy to
// regress (a new SFC without `vapor`, a Vapor-unsupported construct, the Vite
// alias/bootstrap being edited away, or an unpinned RC). This single,
// dependency-free Node check enforces it so the rule cannot silently drift:
//   - every own SFC is a `<script setup vapor ...>` component (no Options API);
//   - no Vapor-unsupported constructs (string-form `<component :is>`, h()/JSX,
//     v-memo, getCurrentInstance, globalProperties, VDOM render APIs);
//   - the Vite config keeps `features.vapor`, and the bare `vue` entry that the
//     Vapor compiler emits imports from still exports the Vapor APIs;
//   - main.ts bootstraps createVaporApp with vaporInteropPlugin;
//   - vue / @vitejs/plugin-vue / vue-tsc are pinned to exact versions, and the
//     redundant Tailwind-v3-era PostCSS/autoprefixer setup stays removed.
//
// Invoked by .githooks/pre-commit, scripts/checks/check.{sh,ps1}, the
// `pnpm check:vapor` root script, and CI. Self-contained: it locates the repo
// root from its own path, so it runs from any working directory.

import { existsSync, readFileSync, readdirSync, statSync } from "node:fs";
import { dirname, join, relative, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));
const root = resolve(here, "..", "..");
const frontendDir = join(root, "packages", "frontend");
const srcDir = join(frontendDir, "src");

const errors = [];
const rel = (p) => relative(root, p).split("\\").join("/");
const lineAt = (content, index) => content.slice(0, Math.max(0, index)).split("\n").length;
const add = (file, line, message) => errors.push({ file, line, message });

function walk(dir, cb) {
  if (!existsSync(dir)) return;
  for (const name of readdirSync(dir)) {
    if (name === "node_modules" || name === "dist" || name.startsWith(".")) continue;
    const p = join(dir, name);
    if (statSync(p).isDirectory()) walk(p, cb);
    else cb(p);
  }
}

const vueFiles = [];
const codeFiles = [];
walk(srcDir, (p) => {
  if (p.endsWith(".vue")) vueFiles.push(p);
  if (/\.(vue|ts|js)$/.test(p)) codeFiles.push(p);
});

// --- 1. Every SFC must be a vapor <script setup>; reject Options API --------
const scriptOpen = /<script\b([^>]*)>/gi;
for (const file of vueFiles) {
  const content = readFileSync(file, "utf8");
  let setupBlocks = 0;
  for (const m of content.matchAll(scriptOpen)) {
    const attrs = m[1] || "";
    if (/\bsetup\b/.test(attrs)) {
      setupBlocks += 1;
      if (!/\bvapor\b/.test(attrs)) {
        add(
          rel(file),
          lineAt(content, m.index),
          '<script setup> must opt into Vapor: use <script setup vapor lang="ts"> (full-tree Vapor Mode).',
        );
      }
    } else {
      // A plain (non-setup) <script> block carrying `export default` is Options API.
      const rest = content.slice(m.index + m[0].length);
      const stop = /<\/script\s*>/i.exec(rest);
      const body = stop ? rest.slice(0, stop.index) : rest;
      const exp = /export\s+default\b/.exec(body);
      if (exp) {
        add(
          rel(file),
          lineAt(content, m.index + m[0].length + exp.index),
          'Vapor does not support Options API (`export default` in <script>); use <script setup vapor>.',
        );
      }
    }
  }
  if (setupBlocks === 0) {
    add(
      rel(file),
      1,
      'Component has no <script setup> block; full-tree Vapor requires <script setup vapor lang="ts">.',
    );
  }
}

// --- 2. Forbidden Vapor-unsupported constructs -----------------------------
const forbidden = [
  {
    re: /\bv-memo\b/,
    exts: [".vue"],
    message: "v-memo is not supported in Vapor.",
  },
  {
    re: /\bgetCurrentInstance\b/,
    exts: [".vue", ".ts"],
    message: "getCurrentInstance() is not supported in Vapor.",
  },
  {
    re: /\.globalProperties\b/,
    exts: [".vue", ".ts"],
    message: "app.config.globalProperties is not supported in Vapor; use provide/inject.",
  },
  {
    // :is="'Name'" / :is='"Name"' — a string literal rather than a component object.
    re: /:is\s*=\s*"\s*'|:is\s*=\s*'\s*"/,
    exts: [".vue"],
    message: 'String-form <component :is="\'X\'"> is not supported in Vapor; bind an imported component object.',
  },
  {
    re: /import\s*\{[^}]*\b(h|createVNode|createBlock|render)\b[^}]*\}\s*from\s*["']vue["']/,
    exts: [".vue", ".ts"],
    message: 'VDOM render APIs (h/createVNode/createBlock/render) imported from "vue" are not supported in Vapor; use templates.',
  },
];

for (const file of codeFiles) {
  const ext = file.slice(file.lastIndexOf("."));
  const content = readFileSync(file, "utf8");
  for (const rule of forbidden) {
    if (!rule.exts.includes(ext)) continue;
    for (const m of content.matchAll(new RegExp(rule.re.source, rule.re.flags.includes("g") ? rule.re.flags : rule.re.flags + "g"))) {
      add(rel(file), lineAt(content, m.index), rule.message);
    }
  }
}

// JSX/TSX render functions are not supported.
walk(srcDir, (p) => {
  if (/\.(tsx|jsx)$/.test(p)) {
    add(rel(p), 1, "JSX/TSX render functions are not supported in Vapor; author templates in .vue SFCs.");
  }
});

// --- 3. Build/bootstrap configuration guards --------------------------------
function requirePattern(file, re, message) {
  if (!existsSync(file)) {
    add(rel(file), 1, `missing file; ${message}`);
    return;
  }
  const content = readFileSync(file, "utf8");
  if (!re.test(content)) add(rel(file), 1, message);
}

const viteConfigPath = join(frontendDir, "vite.config.ts");
requirePattern(
  viteConfigPath,
  /features\s*:[\s\S]*?\bvapor\s*:\s*true/,
  "vite.config must compile SFCs with the Vue plugin features.vapor = true.",
);
// Vapor SFC 的编译产物从裸 `vue` 导入 createVaporApp / defineVaporComponent 等 API。
// vue 3.6.0-rc.13 起默认入口（runtime.esm-bundler）才同时再导出 @vue/runtime-vapor，
// 更早的 RC 只有预构建的 runtime-with-vapor 包含它。这里直接读入口产物，比匹配 vite
// 别名更贴近真实不变量：别名可以指向任何文件，而 `vue` 入口是不是 Vapor-capable 才是
// 「升级/降级后 Vapor 还能不能跑」的判据。
const vueEntryPath = join(frontendDir, "node_modules", "vue", "dist", "vue.runtime.esm-bundler.js");
if (existsSync(vueEntryPath)) {
  if (!/@vue\/runtime-vapor/.test(readFileSync(vueEntryPath, "utf8"))) {
    add(
      rel(vueEntryPath),
      1,
      'the bare "vue" entry must re-export the Vapor APIs; upgrade vue to >= 3.6.0-rc.13 (or restore a runtime-with-vapor alias in vite.config.ts).',
    );
  }
} else if (existsSync(viteConfigPath) && !/runtime-with-vapor/.test(readFileSync(viteConfigPath, "utf8"))) {
  add(
    rel(viteConfigPath),
    1,
    'no installed vue entry to verify and no runtime-with-vapor alias in vite.config; Vapor APIs would be undefined at runtime.',
  );
}

const mainPath = join(srcDir, "main.ts");
requirePattern(mainPath, /createVaporApp/, "main.ts must bootstrap the app with createVaporApp().");
requirePattern(
  mainPath,
  /vaporInteropPlugin/,
  "main.ts must install vaporInteropPlugin so third-party VDOM components (e.g. Phosphor icons) render.",
);
if (existsSync(mainPath)) {
  const main = readFileSync(mainPath, "utf8");
  const vdom = /(^|[^A-Za-z])createApp\s*\(/.exec(main);
  if (vdom) add(rel(mainPath), lineAt(main, vdom.index), "main.ts must not use createApp() in full-tree Vapor; use createVaporApp().");
}

// --- 4. Pinned versions and removed legacy CSS tooling ----------------------
const pkgPath = join(frontendDir, "package.json");
if (existsSync(pkgPath)) {
  const pkg = JSON.parse(readFileSync(pkgPath, "utf8"));
  const exact = (spec) => /^\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$/.test(spec);

  const vueSpec = pkg.dependencies?.vue;
  if (!vueSpec) {
    add("packages/frontend/package.json", 1, "missing the vue dependency.");
  } else if (!exact(vueSpec)) {
    add("packages/frontend/package.json", 1, `vue must be pinned to an exact version (got "${vueSpec}").`);
  } else {
    const m = vueSpec.match(/^(\d+)\.(\d+)\.(\d+)/);
    const major = Number(m[1]);
    const minor = Number(m[2]);
    if (major < 3 || (major === 3 && minor < 6)) {
      add("packages/frontend/package.json", 1, `vue must be >= 3.6 for Vapor Mode (got "${vueSpec}").`);
    }
  }

  for (const tool of ["@vitejs/plugin-vue", "vue-tsc"]) {
    const spec = pkg.devDependencies?.[tool];
    if (spec && !exact(spec)) {
      add("packages/frontend/package.json", 1, `${tool} must be pinned to an exact version for reproducible Vapor builds (got "${spec}").`);
    }
  }

  if (pkg.dependencies?.autoprefixer || pkg.devDependencies?.autoprefixer) {
    add("packages/frontend/package.json", 1, "autoprefixer is redundant with Tailwind v4's @tailwindcss/vite (Lightning CSS); remove it.");
  }
}

if (existsSync(frontendDir)) {
  for (const name of readdirSync(frontendDir)) {
    if (/^postcss\.config\./.test(name)) {
      add(rel(join(frontendDir, name)), 1, "PostCSS config is redundant with the Tailwind v4 Vite plugin; remove it.");
    }
  }
}

// --- Report -----------------------------------------------------------------
if (errors.length > 0) {
  errors.sort((a, b) => (a.file === b.file ? a.line - b.line : a.file.localeCompare(b.file)));
  console.error(`Vapor mode check failed: ${errors.length} violation(s) of the full-tree Vapor invariant.\n`);
  for (const e of errors) console.error(`  ${e.file}:${e.line}: ${e.message}`);
  console.error(
    "\nSee docs/dev/agent-workflow-validation.md (Frontend Vapor Mode) and packages/frontend/vite.config.ts.",
  );
  process.exit(1);
}

console.log(
  `Vapor mode check passed: ${vueFiles.length} SFC(s) all compiled for Vapor, build/bootstrap config and pins intact.`,
);
