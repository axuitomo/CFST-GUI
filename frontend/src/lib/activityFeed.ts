export interface ActivityEntry {
  /** 连续同名事件被合并的次数；1 表示这一行只发生过一次。 */
  count: number;
  detail: string;
  title: string;
  ts: string;
}

export const ACTIVITY_FEED_LIMIT = 10;

/**
 * 相邻的同名事件并成一条。
 * 探测进度会连续推同一条标题（例如 "MICS 抽样进行中"），逐条列出来会把 10 个位置挤满，
 * 真正需要注意的失败反而被顶出去。合并后保留最新的详情与时间，重复次数进入 count。
 * 只合并与前一条同名的：同一件事隔了几条别的记录再发生，仍算新的一次。
 */
export function appendActivity(feed: ActivityEntry[], next: { detail: string; title: string; ts: string }): ActivityEntry[] {
  const [latest] = feed;
  if (latest && latest.title === next.title) {
    return [{ ...next, count: latest.count + 1 }, ...feed.slice(1)];
  }
  return [{ ...next, count: 1 }, ...feed].slice(0, ACTIVITY_FEED_LIMIT);
}
