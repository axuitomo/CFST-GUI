// 活动 feed（重构计划 §4.2）：从 App.vue 原样迁出；合并与上限逻辑见 lib/activityFeed。
import { ref } from "vue";
import { appendActivity, type ActivityEntry } from "../lib/activityFeed";

export function useActivityFeed() {
  const activityFeed = ref<ActivityEntry[]>([]);

  function pushActivity(title: string, detail: string) {
    activityFeed.value = appendActivity(activityFeed.value, { detail, title, ts: new Date().toISOString() });
  }

  return { activityFeed, pushActivity };
}
