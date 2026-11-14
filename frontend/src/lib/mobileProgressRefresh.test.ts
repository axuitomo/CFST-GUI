import { describe, expect, it } from "vitest";
import { shouldRefreshProgress } from "./mobileProgressRefresh";

describe("shouldRefreshProgress", () => {
  it("throttles unchanged mobile progress within the refresh interval", () => {
    expect(shouldRefreshProgress(1000, "stage1_tcp", "stage1_tcp", 20, 100, 1199)).toBe(false);
    expect(shouldRefreshProgress(1000, "stage1_tcp", "stage1_tcp", 20, 100, 1200)).toBe(true);
  });

  it("refreshes immediately on stage changes and completion", () => {
    expect(shouldRefreshProgress(1000, "stage1_tcp", "stage2_trace", 1, 100, 1001)).toBe(true);
    expect(shouldRefreshProgress(1000, "stage1_tcp", "stage1_tcp", 100, 100, 1001)).toBe(true);
  });
});
