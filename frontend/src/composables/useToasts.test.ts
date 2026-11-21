import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useToasts } from "./useToasts";

describe("useToasts", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });
  afterEach(() => {
    vi.useRealTimers();
  });

  it("默认 success tone，id 单调递增", () => {
    const { showToast, toasts } = useToasts();
    showToast("第一条");
    showToast("第二条", "error");
    expect(toasts.value).toEqual([
      { id: 0, message: "第一条", tone: "success" },
      { id: 1, message: "第二条", tone: "error" },
    ]);
  });

  it("3200ms 后按各自到期时间移除", () => {
    const { showToast, toasts } = useToasts();
    showToast("早");
    vi.advanceTimersByTime(1000);
    showToast("晚");
    vi.advanceTimersByTime(2200);
    expect(toasts.value.map((t) => t.message)).toEqual(["晚"]);
    vi.advanceTimersByTime(1000);
    expect(toasts.value).toEqual([]);
  });

  it("实例之间 id 与队列互不影响", () => {
    const a = useToasts();
    const b = useToasts();
    a.showToast("a");
    b.showToast("b");
    expect(a.toasts.value[0].id).toBe(0);
    expect(b.toasts.value[0].id).toBe(0);
    expect(a.toasts.value).toHaveLength(1);
  });
});