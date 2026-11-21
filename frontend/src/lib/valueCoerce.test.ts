import { describe, expect, it } from "vitest";
import {
  asBoolean,
  asCount,
  asNullableNumber,
  asNumber,
  asRecord,
  asString,
  boundedCount,
  clampedNumber,
  minimumCount,
  nonNegativeCount,
  nonNegativeNumber,
  normalizeCloudflareTTL,
  normalizeDownloadHTTPProtocol,
  normalizeResultCloudflareRecordType,
  normalizeSchedulerDailyTime,
  optionalNumberForPayload,
  parseClockMinutes,
  positiveCount,
  schedulerDailyTimesFromText,
  schedulerTriggerModeFromSnapshot,
} from "./valueCoerce";

describe("基础 coercion", () => {
  it("asCount 取整并回退", () => {
    expect(asCount("12")).toBe(12);
    expect(asCount("12.9")).toBe(12);
    expect(asCount(5)).toBe(5);
    expect(asCount(null, 7)).toBe(7);
    expect(asCount(undefined, 7)).toBe(7);
    expect(asCount("", 7)).toBe(7);
    expect(asCount("abc", 7)).toBe(7);
  });

  it("asRecord 只接受非数组对象", () => {
    const obj = { a: 1 };
    expect(asRecord(obj)).toBe(obj);
    expect(asRecord([1])).toEqual({});
    expect(asRecord(null)).toEqual({});
    expect(asRecord("x")).toEqual({});
    expect(asRecord(3)).toEqual({});
  });

  it("asString 空值归一为空串", () => {
    expect(asString("a")).toBe("a");
    expect(asString(null)).toBe("");
    expect(asString(undefined)).toBe("");
    expect(asString(5)).toBe("5");
    expect(asString(true)).toBe("true");
  });

  it("asBoolean 识别常见真/假词", () => {
    expect(asBoolean(true)).toBe(true);
    expect(asBoolean(false)).toBe(false);
    expect(asBoolean(1)).toBe(true);
    expect(asBoolean(0)).toBe(false);
    expect(asBoolean(-2)).toBe(true);
    expect(asBoolean(" YES ")).toBe(true);
    expect(asBoolean("on")).toBe(true);
    expect(asBoolean("1")).toBe(true);
    expect(asBoolean("OFF")).toBe(false);
    expect(asBoolean("no")).toBe(false);
    expect(asBoolean("0")).toBe(false);
    expect(asBoolean("maybe")).toBe(false);
    expect(asBoolean("maybe", true)).toBe(true);
    expect(asBoolean(null, true)).toBe(true);
  });

  it("asNumber 浮点解析并回退", () => {
    expect(asNumber("1.5")).toBe(1.5);
    expect(asNumber(2)).toBe(2);
    expect(asNumber("abc", 9)).toBe(9);
    expect(asNumber(null, 9)).toBe(9);
  });
});

describe("计数边界", () => {
  it("positiveCount 非正回退且可封顶", () => {
    expect(positiveCount(0, 2)).toBe(2);
    expect(positiveCount(-3, 2)).toBe(2);
    expect(positiveCount("5", 2)).toBe(5);
    expect(positiveCount(10, 2, 5)).toBe(5);
    expect(positiveCount(3, 2, 5)).toBe(3);
  });

  it("minimumCount 保底", () => {
    expect(minimumCount(1, 5, 3)).toBe(3);
    expect(minimumCount(10, 5, 3, 8)).toBe(8);
    expect(minimumCount(0, 5, 3)).toBe(5);
  });

  it("boundedCount 双侧夹逼", () => {
    expect(boundedCount(5, 0, 1, 10)).toBe(5);
    expect(boundedCount(-5, 0, 1, 10)).toBe(1);
    expect(boundedCount(50, 0, 1, 10)).toBe(10);
    expect(boundedCount("abc", 7, 1, 10)).toBe(7);
  });

  it("nonNegativeCount 负数回退", () => {
    expect(nonNegativeCount(-1, 4)).toBe(4);
    expect(nonNegativeCount(0)).toBe(0);
    expect(nonNegativeCount("3")).toBe(3);
  });
});

describe("数值边界", () => {
  it("nonNegativeNumber 负数回退", () => {
    expect(nonNegativeNumber(-0.5, 2)).toBe(2);
    expect(nonNegativeNumber("1.25")).toBe(1.25);
    expect(nonNegativeNumber(0)).toBe(0);
  });

  it("clampedNumber 双侧夹逼", () => {
    expect(clampedNumber(5, 0, 1, 10)).toBe(5);
    expect(clampedNumber(-1, 0, 1, 10)).toBe(1);
    expect(clampedNumber(11, 0, 1, 10)).toBe(10);
    expect(clampedNumber("x", 3, 1, 10)).toBe(3);
  });

  it("asNullableNumber 空值与非法值为 null", () => {
    expect(asNullableNumber(null)).toBeNull();
    expect(asNullableNumber(undefined)).toBeNull();
    expect(asNullableNumber("")).toBeNull();
    expect(asNullableNumber("1.5")).toBe(1.5);
    expect(asNullableNumber("abc")).toBeNull();
    expect(asNullableNumber(0)).toBe(0);
  });

  it("optionalNumberForPayload 只放行正有限数", () => {
    expect(optionalNumberForPayload(5)).toBe(5);
    expect(optionalNumberForPayload(0)).toBeNull();
    expect(optionalNumberForPayload(-1)).toBeNull();
    expect(optionalNumberForPayload(null)).toBeNull();
    expect(optionalNumberForPayload(Number.NaN)).toBeNull();
    expect(optionalNumberForPayload(Number.POSITIVE_INFINITY)).toBeNull();
  });
});

