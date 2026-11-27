#!/usr/bin/env node
// No-string-code guard (docs/dev/refactor-plan.md §3.6).
//
// 「字符串里的代码」防回流：HTML/CSS/JS/配置文件必须是真实文件或模板+工具组装，
// 不得以字符串拼装。正则/词法级不变量扫描（不是 AST 工具/代码生成器），仿照
// vapor-mode.mjs 的自包含风格：从自身路径定位仓库根，任意工作目录可运行。
//
// 三个范围：
//   1. packages/frontend/src/**（.ts 全文、.vue 仅 <script> 块、.js 全文）：
//      禁 innerHTML / outerHTML= / insertAdjacentHTML / document.write /
//      cssText / insertRule / new Function / eval(；禁模板字面量文本中出现
//      完整 HTML 标签起始。注释与普通字符串内容被掩码；模板里的 ${...} 插值
//      被掩码，合法 Vue 插值（如 `scaleX(${x})`）与 < 比较不会误伤。
//   2. 非测试 *.go：禁 raw string 含 <!DOCTYPE|<html|<div|<style|<script；
//      禁 Sprintf 拼 JSON 对象起始（{" 或 {%q）。mpb 进度条模板 {{...}} 不是
//      JSON，不在禁列。
//   3. scripts/build/*.sh：禁 heredoc 写入配置文件（Dockerfile/compose/.env/
//      .conf/.nsi/.yml/.yaml/.toml/.ini/.properties/.service 等）；usage 帮助
//      heredoc（cat <<EOF 输出到 stdout）放行。
//
// 豁免登记（refactor-plan §3.6）：
//   - scripts/build/version-bump.sh 的 notes heredoc 写发布 notes（.md，非配置）。
//   - Kotlin/mobileapi 当前 0 命中，暂不纳入守卫（边界写明，回流时再扩）。
//
// 接线：根 package.json check:stringcode、.githooks/pre-commit（变更文件触发）、
// scripts/checks/check.{sh,ps1}、scripts/checks/fast.{ps1,sh}、CI Quality。

import { existsSync, readFileSync, readdirSync, statSync } from "node:fs";
import { dirname, join, relative, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));
const root = resolve(here, "..", "..");

const errors = [];
const rel = (p) => relative(root, p).split("\\").join("/");
const lineAt = (content, index) => content.slice(0, Math.max(0, index)).split("\n").length;
const add = (file, line, message) => errors.push({ file, line, message });

const SKIP_DIRS = new Set(["node_modules", "dist", "build", ".git", "bindings", ".pnpm-store"]);
function walk(dir, cb) {
  if (!existsSync(dir)) return;
  for (const name of readdirSync(dir)) {
    if (name.startsWith(".") || SKIP_DIRS.has(name)) continue;
    const p = join(dir, name);
    if (statSync(p).isDirectory()) walk(p, cb);
    else cb(p);
  }
}

// --- 词法掩码器 -------------------------------------------------------------
// 把注释与字符串内容替换为等长空格（保留换行与引号定界符），返回掩码后的源码
// 与模板字面量区域。模板区域里的 ${...} 插值同样掩码（括号配平，递归处理嵌套
// 模板/字符串），只留下纯文本供 HTML 标签规则扫描。不是 AST：只做区域识别。
function lexJs(source) {
  const masked = source.split("");
  const templates = [];
  const blank = (from, to) => {
    for (let i = from; i < to && i < masked.length; i++) {
      if (masked[i] !== "\n") masked[i] = " ";
    }
  };
  let i = 0;
  const n = source.length;
  const scanTemplate = (start) => {
    // start 指向反引号；返回结束反引号之后的位置。
    let j = start + 1;
    let textStart = j;
    const textParts = [];
    while (j < n) {
      const c = source[j];
      if (c === "\\") {
        j += 2;
        continue;
      }
      if (c === "`") {
        textParts.push(source.slice(textStart, j));
        blank(start + 1, j);
        templates.push({ start, end: j, text: textParts.join(" ") });
        return j + 1;
      }
      if (c === "$" && source[j + 1] === "{") {
        textParts.push(source.slice(textStart, j));
        blank(j, j + 2);
        let depth = 1;
        let k = j + 2;
        const exprStart = k;
        while (k < n && depth > 0) {
          const ch = source[k];
          if (ch === "\\") {
            k += 2;
            continue;
          }
          if (ch === "{") depth++;
          else if (ch === "}") depth--;
          else if (ch === "`") {
            k = scanTemplate(k);
            continue;
          } else if (ch === '"' || ch === "'") {
            const quote = ch;
            k++;
            while (k < n && source[k] !== quote) {
              if (source[k] === "\\") k++;
              k++;
            }
          }
          k++;
        }
        blank(exprStart, Math.max(exprStart, k - 1));
        j = k;
        textStart = j;
        continue;
      }
      j++;
    }
    textParts.push(source.slice(textStart));
    blank(start + 1, n);
    templates.push({ start, end: n, text: textParts.join(" ") });
    return n;
  };
  while (i < n) {
    const c = source[i];
    if (c === "/" && source[i + 1] === "/") {
      let j = i;
      while (j < n && source[j] !== "\n") j++;
      blank(i, j);
      i = j;
      continue;
    }
    if (c === "/" && source[i + 1] === "*") {
      let j = i + 2;
      while (j < n && !(source[j] === "*" && source[j + 1] === "/")) j++;
      j = Math.min(n, j + 2);
      blank(i, j);
      i = j;
      continue;
    }
    if (c === '"' || c === "'") {
      let j = i + 1;
      while (j < n && source[j] !== c) {
        if (source[j] === "\\") j++;
        j++;
      }
      j = Math.min(n, j + 1);
      blank(i + 1, j - 1);
      i = j;
      continue;
    }
    if (c === "`") {
      i = scanTemplate(i);
      continue;
    }
    i++;
  }
  return { masked: masked.join(""), templates };
}

