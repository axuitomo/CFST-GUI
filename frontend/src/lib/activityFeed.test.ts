import { describe, expect, it } from "vitest";
import { ACTIVITY_FEED_LIMIT, appendActivity, type ActivityEntry } from "./activityFeed";

const entry = (title: string, ts = "2026-01-01T00:00:00Z"): ActivityEntry => ({ count: 1, detail: `${title} 详情`, title, ts });

describe("appendActivity", () => {
  it("新标题单独成行", () => {
    const feed = appendActivity([entry("应用已启动")], { detail: "d", title: "配置已保存", ts: "t" });
    expect(feed[0]).toEqual({ count: 1, detail: "d", title: "配置已保存", ts: "t" });
    expect(feed).toHaveLength(2);
  });

  it("连续同名事件合并计数，并保留最新详情与时间", () => {
    let feed = appendActivity([], { detail: "1/10", title: "MICS 抽样进行中", ts: "t1" });
    feed = appendActivity(feed, { detail: "5/10", title: "MICS 抽样进行中", ts: "t2" });
    feed = appendActivity(feed, { detail: "10/10", title: "MICS 抽样进行中", ts: "t3" });

    expect(feed).toHaveLength(1);
    expect(feed[0]).toEqual({ count: 3, detail: "10/10", title: "MICS 抽样进行中", ts: "t3" });
  });

  it("中间隔了别的记录就不再合并", () => {
    let feed = appendActivity([], { detail: "", title: "保存失败", ts: "t1" });
    feed = appendActivity(feed, { detail: "", title: "配置已加载", ts: "t2" });
    feed = appendActivity(feed, { detail: "", title: "保存失败", ts: "t3" });

    expect(feed.map((item) => item.count)).toEqual([1, 1, 1]);
  });

  it("合并不会撑破上限", () => {
    let feed: ActivityEntry[] = [];
    for (let index = 0; index < ACTIVITY_FEED_LIMIT + 5; index += 1) {
      feed = appendActivity(feed, { detail: `第 ${index} 次`, title: "MICS 抽样进行中", ts: `t${index}` });
    }

    expect(feed).toHaveLength(1);
    expect(feed[0].count).toBe(ACTIVITY_FEED_LIMIT + 5);
  });

  it("超过上限时丢掉最旧的一条", () => {
    let feed: ActivityEntry[] = [];
    for (let index = 0; index < ACTIVITY_FEED_LIMIT + 1; index += 1) {
      feed = appendActivity(feed, { detail: "", title: `事件 ${index}`, ts: `t${index}` });
    }

    expect(feed).toHaveLength(ACTIVITY_FEED_LIMIT);
    expect(feed[0].title).toBe(`事件 ${ACTIVITY_FEED_LIMIT}`);
    expect(feed.some((item) => item.title === "事件 0")).toBe(false);
  });
});
