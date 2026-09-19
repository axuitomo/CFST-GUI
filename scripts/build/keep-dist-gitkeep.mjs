#!/usr/bin/env node
// Vite 的 build.emptyOutDir 会在构建前清空 frontend/dist，连占位用的 .gitkeep 也会一起删掉。
// 仓库约定 frontend/dist「只跟踪 .gitkeep、构建产物一律忽略」（见根目录 .gitignore）：
// .gitkeep 让 //go:embed all:frontend/dist 在全新克隆、尚未构建前端时仍可编译。
// 因此每次 `pnpm build`（npm/pnpm 会在 build 后自动触发 postbuild）后补回空的 .gitkeep，
// 既保证工作区不出现“.gitkeep 被删除”的噪声，也让漏跑前端构建时得到的是明确报错而不是旧界面。
//
// 用相对本脚本的路径定位 dist，不依赖调用方 cwd：pnpm 在 frontend/ 下触发，CI 也可能从仓库根触发。
import { mkdirSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));
const distDir = join(here, "..", "..", "frontend", "dist");

mkdirSync(distDir, { recursive: true });
writeFileSync(join(distDir, ".gitkeep"), "");
