#!/usr/bin/env node
// 释放 Vite 开发端口，避免上一次 `wails3 dev` 留下的孤儿 vite 把端口占死。
//
// 为什么需要它：`dev_mode.executes` 里的 `pnpm --dir frontend dev` 是 background 类型，
// 收尾交给 refresh 引擎的 `taskkill /T`。但当 wails3 本身被强杀（关终端、IDE 停任务、
// SIGTERM）时收尾不会执行，vite 会变成孤儿并继续监听端口。而 `wails3 dev` 在启动前会先
// 探测端口（wails v3 internal/commands/dev.go 的 net.Listen 预检），占着就直接报错退出，
// 表现是「什么都没发生、窗口还是旧的」。本脚本作为 executes 的第一步（type: once）清场。
//
// 只处理「正在监听目标端口的 node 进程」，命令行不含 vite 或进程名不是 node 时只打印提示，
// 绝不误杀。端口取 WAILS_VITE_PORT（wails3 在跑 executes 之前已导出），回退 9245。
//
// 用 .mjs 而不是 .sh/.ps1：executes 的 cmd 在 Windows 走 cmd.exe、在 Unix 走 /bin/sh，
// 同一份命令字符串要在两边都能跑，Node 是仓库已有的跨平台依赖（前端工具链）。
//
// 退出码恒为 0：清不掉时让 wails3 自己报端口错误，比在这里中断更有信息量。
//
// 输出一律用英文：仓库里 scripts/ 下所有脚本的 echo/printf 都是英文（注释才是中文），
// 而且这里的输出会经过 `wails3 dev` 的 refresh 日志管道，该管道按系统代码页解码，
// 写中文会变成乱码（实测 [free-dev-port] 的中文输出在日志里不可读）。

import { spawnSync } from "node:child_process";

const port = Number(process.env.WAILS_VITE_PORT) || 9245;
const isWindows = process.platform === "win32";

const log = (message) => process.stdout.write(`[free-dev-port] ${message}\n`);

function run(command, args) {
  const result = spawnSync(command, args, { encoding: "utf8" });
  if (result.error || result.status !== 0) {
    return "";
  }
  return result.stdout ?? "";
}

// 列出监听目标端口的进程号，去重。
function listeningPids() {
  if (isWindows) {
    const suffix = `:${port}`;
    const pids = new Set();
    for (const line of run("netstat", ["-ano", "-p", "tcp"]).split("\n")) {
      const fields = line.trim().split(/\s+/);
      if (fields.length < 5) continue;
      if (!fields[1].endsWith(suffix)) continue;
      if (fields[3] !== "LISTENING") continue;
      const pid = Number(fields[4]);
      if (Number.isInteger(pid) && pid > 0) pids.add(pid);
    }
    return [...pids];
  }

  const fromLsof = run("lsof", ["-ti", `tcp:${port}`, "-sTCP:LISTEN"]);
  if (fromLsof.trim()) {
    return [...new Set(fromLsof.trim().split(/\s+/).map(Number).filter(Number.isInteger))];
  }

  const pids = new Set();
  for (const match of run("ss", ["-lptnH", `sport = :${port}`]).matchAll(/pid=(\d+)/g)) {
    pids.add(Number(match[1]));
  }
  return [...pids];
}

function describe(pid) {
  if (isWindows) {
    const csv = run("tasklist", ["/FI", `PID eq ${pid}`, "/FO", "CSV", "/NH"]).trim();
    const name = (csv.split(",")[0] ?? "").replaceAll('"', "");
    let commandLine = run("pwsh", [
      "-NoProfile",
      "-Command",
      `(Get-CimInstance Win32_Process -Filter 'ProcessId=${pid}').CommandLine`,
    ]).trim();
    if (!commandLine) {
      commandLine = run("powershell", [
        "-NoProfile",
        "-Command",
        `(Get-CimInstance Win32_Process -Filter 'ProcessId=${pid}').CommandLine`,
      ]).trim();
    }
    return { name, commandLine };
  }

  return {
    name: run("ps", ["-o", "comm=", "-p", String(pid)]).trim(),
    commandLine: run("ps", ["-o", "args=", "-p", String(pid)]).trim(),
  };
}

function kill(pid) {
  if (isWindows) {
    const result = spawnSync("taskkill", ["/PID", String(pid), "/T", "/F"], { encoding: "utf8" });
    return result.status === 0;
  }

  try {
    process.kill(pid, "SIGTERM");
  } catch {
    return false;
  }
  const deadline = Date.now() + 1500;
  while (Date.now() < deadline) {
    try {
      process.kill(pid, 0);
    } catch {
      return true;
    }
    // 同步等待，避免为了一个 sleep 引入额外的进程或异步主流程。
    Atomics.wait(new Int32Array(new SharedArrayBuffer(4)), 0, 0, 100);
  }
  try {
    process.kill(pid, "SIGKILL");
    return true;
  } catch {
    return false;
  }
}

function warnIfDevBinaryIsRunning() {
  if (!isWindows) return;
  const listing = run("tasklist", ["/FI", "IMAGENAME eq cfst-gui-dev.exe", "/FO", "CSV", "/NH"]);
  if (!listing.toLowerCase().includes("cfst-gui-dev.exe")) return;
  log("note: cfst-gui-dev.exe is still running and holds the same file under build/bin,");
  log("      so `go build -o build/bin/cfst-gui-dev.exe` may fail with Access is denied;");
  log("      close that window when you no longer need it, or use a different output name.");
}

const stale = listeningPids();
if (stale.length === 0) {
  log(`port ${port} is free`);
  warnIfDevBinaryIsRunning();
  process.exit(0);
}

for (const pid of stale) {
  const { name, commandLine } = describe(pid);
  if (name && !/^node(\.exe)?$/i.test(name)) {
    log(`port ${port} is held by ${name} (PID ${pid}), which is not node; skipping, handle it manually.`);
    continue;
  }
  if (commandLine && !commandLine.includes("vite")) {
    log(`PID ${pid} command line has no "vite"; skipping, handle it manually: ${commandLine}`);
    continue;
  }
  if (!commandLine) {
    log(`PID ${pid} is node but its command line is unreadable; treating it as a stale dev server.`);
  }

  log(`killing stale dev server: PID ${pid}${commandLine ? ` (${commandLine})` : ""}`);
  if (!kill(pid)) {
    log(`failed to kill PID ${pid}; handle it manually.`);
  }
}

const remaining = listeningPids();
if (remaining.length === 0) {
  log(`port ${port} released`);
} else {
  log(`port ${port} is still held by PID ${remaining.join(", ")}; wails3 dev will likely fail.`);
}

warnIfDevBinaryIsRunning();
process.exit(0);
