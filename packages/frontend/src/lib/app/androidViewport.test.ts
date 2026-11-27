import { describe, expect, it } from "vitest";
import { ANDROID_KEYBOARD_DETECT_THRESHOLD_PX, computeAndroidViewportState, type AndroidViewportSample } from "./androidViewport";

function sample(overrides: Partial<AndroidViewportSample>): AndroidViewportSample {
  return { editing: false, innerHeight: 411, visualTop: 0, visualHeight: 411, ...overrides };
}

describe("computeAndroidViewportState", () => {
  it("detects the keyboard when adjustResize shrinks the window itself", () => {
    // 真机横屏实测：411 → 124，innerHeight 与 visualViewport.height 一起缩，两者差值恒为 0。
    const state = computeAndroidViewportState(411, sample({ editing: true, innerHeight: 124, visualHeight: 124 }));
    expect(state.keyboardOpen).toBe(true);
    expect(state.keyboardInset).toBe(0);
    expect(state.baselineHeight).toBe(411);
  });

  it("detects the keyboard when the window keeps its height (adjustPan)", () => {
    const state = computeAndroidViewportState(914, sample({ editing: true, innerHeight: 914, visualHeight: 563 }));
    expect(state.keyboardOpen).toBe(true);
    expect(state.keyboardInset).toBe(351);
  });

  it("treats a viewport change without a focused text control as rotation, not keyboard", () => {
    const state = computeAndroidViewportState(914, sample({ editing: false, innerHeight: 411, visualHeight: 411 }));
    expect(state.keyboardOpen).toBe(false);
    expect(state.baselineHeight).toBe(411);
  });

  it("keeps the baseline while editing so the shrink does not become the new normal", () => {
    const open = computeAndroidViewportState(411, sample({ editing: true, innerHeight: 124, visualHeight: 124 }));
    const closed = computeAndroidViewportState(open.baselineHeight, sample({ innerHeight: 411, visualHeight: 411 }));
    expect(open.keyboardOpen).toBe(true);
    expect(closed.keyboardOpen).toBe(false);
    expect(closed.baselineHeight).toBe(411);
  });

  it("follows a taller viewport while editing instead of reporting a keyboard", () => {
    const state = computeAndroidViewportState(411, sample({ editing: true, innerHeight: 914, visualHeight: 914 }));
    expect(state.keyboardOpen).toBe(false);
    expect(state.baselineHeight).toBe(914);
  });

  it("applies the threshold strictly", () => {
    const atThreshold = computeAndroidViewportState(411, sample({ editing: true, innerHeight: 411 - ANDROID_KEYBOARD_DETECT_THRESHOLD_PX, visualHeight: 411 - ANDROID_KEYBOARD_DETECT_THRESHOLD_PX }));
    const overThreshold = computeAndroidViewportState(411, sample({ editing: true, innerHeight: 411 - ANDROID_KEYBOARD_DETECT_THRESHOLD_PX - 1, visualHeight: 411 - ANDROID_KEYBOARD_DETECT_THRESHOLD_PX - 1 }));
    expect(atThreshold.keyboardOpen).toBe(false);
    expect(overThreshold.keyboardOpen).toBe(true);
  });

  it("does not guess a keyboard on the first sample taken while editing", () => {
    const state = computeAndroidViewportState(0, sample({ editing: true, innerHeight: 124, visualHeight: 124 }));
    expect(state.keyboardOpen).toBe(false);
    expect(state.baselineHeight).toBe(124);
  });
});
