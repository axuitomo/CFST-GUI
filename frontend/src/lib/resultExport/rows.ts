// 结果导出的行裁剪工具：TopN=0 视为全量，正数按顺序截断。
import { nonNegativeCount } from "../valueCoerce";
import type { ProbeResult } from "../bridge";

export function limitRowsForQuickPush(rows: ProbeResult[], topN: number) {
  const normalizedTopN = nonNegativeCount(topN, 0);
  return normalizedTopN > 0 ? rows.slice(0, normalizedTopN) : rows;
}
