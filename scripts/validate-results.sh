#!/usr/bin/env bash
set -euo pipefail

# shellcheck source=scripts/lib/common.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/common.sh"

results_dir="cfst-results"
write_latest=0

usage() {
  cat <<'EOF'
usage: scripts/validate-results.sh [--dir <path>] [--write-latest]

Validates CFST CSV result files and optionally writes latest.json.
EOF
}

while (($# > 0)); do
  case "$1" in
    --dir)
      results_dir="${2:-}"
      shift
      ;;
    --write-latest)
      write_latest=1
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      printf 'unknown option: %s\n' "$1" >&2
      usage >&2
      exit 2
      ;;
  esac
  shift
done

node - "$ROOT_DIR" "$results_dir" "$write_latest" <<'NODE'
const fs = require("fs");
const path = require("path");

const root = process.argv[2];
const inputDir = process.argv[3];
const writeLatest = process.argv[4] === "1";
const targetDir = path.resolve(root, inputDir);

function walk(dir) {
  if (!fs.existsSync(dir)) return [];
  const entries = fs.readdirSync(dir, { withFileTypes: true });
  const files = [];
  for (const entry of entries) {
    const full = path.join(dir, entry.name);
    if (entry.isDirectory()) {
      const rel = path.relative(root, full).split(path.sep).join("/");
      if (
        rel === ".git" ||
        rel === "build" ||
        rel === ".android-toolchain" ||
        rel === "frontend/node_modules" ||
        rel === "frontend/dist" ||
        rel === "mobile/android/.gradle" ||
        rel === "mobile/android/build" ||
        rel === "mobile/android/app/build"
      ) {
        continue;
      }
      files.push(...walk(full));
    }
    else if (entry.isFile() && entry.name.endsWith(".csv")) files.push(full);
  }
  return files;
}

function parseCSVLine(line) {
  const result = [];
  let current = "";
  let quoted = false;
  for (let i = 0; i < line.length; i++) {
    const ch = line[i];
    if (ch === '"') {
      if (quoted && line[i + 1] === '"') {
        current += '"';
        i++;
      } else {
        quoted = !quoted;
      }
      continue;
    }
    if (ch === "," && !quoted) {
      result.push(current);
      current = "";
      continue;
    }
    current += ch;
  }
  result.push(current);
  return result;
}

const files = walk(targetDir);
if (files.length === 0) {
  console.error(`No CSV files found under ${inputDir}/`);
  process.exit(1);
}

const requiredHeaders = ["IP 地址", "TCP延迟(ms)", "平均速率(MB/s)", "地区码"];
const validated = [];

for (const file of files) {
  const raw = fs.readFileSync(file, "utf8").replace(/^\uFEFF/, "");
  if (!raw.trim()) {
    console.error(`CSV file is empty: ${path.relative(root, file)}`);
    process.exit(1);
  }
  const lines = raw.split(/\r?\n/).filter((line) => line.trim().length > 0);
  const header = parseCSVLine(lines[0]);
  for (const required of requiredHeaders) {
    if (!header.includes(required)) {
      console.error(`CSV file missing required header "${required}": ${path.relative(root, file)}`);
      process.exit(1);
    }
  }
  validated.push({ file, header, lines, relative: path.relative(root, file).split(path.sep).join("/") });
}

validated.sort((a, b) => b.relative.localeCompare(a.relative));
const latest = validated[0];
const headerIndex = Object.fromEntries(latest.header.map((value, index) => [value, index]));
const rows = latest.lines.slice(1).map(parseCSVLine).filter((row) => row[0] && row[0].trim());
const topRows = rows.slice(0, 10).map((row) => ({
  ip: row[headerIndex["IP 地址"]] || "",
  colo: row[headerIndex["地区码"]] || "",
  tcp_latency_ms: row[headerIndex["TCP延迟(ms)"]] || "",
  download_mbps: row[headerIndex["平均速率(MB/s)"]] || "",
  trace_latency_ms: headerIndex["追踪延迟(ms)"] !== undefined ? row[headerIndex["追踪延迟(ms)"]] || "" : "",
}));

console.log(`Validated ${validated.length} CSV file(s).`);
console.log(`Latest CSV: ${latest.relative}`);
console.log(`Rows: ${rows.length}`);

if (writeLatest) {
  const output = {
    path: latest.relative,
    generated_at: new Date().toISOString(),
    row_count: rows.length,
    top_rows: topRows,
  };
  fs.writeFileSync(path.join(targetDir, "latest.json"), JSON.stringify(output, null, 2) + "\n");
  console.log(`Wrote ${path.relative(root, path.join(targetDir, "latest.json"))}`);
}
NODE
