#!/usr/bin/env bash
set -euo pipefail

# 秒级报错闭环（fast.ps1 的 bash 孪生版，计划 docs/dev/refactor-plan.md §2.3）。
# 前端三件套直调 node 二进制并行执行；Go 侧 -short。全量门禁仍用 check.sh / CI。

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/../lib/common.sh"

cfst_require_cmd go
cfst_require_cmd node
cfst_require_cmd git

base=""
while (($# > 0)); do
  case "$1" in
    --base) base="${2:-}"; shift ;;
    -h|--help) printf 'usage: scripts/checks/fast.sh [--base <ref>]\n'; exit 0 ;;
    *) printf 'unknown option: %s\n' "$1" >&2; exit 2 ;;
  esac
  shift
done

declare -A timings=()
failures=()
total_start=$(date +%s%N)
tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT

now_ms() { echo $(( ($(date +%s%N) - $1) / 1000000 )); }

record() { timings["$1"]=$(now_ms "$2"); }

fail() {
  failures+=("$1")
  printf '\n[fast] FAILED: %s\n' "$1" >&2
  if [[ -n "${2:-}" ]]; then printf '%s\n' "$2" >&2; fi
}

run_go() {
  local name="$1"; shift
  local start; start=$(date +%s%N)
  local out code=0
  out="$(cd "$ROOT_DIR" && go "$@" 2>&1)" || code=$?
  record "$name" "$start"
  if ((code != 0)); then fail "$name" "$out"
  elif [[ "$out" == *"no tests to run"* ]]; then fail "$name" "boundary gate ran nothing: $out"
  else printf '[fast] %s ok (%sms)\n' "$name" "${timings[$name]}"
  fi
}

frontend_pids=()
frontend_names=()
frontend_start=0
boundary_pid=""
boundary_start=0

start_frontend() {
  if [[ ! -d "$FRONTEND_DIR/node_modules/vue-tsc" ]]; then
    printf 'frontend/node_modules missing: run pnpm install --frozen-lockfile in frontend first\n' >&2
    exit 1
  fi
  frontend_start=$(date +%s%N)
  (cd "$FRONTEND_DIR" && node node_modules/oxlint/bin/oxlint --deny-warnings src vite.config.ts capacitor.config.ts vitest.config.ts vitest.browser.config.ts >"$tmpdir/oxlint.out" 2>&1) &
  frontend_pids+=($!); frontend_names+=("oxlint")
  (cd "$FRONTEND_DIR" && node node_modules/vitest/vitest.mjs run >"$tmpdir/vitest.out" 2>&1) &
  frontend_pids+=($!); frontend_names+=("vitest")
  (cd "$FRONTEND_DIR" && node node_modules/vue-tsc/bin/vue-tsc.js --noEmit >"$tmpdir/vue-tsc.out" 2>&1) &
  frontend_pids+=($!); frontend_names+=("vue-tsc")
}

complete_frontend() {
  local i
  for i in "${!frontend_pids[@]}"; do
    local code=0
    wait "${frontend_pids[$i]}" || code=$?
    if ((code != 0)); then fail "${frontend_names[$i]}" "$(cat "$tmpdir/${frontend_names[$i]}.out")"
    else printf '[fast] %s ok\n' "${frontend_names[$i]}"
    fi
  done
  record "frontend(lint+test+tsc parallel)" "$frontend_start"

  local start; start=$(date +%s%N)
  local code=0
  (cd "$ROOT_DIR" && node scripts/checks/vapor-mode.mjs) || code=$?
  ((code == 0)) || fail "vapor-mode" "see output above"
  code=0
  bash "$ROOT_DIR/scripts/checks/frontend-boundary.sh" || code=$?
  ((code == 0)) || fail "frontend-boundary" "see output above"
  record "vapor+boundary" "$start"
}

start_boundary() {
  boundary_start=$(date +%s%N)
  (cd "$ROOT_DIR" && go test ./internal/appcore/ -count=1 -run \
    'TestSharedCorePackagesDoNotTransitivelyImportPlatform|TestSharedInternalPackagesDoNotImportPlatformAdapters|TestPlatformInvokeDoesNotReimplementSharedCommands' \
    >"$tmpdir/boundary.out" 2>&1) &
  boundary_pid=$!
}

complete_boundary() {
  local code=0
  wait "$boundary_pid" || code=$?
  record "appcore-boundary(parallel)" "$boundary_start"
  local out; out="$(cat "$tmpdir/boundary.out")"
  if ((code != 0)); then fail "appcore-boundary" "$out"
  elif [[ "$out" == *"no tests to run"* ]]; then fail "appcore-boundary" "boundary gate ran nothing: $out"
  else printf '[fast] appcore-boundary ok\n'
  fi
}

