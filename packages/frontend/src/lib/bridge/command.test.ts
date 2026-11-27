import { describe, expect, it } from "vitest";
import { normalizeCommandResult, SCHEMA_VERSION } from "./command";

describe("normalizeCommandResult", () => {
  it("turns non-object responses into stable failures", () => {
    expect(normalizeCommandResult("native bridge unavailable")).toEqual({
      code: "UNKNOWN",
      data: null,
      message: "native bridge unavailable",
      ok: false,
      schema_version: SCHEMA_VERSION,
      task_id: null,
      warnings: [],
    });
  });
});
