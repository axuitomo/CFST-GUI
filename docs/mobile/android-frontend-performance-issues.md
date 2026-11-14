# Android Frontend Performance Findings

## Scope

- Build: current workspace Android ARM64 Debug build, package `io.github.axuitomo.cfstgui.debug`, version `2.0.6`.
- Device: `23113RKC6C`, Android 16 / API 36, WebView `153.0.8010.36`, CSS viewport `411 × 914`; display supports 60/90/120 Hz and reported 120 Hz after sampling.
- Test data size: 2 input profiles and 4 source rows. Names and source contents are intentionally omitted.
- Input: CDP-dispatched mouse events against the real device WebView, not emulator clicks. This exercises the browser event/render path but is not a finger-touch latency measurement.
- Interaction timing uses `PerformanceObserver` Event Timing entries with a 16 ms duration threshold; “INP candidate” below is the largest observed interaction entry in the sample, not a formal field INP percentile. Frame rates are short-window `requestAnimationFrame` samples. No persistent instrumentation was added.

## Findings

### P1: Mobile page navigation preserves the previous scroll offset

**Reproduction:** in the Settings page, set `.mobile-main.scrollTop` to `600`, then navigate to Sources. The Settings page had `scrollHeight=1727` and `clientHeight=411`. On Sources, the shared scroll container retained/clamped the offset to `307.7` (`719 - 411`); the “expand source-group manager” control was at viewport Y `-170` and was not visible. Navigating onward to Dashboard retained `scrollTop=307.7`, with page content starting at Y `-183.7`.

This makes a newly selected page open partway down, and can put its top-level controls outside the viewport. The result is repeatable and independent of the performance observer.

