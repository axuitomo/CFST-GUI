# CFST-GUI

[![Go Version](https://img.shields.io/github/go-mod/go-version/axuitomo/CFST-GUI?style=flat-square&label=Go&color=00ADD8&logo=go)](go.mod)
[![Wails](https://img.shields.io/badge/Wails-v3.2.0--nightly.20261103-f36f45?style=flat-square)](https://wails.io/)
[![License](https://img.shields.io/github/license/axuitomo/CFST-GUI?style=flat-square&label=License)](LICENSE)
[![Telegram](https://img.shields.io/badge/Telegram-Channel%20DM-26A5E4?style=flat-square&logo=telegram&logoColor=white)](https://t.me/axuitomo_pindao?direct)

[简体中文](README.md) | **English**

CFST-GUI is a Cloudflare/CDN IP speed test tool built on Wails + Vue + Capacitor, offering a visual task panel, input source management, result export, config sync, DNS record reading, DNS platform self-check, and automatic pushing.

One Vue frontend and one Go shared kernel (`internal/kernel/appcore`, `internal/kernel/domain/...`, `internal/kernel/infra/...`) serve three forms via platform adapters: the Wails desktop GUI, the Linux WebUI (HTTP service), and the Android app.

<img width="2560" height="1599" alt="CFST-GUI main window" src="https://github.com/user-attachments/assets/64bfa564-391a-47de-aca8-62d3afa57e3e" />

## Current State

- Desktop: Wails v3.2.0-nightly.20261103 + Go 1.27.4, native desktop GUI by default
- Frontend: Vue 3 + Vite 8 + Tailwind CSS 4 + TypeScript (`vue-tsc` plus standalone `tsc`) + Phosphor Icons
- Android: Capacitor 8.6.2 WebView + Kotlin Plugin (foreground service, WorkManager, SAF, permissions, update install) + gomobile AAR; release baseline JDK 25, AGP 9.5.1, Gradle 9.9.1, SDK/target 37, Build Tools 37.0.0, NDK 30.0.16248370
- Release artifacts cover Windows and Android only, all written to `build/artifacts/release/`; GitHub Releases publish no Linux, Docker, macOS, or iOS assets

## Features

| Capability | Description |
| --- | --- |
| Desktop speed test | Fixed 4 stages: IP pool → TCP latency → trace probe → file speed test. Fast mode runs only the first three stages; full mode adds the file speed test. Supports a packet-loss ceiling (default 15%, maximum 100%), region-code and download-speed threshold filtering; the result page supports sorting, paging, and IPv4/IPv6 filtering. |
| Task control | Running or paused tasks can be explicitly terminated; termination interrupts input source loading, network probing, retry waits, and post-test pushing, ending in a "terminated" state that counts as neither failure nor completion. Pause covers input source preparation, probing, retry/cooldown, CSV export, pushing, and result persistence; non-rollbackable external writes finish the current safe step first. |
| Input sources | Remote URLs, local files, or manual input. Lines are cleaned per line, skipping empty lines and `#` comments, then parsed as IP/CIDR or domains (domains resolve through the system local DNS). Candidate handling supports `traverse` and MICS sampling, with the total candidate count bounded by `probe.source_pool_limit` (default 65536). Desktop uses the system file dialog, Android uses the SAF picker (imported files are copied into the app private directory; persistent grants are used only for the export directory). |
| Probe strategy | The settings page folds the three stages and MICS sampling into a four-row summary; expanding it edits concurrency, timeouts, accepted status codes, region-code allow/deny lists, test duration, and rate basis. Units sit inline at the right of each input, `0` renders as "unlimited", conflicting ranges get a red outline, and a stage-1 timeout below the latency ceiling gets a yellow warning with one-click sync. Full definitions live in the [configuration reference](docs/guide/configuration.md) and [behavior baseline](docs/dev/behavior-baseline.md). |
| Results and export | CSV export defaults to `result.csv`; desktop/WebUI write to the export directory, Android writes to the persistently granted SAF export directory. The file speed test uses 4 HTTP Range GET shards by default and falls back to a full streaming GET when the server does not support Range. |
| DNS | The DNS read page only reads records through the official Cloudflare API, either per Zone in full, by the configured record names, or by explicit subdomains, filtered to A/AAAA. Pushing runs in scheduled tasks and in "push automatically after speed test", with Cloudflare (API Token + Zone ID + routing rules), DNSHE, Alibaba Cloud DNS, and DNSPod configurable side by side; IPv4 writes A records, IPv6 writes AAAA records. |
| Config and sync | Follows the current `config_snapshot` schema. ZIP config packages always contain `cfst-gui-config.json`; WebDAV test/backup/restore, `source-profiles.json` input groups, and GitHub result CSV export are also supported. Old configs get default values filled in and legacy field aliases migrated on read, and are written back in the current canonical format. |
| Online update | The settings page talks to GitHub Releases directly, matches platform assets through `cfst-gui-update-manifest.json`, downloads over an accelerator candidate chain without reading environment proxies, and verifies with SHA256. |

Config exports, ZIP archives, WebDAV backups, and local backup files may contain Cloudflare tokens, WebDAV credentials, export paths, and input source paths; store them in trusted locations only.

## Documentation

Complete usage, deployment, and interface documentation lives in [`docs/`](docs/index.md). These documents are written in Chinese:

| Topic | Document |
| --- | --- |
| Beginner walkthrough: adding input sources, handling COLO, setting up trace, starting a test | [docs/guide/CFST-GUI入门教程.md](docs/guide/CFST-GUI入门教程.md) |
| First use: export directory, COLO dictionary, input sources, and speed test | [docs/guide/quick-start.md](docs/guide/quick-start.md) |
| Product positioning, release assets, and installation advice | [docs/guide/介绍产品.md](docs/guide/介绍产品.md) |
| Running, deploying, building, and signing | [docs/guide/deployment.md](docs/guide/deployment.md) |
| Config directory, field defaults, and legacy config compatibility | [docs/guide/configuration.md](docs/guide/configuration.md) |
| All development, check, and release commands | [docs/dev/commands.md](docs/dev/commands.md) |
| Cross-platform commands, events, config, and task behavior baseline | [docs/dev/behavior-baseline.md](docs/dev/behavior-baseline.md) |
| Feature interface contracts, config structures, and code locations | [docs/reference/README.md](docs/reference/README.md) |
| Browser component tests and screenshot baselines | [docs/dev/browser-tests.md](docs/dev/browser-tests.md) |
| WebUI, Docker, Android, and Actions environment variables | [docs/guide/docker-env.md](docs/guide/docker-env.md) |
| Android architecture, SAF file access, and mobile bridge | [docs/mobile/android-mobile.md](docs/mobile/android-mobile.md) |
| Minimum-privilege Cloudflare DNS token | [docs/integration/cloudflare-api-token.md](docs/integration/cloudflare-api-token.md) |
| Minimum-privilege GitHub PAT for result export | [docs/integration/github-pat.md](docs/integration/github-pat.md) |
| Telegram Bot upload notification setup and troubleshooting | [docs/integration/telegram-bot.md](docs/integration/telegram-bot.md) |
| Upload filtering and automatic push rules | [docs/integration/upload-design.md](docs/integration/upload-design.md) |
| Version changelog | [CHANGELOG.md](CHANGELOG.md) |
| Release notes for every past version | [docs/release-notes/README.md](docs/release-notes/README.md) |

## Quick Start

### Requirements

Go 1.27.4, Node.js 26.12.1 + pnpm 12.16.1, and the Wails v3 CLI. The daily primary environment is PowerShell 7; cross-platform `.sh` scripts run through the `bash.exe` shipped with Git for Windows, so WSL is not required. Linux and macOS targets still need the matching platform toolchain.

```powershell
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.2.0-nightly.20261103
pnpm --dir frontend install
```

### Development

```powershell
wails3 dev                                  # start Vite and open the native Wails window with frontend HMR
wails3 generate bindings                    # generate Wails bridge code when frontend/bindings is missing
pnpm --dir frontend build; go run .         # run a standalone program embedding the current frontend output
bash scripts/dev/open-dev.sh webui          # browser form at http://127.0.0.1:34115
bash scripts/build/build-android-mobile.sh  # generate web assets and mobileapi.aar
```

`frontend/dist` is a build output and is not committed (only a `.gitkeep` is kept so `//go:embed` compiles); any binary that runs or ships must build the frontend first. A missing build fails fast and asks for `pnpm --dir frontend build` instead of falling back to an old UI. Startup logs print the frontend source in use: `proxying live Vite dev server` means live Vite source, `serving embedded frontend/dist snapshot` means the snapshot embedded in the binary.

The WebView2 user data directory is fixed at `%LOCALAPPDATA%\CFST-GUI\webview2` (kept out of the roaming profile to avoid syncing browser caches). Troubleshooting for development port conflicts, orphaned Vite processes, the single-instance lock, and the tray is in [deployment and build](docs/guide/deployment.md#本地开发); Android on-device debugging is in the [mobile documentation](docs/mobile/android-mobile.md#android-studio-真机调试).

### Building a Release

```powershell
$env:CFST_ANDROID_KEYSTORE = 'C:\path\to\release.jks'   # also needs KEYSTORE_PASSWORD
$env:CFST_ANDROID_KEY_ALIAS = '...'                     # and KEY_PASSWORD
bash scripts/build/build-release.sh                     # all by default
bash scripts/build/build-release.sh windows             # or build a single target
```

Available targets: `windows`, `linux`, `linux-amd64`, `linux-arm64`, `darwin-amd64`, `darwin-arm64`, `android`, `manifest`. The default `all` requires Windows amd64, Linux amd64/arm64, and Android arm64 and fails if any artifact is missing; a comma-separated `CFST_RELEASE_TARGETS` (for example `windows,android`) narrows the local build set. macOS builds only on a matching host and never enters a GitHub Release.

Release artifacts (`build/artifacts/release/`): the Windows installer `cfst-gui-windows-amd64.exe` (Simplified Chinese UI, checks the WebView2 Runtime before installing, and shows GPL-3.0 plus third-party notices at the bottom) and the portable build `-portable.exe`; the Linux WebUI bundle `cfst-gui-linux-{amd64,arm64}.tar.gz`; the Android build `cfst-gui-android-arm64-v8a-release.apk`; and `cfst-gui-update-manifest.json`. The GitHub Actions pipeline lives in `.github/workflows/release.yml`, triggered by `v*` tags, pushes to the `test` branch, or manually, and publishes only Windows, Android, and the manifest. Signing certificates, Windows SDK SignTool, Docker/GHCR, and other release details are in [deployment and build](docs/guide/deployment.md).

## Repository Layout

```text
frontend/          Vue frontend shared across desktop, WebUI, and Android (src/lib/bridge.ts facade + src/lib/bridge contracts + src/lib/client RPC clients + src/lib/adapters/{desktop,webui,android})
internal/
├── app/           Desktop / WebUI Go adapter (Wails bindings, WebUI HTTP/SSE, tray, and updater)
├── kernel/        Backend shared kernel (appcore orchestration + domain/* capabilities + infra/* foundations)
└── contracttest/  Cross-platform contract tests (desktop vs Android adapter parity)
mobileapi/         Android gomobile adapter exposed to the Kotlin layer
mobile/android/    Android native project, Kotlin Plugin, resources, and Gradle configuration
cmd/               Windows installer source (install actions, safety checks, custom UI)
build/             App icons, platform resources (config/ build config, windows/ installer manifest and icons), and local artifacts (artifacts/, not committed)
scripts/           Build (build/), quality gates (checks/), developer helpers (dev/), and shared functions (lib/)
docs/              guide/ user guides, dev/ developer docs, integration/ integrations, mobile/ mobile, reference/ interface reference, release-notes/ version history, plan/ maintenance records
tests/             Playwright end-to-end cases, visual baselines, and ARIA snapshots
devtools/          CDP debugging scripts (committed) and third-party runtime libraries plus browser profiles (not committed)
tools/             Developer helper tools (including the Cloudflare Worker probe)
sqlitegate/        Android gomobile SQLite driver verification entrypoint
```

The Go module path is `github.com/axuitomo/CFST-GUI`. Build artifacts all live under `build/artifacts/`; `cfst-results/` holds date-archived result CSVs and is not a build artifact; `devtools/profiles/` contains sensitive login data and is ignored.

## Common Development Commands

```powershell
& .\scripts\checks\ci-local.ps1          # one-shot local quality gate
& .\scripts\checks\check.ps1              # Go tests + frontend unit tests/typecheck/build
pnpm check:fast                           # second-level loop: minimal check set for changed areas
& .\scripts\checks\lint.ps1               # go vet, golangci-lint, shellcheck, actionlint, Oxlint, stylelint, markdownlint, ktlint/detekt
bash scripts/dev/format.sh                # formatting
& .\scripts\dev\audit.ps1                 # dependency verification and security audit
& .\scripts\checks\verify-generated.ps1   # Wails/frontend generated-file consistency
bash scripts/checks/changed-check.sh      # check only the current changes
bash scripts/checks/doctor.sh             # diagnose the development environment
bash scripts/checks/check-android.sh      # Android debug build, 16 KB page alignment, and APK manifest checks
bash scripts/checks/release-preflight.sh <version>   # pre-release checks
```

Helper scripts are read-only or dry-run by default; scripts that change files or the environment require explicit flags (such as `--apply`, `--force`). The complete command list and the prerequisites of `check.ps1` and `verify-generated.ps1` are in [docs/dev/commands.md](docs/dev/commands.md) and [`scripts/`](scripts/).

## Config and Data

The config root comes from Go's `os.UserConfigDir()` with `CFST-GUI` appended; desktop always uses the app data directory, and Android uses `/storage/emulated/0/Android/data/<package>/files`. The settings page no longer offers a custom storage directory, but `CFST_GUI_PORTABLE_ROOT` / `portable.json` still enable a portable data directory.

Main files:

- `storage.json`: storage bootstrap; the legacy custom `storage_dir` is only used for a one-time migration
- `desktop-config.json` / `mobile-config.json`: current config snapshots for the desktop GUI / WebUI and Android, structurally identical; `config.json` stays compatible with the legacy bridge structure
- `source-profiles.json`: input groups (`items[].sources`)
- `cfip-log.txt`: debug log, JSONL by default, with free-form format and record granularity selectable
- `exports/`, `imports/`, `backups/`: result exports, imported files, and pre-import backups

Fields missing from old configs are filled with current defaults on read. Unknown fields never fail the read, but are cleaned to the canonical format after saving, importing, writing back through WebDAV, or switching profiles. Field-level details are in the [configuration reference](docs/guide/configuration.md).

## Notes

- The default file speed test URL is `https://speed.cloudflare.com/__down?bytes=99999999` (that endpoint rejects values ≥ `100000000`); the default from older versions follows automatically during config normalization, and custom addresses are unaffected.
- All HTTP egress uses the shared client. The file speed test prefers HTTP/3 and falls back to HTTP/1.1/2 over TCP; source fetching and trace probing always use TCP so QUIC handshakes cannot eat into fetch or trace timeouts. Speed test GETs send `no-store`/`no-cache` and validate length, Range, and the Digest/MD5/SHA256 response headers. TLS certificate verification is off by default and can be enabled in "Network & Tasks → TLS certificate verification"; remote source fetching and update packages always verify.
- Speed test results depend on proxies, carriers, router policies, and local network conditions; trace probing, file speed tests, and large scans may trigger remote or network-side rate limiting.
- The DNS read page never modifies live records; scheduled tasks and push-after-test really create, update, or delete records, so read and confirm record names and routing rules before enabling them.
- Config and archive files may contain sensitive credentials; never commit them to public repositories or share them publicly.

## Contact

Telegram channel direct message: <https://t.me/axuitomo_pindao?direct>

## Acknowledgements

- [XIU2/CloudflareSpeedTest](https://github.com/XIU2/CloudflareSpeedTest): the original core speed test logic this project was based on (upstream GPLv3 attribution in the [third-party notices](THIRD-PARTY-NOTICES.md))
- [Leo-Mu/montecarlo-ip-searcher](https://github.com/Leo-Mu/montecarlo-ip-searcher): the MICS sampling search approach
- [cmliu/CF-Workers-SpeedTestURL](https://github.com/cmliu/CF-Workers-SpeedTestURL): speed test URL reference
- [Netrvin/cloudflare-colo-list](https://github.com/Netrvin/cloudflare-colo-list): Cloudflare COLO data source reference
- [baipiao.eu.org cloudflare locations](https://www.baipiao.eu.org/cloudflare/locations): fallback city naming source for the COLO dictionary (referenced via [PoemMisty/CFData-WEB](https://github.com/PoemMisty/CFData-WEB))
- [Cloudflare local IP ranges](https://api.cloudflare.com/local-ip-ranges.csv): Cloudflare local IP range data source
- [xiaolin-007/CloudFlareScan](https://github.com/xiaolin-007/CloudFlareScan): Cloudflare scanning tool reference

## License

Copyright (C) 2026 axuitomo.

This project is licensed under GNU GPL version 3 (SPDX: GPL-3.0-only); the full license text is in [LICENSE](LICENSE). `LICENSE` is a verbatim copy of the FSF text, so do not append the project name, copyright lines, comments, or third-party notices to it, and do not merge in other licenses, otherwise platforms such as GitHub will fail to recognize it; `pnpm check:license` enforces this.

Third-party components shipped with release artifacts, their original license texts, and the attribution list are in [THIRD-PARTY-NOTICES.md](THIRD-PARTY-NOTICES.md).