describe("域归一化", () => {
  it("normalizeCloudflareTTL 只允许 60/300/600", () => {
    expect(normalizeCloudflareTTL(60)).toBe(60);
    expect(normalizeCloudflareTTL(300)).toBe(300);
    expect(normalizeCloudflareTTL("600")).toBe(600);
    expect(normalizeCloudflareTTL(120)).toBe(300);
    expect(normalizeCloudflareTTL(null)).toBe(300);
  });

  it("normalizeDownloadHTTPProtocol 别名收敛", () => {
    expect(normalizeDownloadHTTPProtocol("h1")).toBe("h1");
    expect(normalizeDownloadHTTPProtocol("h1.1")).toBe("h1");
    expect(normalizeDownloadHTTPProtocol("HTTP/1.1")).toBe("h1");
    expect(normalizeDownloadHTTPProtocol("h2")).toBe("h2");
    expect(normalizeDownloadHTTPProtocol("http/2")).toBe("h2");
    expect(normalizeDownloadHTTPProtocol("h3")).toBe("h3");
    expect(normalizeDownloadHTTPProtocol("http/3")).toBe("h3");
    expect(normalizeDownloadHTTPProtocol("auto")).toBe("auto");
    expect(normalizeDownloadHTTPProtocol("")).toBe("auto");
    expect(normalizeDownloadHTTPProtocol(null)).toBe("auto");
    expect(normalizeDownloadHTTPProtocol("bogus")).toBe("auto");
  });

  it("normalizeResultCloudflareRecordType 大小写与回退", () => {
    expect(normalizeResultCloudflareRecordType("a")).toBe("A");
    expect(normalizeResultCloudflareRecordType("aaaa")).toBe("AAAA");
    expect(normalizeResultCloudflareRecordType(" A ")).toBe("A");
    expect(normalizeResultCloudflareRecordType("all")).toBe("ALL");
    expect(normalizeResultCloudflareRecordType("")).toBe("ALL");
    expect(normalizeResultCloudflareRecordType(null)).toBe("ALL");
  });

  it("schedulerTriggerModeFromSnapshot daily 优先", () => {
    expect(schedulerTriggerModeFromSnapshot({ daily_times: ["08:00"], interval_minutes: 0 } as never, "interval")).toBe("daily");
    expect(schedulerTriggerModeFromSnapshot({ daily_times: [], interval_minutes: 5 } as never, "daily")).toBe("interval");
    expect(schedulerTriggerModeFromSnapshot({ daily_times: [], interval_minutes: 0 } as never, "daily")).toBe("daily");
    expect(schedulerTriggerModeFromSnapshot({ daily_times: ["08:00"], interval_minutes: 5 } as never, "interval")).toBe("daily");
  });

  it("normalizeSchedulerDailyTime 补零与越界拒绝", () => {
    expect(normalizeSchedulerDailyTime("8:5")).toBe("08:05");
    expect(normalizeSchedulerDailyTime("23:59:59")).toBe("23:59:59");
    expect(normalizeSchedulerDailyTime("12：30")).toBe("12:30");
    expect(normalizeSchedulerDailyTime("24:00")).toBe("");
    expect(normalizeSchedulerDailyTime("12:60")).toBe("");
    expect(normalizeSchedulerDailyTime("12:00:60")).toBe("");
    expect(normalizeSchedulerDailyTime("abc")).toBe("");
  });

  it("schedulerDailyTimesFromText 多分隔符切分并过滤非法项", () => {
    expect(schedulerDailyTimesFromText("8:00, 9：30；10:00:05、bad")).toEqual(["08:00", "09:30", "10:00:05"]);
    expect(schedulerDailyTimesFromText("07:00 08:00")).toEqual(["07:00", "08:00"]);
    expect(schedulerDailyTimesFromText("")).toEqual([]);
  });

  it("parseClockMinutes 解析时钟并回退", () => {
    expect(parseClockMinutes("19:30", -1)).toBe(1170);
    expect(parseClockMinutes("7:00", -1)).toBe(420);
    expect(parseClockMinutes(" 07:05 ", -1)).toBe(425);
    expect(parseClockMinutes("24:00", -1)).toBe(-1);
    expect(parseClockMinutes("12:60", -1)).toBe(-1);
    expect(parseClockMinutes("abc", -1)).toBe(-1);
  });
});