// Go 词法掩码：只掩码注释（字符串内容保留，Sprintf 规则要看格式串起始）；
// raw string（反引号）区域单独收集供 HTML 规则扫描。
function lexGo(source) {
  const masked = source.split("");
  const raws = [];
  const blank = (from, to) => {
    for (let i = from; i < to && i < masked.length; i++) {
      if (masked[i] !== "\n") masked[i] = " ";
    }
  };
  let i = 0;
  const n = source.length;
  while (i < n) {
    const c = source[i];
    if (c === "/" && source[i + 1] === "/") {
      let j = i;
      while (j < n && source[j] !== "\n") j++;
      blank(i, j);
      i = j;
      continue;
    }
    if (c === "/" && source[i + 1] === "*") {
      let j = i + 2;
      while (j < n && !(source[j] === "*" && source[j + 1] === "/")) j++;
      j = Math.min(n, j + 2);
      blank(i, j);
      i = j;
      continue;
    }
    if (c === "`") {
      let j = i + 1;
      while (j < n && source[j] !== "`") j++;
      raws.push({ start: i, end: Math.min(n, j + 1), text: source.slice(i + 1, j) });
      i = j + 1;
      continue;
    }
    if (c === '"' || c === "'") {
      let j = i + 1;
      while (j < n && source[j] !== c) {
        if (source[j] === "\\") j++;
        j++;
      }
      i = Math.min(n, j + 1);
      continue;
    }
    i++;
  }
  return { masked: masked.join(""), raws };
}

