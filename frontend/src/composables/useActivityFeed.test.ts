import { describe, expect, it } from "vitest";
import { useActivityFeed } from "./useActivityFeed";

describe("useActivityFeed", () => {
  it("相邻同名事件合并计数，新事件置顶", () => {
    const { activityFeed, pushActivity } = useActivityFeed();
    pushActivity("探测", "第一次");
    pushActivity("探测", "第二次");
    expect(activityFeed.value).toHaveLength(1);
    expect(activityFeed.value[0]).toMatchObject({ count: 2, detail: "第二次", title: "探测" });
    pushActivity("导出", "完成");
    expect(activityFeed.value.map((e) => e.title)).toEqual(["导出", "探测"]);
    expect(activityFeed.value[0].count).toBe(1);
  });

  it("时间戳为 ISO 字符串", () => {
    const { activityFeed, pushActivity } = useActivityFeed();
    pushActivity("标题", "详情");
    expect(Number.isNaN(Date.parse(activityFeed.value[0].ts))).toBe(false);
  });
});