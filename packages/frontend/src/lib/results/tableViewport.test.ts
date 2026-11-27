import { describe, expect, it } from "vitest";
import { computeTableWindow } from "./tableViewport";

const base = { headerHeight: 40, overscan: 8, rowHeight: 57, windowRows: 40 };

describe("computeTableWindow", () => {
  it("顶部时从第 0 行开始，下方占满剩余高度", () => {
    expect(computeTableWindow({ ...base, scrollTop: 0, total: 1000 })).toEqual({ bottomSpacer: 54720, start: 0, topSpacer: 0 });
  });

  it("滚动后按行高定位窗口并留出上下占位", () => {
    // 表头 40 + 100 行 = 5740，再减去 overscan 8 行。
    expect(computeTableWindow({ ...base, scrollTop: 5740, total: 1000 })).toEqual({ bottomSpacer: 49476, start: 92, topSpacer: 5244 });
  });

  it("接近底部时窗口不越过最后一行", () => {
    const window = computeTableWindow({ ...base, scrollTop: 40 + 57 * 500, total: 100 });
    expect(window.start).toBe(99);
    expect(window.topSpacer).toBe(99 * 57);
    expect(window.bottomSpacer).toBe(0);
  });

  it("空表与非法行高都不产生 NaN", () => {
    expect(computeTableWindow({ ...base, scrollTop: 0, total: 0 })).toEqual({ bottomSpacer: 0, start: 0, topSpacer: 0 });
    const fallback = computeTableWindow({ ...base, rowHeight: 0, scrollTop: 40, total: 100 });
    expect(Number.isFinite(fallback.topSpacer)).toBe(true);
    expect(fallback.start).toBe(0);
  });

  it("内容少于窗口行数时没有下方占位", () => {
    expect(computeTableWindow({ ...base, scrollTop: 0, total: 10 })).toEqual({ bottomSpacer: 0, start: 0, topSpacer: 0 });
  });
});
