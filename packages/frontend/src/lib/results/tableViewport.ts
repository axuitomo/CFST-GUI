// 桌面结果表格的窗口化计算（纯函数）：只渲染滚动窗口内的行，用上下占位行撑起滚动高度。
export interface TableWindowInput {
  /** 表头高度：表头在同一个滚动容器里，占位高度必须扣掉它。 */
  headerHeight: number;
  overscan: number;
  rowHeight: number;
  scrollTop: number;
  total: number;
  windowRows: number;
}

export interface TableWindow {
  bottomSpacer: number;
  start: number;
  topSpacer: number;
}

export function computeTableWindow(input: TableWindowInput): TableWindow {
  const rowHeight = input.rowHeight > 0 ? input.rowHeight : 1;
  const total = Math.max(0, input.total);
  const windowRows = Math.max(1, input.windowRows);
  const contentTop = Math.max(0, input.scrollTop - Math.max(0, input.headerHeight));
  const first = Math.floor(contentTop / rowHeight) - Math.max(0, input.overscan);
  const start = Math.min(Math.max(0, first), Math.max(0, total - 1));
  const rendered = Math.min(windowRows, total - start);
  return {
    bottomSpacer: Math.max(0, (total - start - rendered) * rowHeight),
    start,
    topSpacer: start * rowHeight,
  };
}