# ---- 变更集 ----
changed_files=()
if [[ -n "$base" ]]; then
  mapfile -t -O "${#changed_files[@]}" changed_files < <(git -C "$ROOT_DIR" diff --name-only --diff-filter=ACMR "$base"...HEAD)
fi
mapfile -t -O "${#changed_files[@]}" changed_files < <(
  {
    git -C "$ROOT_DIR" diff --name-only --diff-filter=ACMR HEAD
    git -C "$ROOT_DIR" ls-files --others --exclude-standard
  } | sort -u
)
mapfile -t changed_files < <(printf '%s\n' "${changed_files[@]}" | sort -u | sed '/^$/d')

has_go=0 has_frontend=0 has_scripts=0
go_all=0
for f in "${changed_files[@]}"; do
  case "$f" in
    go.mod|go.sum) has_go=1; go_all=1 ;;
    *.go) has_go=1 ;;
    frontend/*) has_frontend=1 ;;
    scripts/*.sh) has_scripts=1 ;;
  esac
done

if ((${#changed_files[@]} == 0)); then
  cfst_log "No changed files: minimal full sweep (go -short all + frontend trio)"
  start_frontend
  mapfile -t go_packages < <(cfst_go_packages)
  run_go "go-test-short(all)" test -short -count=1 "${go_packages[@]}"
  complete_frontend
else
  printf '[fast] changed files: %s\n' "${#changed_files[@]}"
  # 前端三件套先启动（非阻塞），与 Go 阶段完全并行。
  ((has_frontend)) && start_frontend
  if ((has_go)); then
    pkgs=()
    if ((go_all)); then
      mapfile -t pkgs < <(cfst_go_packages)
    else
      # 直接按目录映射包路径（省去 go list 全量扫描）；testdata 不是包，跳过。
      while IFS= read -r d; do
        [[ -z "$d" ]] && d="."
        [[ "/$d/" == */testdata/* ]] && continue
        if [[ "$d" == "." ]]; then pkgs+=("."); else pkgs+=("./$d"); fi
      done < <(printf '%s\n' "${changed_files[@]}" | grep '\.go$' | xargs -r -n1 dirname | sort -u)
      if ((${#pkgs[@]} == 0)); then cfst_warn "changed Go files map to no testable package; skipping go test"; fi
    fi
    if ((${#pkgs[@]} > 0)); then
      appcore_hit=0
      for p in "${pkgs[@]}"; do [[ "$p" == *internal/appcore* ]] && appcore_hit=1; done
      ((appcore_hit == 0)) && start_boundary
      run_go "go-test-short" test -short -count=1 "${pkgs[@]}"
      [[ -n "$boundary_pid" ]] && complete_boundary
    fi
  fi
  ((${#frontend_pids[@]} > 0)) && complete_frontend
  if ((has_scripts)); then
    start=$(date +%s%N)
    for s in "${changed_files[@]}"; do
      [[ "$s" == scripts/*.sh ]] && { bash -n "$ROOT_DIR/$s" || fail "bash -n $s" ""; }
    done
    record "bash -n" "$start"
  fi
fi

# ---- no-string-code 守卫（refactor-plan §3.6，词法级全仓扫描） ----
guard_start=$(date +%s%N)
guard_code=0
(cd "$ROOT_DIR" && node scripts/checks/no-string-code.mjs) || guard_code=$?
((guard_code == 0)) || fail "no-string-code" "see output above"
record "no-string-code" "$guard_start"

# ---- 拆分红线警告（不阻断） ----
for f in "${changed_files[@]}"; do
  case "$f" in
    *_test.go|*.test.ts|*.spec.ts|*.browser.test.ts) continue ;;
    *.go|*.ts|*.vue|*.css|*.kt) ;;
    *) continue ;;
  esac
  if [[ -f "$ROOT_DIR/$f" ]]; then
    n=$(wc -l < "$ROOT_DIR/$f")
    if ((n > 400)); then cfst_warn "$f: $n lines (>400 拆分红线)"; fi
  fi
done

# ---- 汇总 ----
printf '\n==> fast check timings\n'
for k in "${!timings[@]}"; do printf '%-34s %6sms\n' "$k" "${timings[$k]}"; done
printf '%-34s %6sms\n' "TOTAL" "$(now_ms "$total_start")"
if ((${#failures[@]} > 0)); then
  printf '\n[fast] FAILED steps: %s\n' "${failures[*]}" >&2
  exit 1
fi
printf '[fast] all checks passed\n'