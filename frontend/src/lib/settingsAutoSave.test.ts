import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { createSettingsAutoSaver } from "./settingsAutoSave";

describe("createSettingsAutoSaver", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("coalesces a burst of interactions into one run", () => {
    const run = vi.fn();
    const saver = createSettingsAutoSaver(run, 200);

    saver.schedule("interaction");
    vi.advanceTimersByTime(150);
    saver.schedule("interaction");
    vi.advanceTimersByTime(150);
    expect(run).not.toHaveBeenCalled();

    vi.advanceTimersByTime(50);
    expect(run).toHaveBeenCalledTimes(1);
    expect(run).toHaveBeenCalledWith("interaction");
  });

  it("cancels a pending run when the caller flushes first", () => {
    const run = vi.fn();
    const saver = createSettingsAutoSaver(run, 200);

    saver.schedule("interaction");
    expect(saver.takePending()).toBe(true);
    expect(saver.takePending()).toBe(false);

    vi.advanceTimersByTime(500);
    expect(run).not.toHaveBeenCalled();
  });
});