**Code location:** [MobileShell.vue](../../frontend/src/components/layout/MobileShell.vue#L44) keeps one `.mobile-main` scroll container around the changing slot. [App.vue](../../frontend/src/App.vue#L2527) changes the selected view but does not reset or restore that container's scroll position.

**Confidence:** confirmed.

### P2: Repeated input-profile manager expand/collapse drops to about 60 FPS

The test toggled the profile manager 5 times with 2 profiles present. The first cycles were about 118–120 FPS; cycles 3–5 repeatedly fell to about 60 FPS. Across the full run, INP candidates were P50 `24 ms`, max `56 ms`; the independent repeat reached `64 ms`. The repeated run recorded 16.7 ms frame intervals and no `longtask` entries over 50 ms. A later four-source editor expand-all-then-collapse-all sequence stayed around 118–120 FPS, with INP candidates P50 `16 ms`, max `24 ms`.

The profile manager creates the profile list and its controls together, then animates the container's height. The CSS transition changes `height`, margin and padding for 260 ms. That is a plausible layout/paint cost under repeated toggles; the absence of long tasks makes a large synchronous JavaScript task less likely, but does not prove the rendering root cause.

**Code location:** [SourcesView.vue](../../frontend/src/views/SourcesView.vue#L633) toggles the manager; the profile list is rendered with `v-for` at [SourcesView.vue](../../frontend/src/views/SourcesView.vue#L655). [styles.css](../../frontend/src/styles.css#L869) animates `height`, margin and padding.

**Confidence:** repeated symptom confirmed; layout/paint attribution is a hypothesis pending a browser/Perfetto trace. Only 2 profiles were available, so larger production configurations remain untested.

### P2: Page switching has isolated frame delays, without a JS long task

Across 17 switches over the Dashboard, Results, Settings and Sources views, the interaction candidate was P50 `48 ms`, max `96 ms`. The short-window frame-rate sample was P50 `110.6 FPS`, with a low of `99.2 FPS`; 25 sampled frame intervals exceeded `16.7 ms` across the action windows. No `longtask` entry over 50 ms was recorded. The display supports 120 Hz, and Android reported an active 120 Hz mode after the run; refresh mode was not pinned or sampled for each individual interaction.

The route path uses `document.startViewTransition()` when supported, changes a `v-if`-selected page subtree, and animates the full `.page-transition-content` snapshot for 60–80 ms. Those operations are the primary suspects for the isolated delays, but the current data does not distinguish snapshot capture, layout, paint, compositing, or Android refresh-rate policy.

**Code location:** [App.vue](../../frontend/src/App.vue#L2527) starts the view transition; [MobileShell.vue](../../frontend/src/components/layout/MobileShell.vue#L46) applies it to the full page slot; [styles.css](../../frontend/src/styles.css#L813) defines the snapshot animation durations.

**Confidence:** frame-delay symptom confirmed in this run; specific rendering stage unconfirmed.

### Settings page and source-group controls

The test exercised the Basic/Advanced settings switch and opened/closed the Online Update, Probe Strategy and App Data Directory sections. Ten measured actions had INP candidates P50 `24 ms`, max `40 ms`, frame-rate P50 `120.1 FPS`, and no long tasks. Three frame intervals exceeded `16.7 ms` across these windows.

The input-group settings panel was opened and closed three times. Six actions had INP candidates P50 `24 ms`, max `40 ms`; frame-rate P50 was `109.8 FPS`, with no long tasks. These actions did not show the repeated 60 FPS behavior seen in the profile-manager list.

**Code location:** settings mode and conditional advanced content are in [SettingsView.vue](../../frontend/src/views/SettingsView.vue#L493) and [SettingsView.vue](../../frontend/src/views/SettingsView.vue#L824); the input-group control transition is in [SourcesView.vue](../../frontend/src/views/SourcesView.vue#L641).

## Measurement Summary

| Scenario | Actions | INP candidate P50 / max | RAF FPS P50 / min | Long tasks |
| --- | ---: | ---: | ---: | ---: |
| Settings mode and sections | 10 | 24 / 40 ms | 120.1 / 116.3 | 0 |
| Profile manager expand/collapse | 10 | 24 / 56 ms; repeat max 64 ms | 118.1 / 59.9 | 0 |
| Input-group settings panel | 6 | 24 / 40 ms | 109.8 / 104.3 | 0 |
| Four source editors, all open then all closed | 8 | 16 / 24 ms | 120.2 / 118.3 | 0 |
| Page switching | 17 | 48 / 96 ms | 110.6 / 99.2 | 0 |

FPS is an observed short-window rate, not a guaranteed fixed display refresh rate. Frame-interval counts are reported as samples, not as a device-wide jank percentage.

## Next Verification
The initial verification items have been superseded by the 2026-11-13 Perfetto and INP retest below.


## Perfetto and INP Retest (2026-11-13)

### Capture

- Device/build remained `23113RKC6C`, Android 16 / API 36, WebView `153.0.8010.36`, Debug `2.0.6`; source test data was 2 profiles and 4 source rows.
- A 90-second Perfetto trace was captured with `linux.ftrace` (app-scoped `gfx`, `view`, `input`, `wm`, `am`, `webview`, `sched`, CPU frequency/idle), `android.surfaceflinger.frametimeline`, `android.surfaceflinger.frame`, and `android.surfaceflinger.layers`. The trace covers 88.81 seconds of FrameTimeline data.
- Chrome tracing ran in parallel for three phases: Settings, Sources, and repeated page navigation. Event Timing entries were grouped by `interactionId`; the largest event duration per tap is reported as the INP candidate. Durations are decomposed into input delay, event processing, and presentation delay. This remains CDP-dispatched input, not a finger-touch test or field INP. Chrome tracing overhead is present and may elevate timings.
- Raw local captures (not committed): `cfst-focused.perfetto-trace`, `cfst-chrome-settings.json`, `cfst-chrome-sources.json`, `cfst-chrome-navigation.json`, and `cfst-inp-results.json`. These are temporary artifacts, not durable repository fixtures.

### INP Results

| Scenario | Actions | Candidate P50 | P95 | Max | Renderer `RunTask` >50 ms |
| --- | ---: | ---: | ---: | ---: | ---: |
| Settings navigation, sections and mode toggle | 9 | 24 ms | 168 ms | 168 ms | 4 |
| Input-profile manager expand/collapse | 12 | 32 ms | 48 ms | 48 ms | 0 |
| Input-group settings panel expand/collapse | 6 | 24 ms | 40 ms | 40 ms | 0 |
| Four source editors, open and close | 8 | 16 ms | 32 ms | 32 ms | 0 |
| Page navigation | 13 valid actions | 64 ms | 112 ms | 112 ms | 6 |

The “Settings” maximum was entering the page: `168 ms` total, split into `15.9 ms` input delay, `2.5 ms` processing, and `149.6 ms` presentation. Entering Sources measured `152 ms` (`0.7 ms` input delay, `0.2 ms` processing, `151.1 ms` presentation). This points to post-input rendering/presentation rather than a slow event handler.

For the input-profile manager, all 12 toggles completed at or below `48 ms`; the renderer main thread had no `RunTask` over 50 ms. Across these windows, the longest `UpdateLayoutTree` / style-layout slice was `15.4 ms`, and the longest `Paint` slice was under `1 ms`. This run does not reproduce the earlier sustained 60 FPS observation. It also does not establish scaling behavior beyond the two profiles present on the device.

The advanced/basic settings mode switches measured `64 ms` and `56 ms`; each aligned with a renderer `RunTask` of about `77 ms`. Settings page entry had a `136.3 ms` renderer-main `RunTask`; the nested trace included a `20.5 ms` style/layout update. Page navigation had six renderer-main tasks above 50 ms, with a maximum of `86.8 ms` and a maximum style/layout slice of `34.1 ms`. `ViewTransition::ProcessCurrentState` events were present during route transitions. These are measured correlations; the trace does not prove that View Transition alone causes the delay.

### Perfetto FrameTimeline Limitation

The 88.81-second Perfetto query of `actual_frame_timeline_slice` returned rows for `/system/bin/surfaceflinger` and `com.android.systemui`, but **no rows for `io.github.axuitomo.cfstgui.debug` or its `MainActivity` layer**. An earlier broad capture had only 67 CFST layer rows spanning 0.55 seconds, which is also insufficient to compute an app jank rate. Therefore this Perfetto capture does not support a valid CFST FPS or app-jank percentage. The Chrome renderer trace above supplies main-thread/layout/paint evidence, but is not a substitute for app FrameTimeline presentation data.

### Updated Assessment

1. Settings and route-entry INP outliers are presentation-dominated. The event handlers themselves are generally short; page mounting, style/layout, and transition presentation are the likely cost centers.
2. Settings mode changes are the clearest synchronous main-thread hotspot: approximately 77 ms `RunTask` on both mode switches.
3. Source-profile manager toggles are not showing a >50 ms renderer task at this data size. The previous FPS decline remains unconfirmed and should not be attributed to CSS height animation without a matching app-frame trace.
4. The scroll-offset defect documented above is separate and remains confirmed.

### Remaining Validation

1. Fix or otherwise validate Android FrameTimeline capture for the WebView/MainActivity layer, then repeat a short trace started immediately before the interaction sequence. Until app rows are present, keep FPS and jank conclusions open.
2. Repeat Settings entry/mode switches and page navigation without DevTools tracing, and with actual finger taps, to estimate tracing and CDP input overhead.
3. Repeat the profile-manager test with 10, 25, and 50 profiles.

This retest changed no application code. Settings mode and source panels were restored; the app was left on Dashboard with scroll position reset.