// --- 范围 1：packages/frontend/src ------------------------------------------
const FRONTEND_APIS = [
  { re: /\binnerHTML\b/g, message: "innerHTML 禁用：markup 必须是真实 HTML/模板，不得字符串拼装。" },
  { re: /\bouterHTML\s*=/g, message: "outerHTML 赋值禁用：markup 必须是真实 HTML/模板。" },
  { re: /\binsertAdjacentHTML\b/g, message: "insertAdjacentHTML 禁用：markup 必须是真实 HTML/模板。" },
  { re: /\bdocument\s*\.\s*write\s*\(/g, message: "document.write 禁用。" },
  { re: /\bcssText\b/g, message: "style.cssText 禁用：样式走 class/真实 CSS 文件。" },
  { re: /\binsertRule\b/g, message: "insertRule 禁用：样式走真实 CSS 文件。" },
  { re: /\bnew\s+Function\s*\(/g, message: "new Function 禁用：代码不得在字符串里。" },
  { re: /\beval\s*\(/g, message: "eval 禁用：代码不得在字符串里。" },
];
const HTML_TAG = /<\/?[a-zA-Z][\w-]*(?:\s|>|\/)/g;

const srcDir = join(root, "packages", "frontend", "src");
let frontendFiles = 0;
const frontendTargets = [];
walk(srcDir, (p) => {
  if (/\.(ts|js)$/.test(p)) {
    frontendTargets.push({ file: p, content: readFileSync(p, "utf8"), lineBase: 0 });
  } else if (p.endsWith(".vue")) {
    const content = readFileSync(p, "utf8");
    for (const m of content.matchAll(/<script\b[^>]*>([\s\S]*?)<\/script\b[^>]*>/gi)) {
      const bodyStart = m.index + m[0].indexOf(m[1]);
      frontendTargets.push({ file: p, content: m[1], lineBase: content.slice(0, bodyStart).split("\n").length - 1 });
    }
  }
});
for (const { file, content, lineBase } of frontendTargets) {
  frontendFiles++;
  const { masked, templates } = lexJs(content);
  for (const rule of FRONTEND_APIS) {
    for (const m of masked.matchAll(rule.re)) {
      add(rel(file), lineBase + lineAt(content, m.index), rule.message);
    }
  }
  for (const tpl of templates) {
    for (const m of tpl.text.matchAll(HTML_TAG)) {
      add(rel(file), lineBase + lineAt(content, tpl.start), `模板字面量含 HTML 标签起始（${m[0].trim()}）：markup 写进真实 HTML/模板。`);
    }
  }
}

// --- 范围 2：非测试 *.go -----------------------------------------------------
const GO_RAW_HTML = /<!DOCTYPE|<html|<div|<style|<script/i;
const GO_SPRINTF_JSON = /\bSprintf\(\s*[`"]\s*\{\s*(?:["']|%q)/g;
let goFiles = 0;
walk(root, (p) => {
  if (!p.endsWith(".go") || p.endsWith("_test.go")) return;
  goFiles++;
  const content = readFileSync(p, "utf8");
  const { masked, raws } = lexGo(content);
  for (const raw of raws) {
    if (GO_RAW_HTML.test(raw.text)) {
      add(rel(p), lineAt(content, raw.start), "Go raw string 含 HTML markup：页面/片段走 embed 真实文件 + 模板工具组装。");
    }
  }
  for (const m of masked.matchAll(GO_SPRINTF_JSON)) {
    add(rel(p), lineAt(content, m.index), "Sprintf 拼 JSON 对象起始：用 encoding/json 等类型化序列化。");
  }
});

// --- 范围 3：scripts/build/*.sh ----------------------------------------------
const CONFIG_TARGET = /(^|\/)(Dockerfile[^/]*|docker-compose[^/]*|\.env[^/]*|[^/]*\.(conf|nsi|ya?ml|toml|ini|properties|service|env))$/i;
const HEREDOC_REDIRECT = /(?:cat|tee)\s+>>?\s*(?:"([^"]+)"|'([^']+)'|([^\s<]+))\s*<<-?\s*(['"]?)([A-Za-z_]\w*)\4/g;
const HEREDOC_THEN_REDIRECT = /(?:cat|tee)\s+<<-?\s*(['"]?)([A-Za-z_]\w*)\1\s*>>?\s*(?:"([^"]+)"|'([^']+)'|([^\s<]+))/g;
const EXEMPTIONS = [
  {
    file: "scripts/build/version-bump.sh",
    reason: "发布 notes heredoc 写 docs/release-notes/*.md（非配置文件），refactor-plan §3.6 登记放行。",
    match: (hit) => /notes/.test(hit.target),
  },
];
let shFiles = 0;
const buildDir = join(root, "scripts", "build");
if (existsSync(buildDir)) {
  for (const name of readdirSync(buildDir)) {
    if (!name.endsWith(".sh")) continue;
    shFiles++;
    const p = join(buildDir, name);
    const content = readFileSync(p, "utf8");
    const hits = [];
    for (const m of content.matchAll(HEREDOC_REDIRECT)) {
      hits.push({ target: m[1] ?? m[2] ?? m[3], index: m.index });
    }
    for (const m of content.matchAll(HEREDOC_THEN_REDIRECT)) {
      hits.push({ target: m[3] ?? m[4] ?? m[5], index: m.index });
    }
    for (const hit of hits) {
      const exempt = EXEMPTIONS.some((e) => e.file === rel(p) && e.match(hit));
      if (exempt) continue;
      const isVariable = hit.target.startsWith("$");
      if (isVariable || CONFIG_TARGET.test(hit.target)) {
        add(rel(p), lineAt(content, hit.index), `heredoc 写入配置目标「${hit.target}」：配置必须是 scripts/build/templates/ 下的真实文件 + 工具侧组装。`);
      }
    }
  }
}

// --- 报告 --------------------------------------------------------------------
if (errors.length > 0) {
  errors.sort((a, b) => (a.file === b.file ? a.line - b.line : a.file.localeCompare(b.file)));
  console.error(`No-string-code check failed: ${errors.length} violation(s).\n`);
  for (const e of errors) console.error(`  ${e.file}:${e.line}: ${e.message}`);
  console.error("\nSee docs/dev/refactor-plan.md §3.6（字符串代码歼灭与防回流守卫）.");
  process.exit(1);
}
console.log(`No-string-code check passed: ${frontendFiles} frontend file(s), ${goFiles} Go file(s), ${shFiles} build script(s) clean.`);