import { describe, expect, it } from "vitest";
import { taskActionLabel, taskSessionLabel, taskStageLabel, taskStatusLabel } from "./labels";

describe("labels", () => {
  it("把登记过的状态映射到 tone 词表", () => {
    expect(taskStatusLabel("completed")).toBe("已完成");
    expect(taskStatusLabel("partial")).toBe("部分完成");
    expect(taskStatusLabel("recovery_required")).toBe("警告");
    expect(taskStatusLabel("")).toBe("就绪");
  });

  it("未登记的状态原样返回，不编词", () => {
    expect(taskStatusLabel("some_future_state")).toBe("some_future_state");
  });

  it("动作词与句子里的用法一致", () => {
    expect(taskActionLabel("cancel")).toBe("终止");
    expect(taskActionLabel("start")).toBe("启动");
  });

  it("阶段与行级 stage_status 共用一张阶段表", () => {
    expect(taskStageLabel("stage1_tcp")).toBe("TCP 延迟");
    expect(taskStageLabel("stage3_get")).toBe("文件测速");
    expect(taskStageLabel("")).toBe("-");
  });

  it("会话状态有独立词表，不混进任务状态", () => {
    expect(taskSessionLabel("persisted_only")).toBe("仅恢复快照");
    expect(taskSessionLabel("paused_runtime")).toBe("已暂停");
  });
});
