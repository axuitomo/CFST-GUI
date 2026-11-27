import { ref, watch } from "vue";
import type { SettingsSectionKey } from "./types";

const EXPANDED_SECTIONS_STORAGE_KEY = "cfst.settings.expandedSections.v1";

function loadExpandedSections(): Record<SettingsSectionKey, boolean> {
  const sections: Record<SettingsSectionKey, boolean> = {
    appearance: false,
    backup: false,
    cloudflare: false,
    debug: false,
    dns: false,
    export: false,
    github: false,
    postPush: false,
    probe: false,
    protection: false,
    scheduler: false,
    sources: false,
    colo: false,
    storage: false,
    telegram: false,
    tls: false,
    upload: false,
    updates: false,
    viewport: false,
    webhook: false,
  };
  try {
    const raw = window.localStorage.getItem(EXPANDED_SECTIONS_STORAGE_KEY);
    const parsed = raw ? (JSON.parse(raw) as Record<string, unknown>) : null;
    if (parsed) {
      for (const key of Object.keys(sections) as SettingsSectionKey[]) {
        if (typeof parsed[key] === "boolean") {
          sections[key] = parsed[key];
        }
      }
    }
  } catch {
    // 存储不可用时从全收起开始，不影响设置页使用。
  }
  return sections;
}

export function useSettingsSections() {
  // 展开状态跨导航保存：设置页每次进入都会重新挂载，否则用户刚展开的分区会被重置。
  const expandedSections = ref<Record<SettingsSectionKey, boolean>>(loadExpandedSections());
  watch(
    expandedSections,
    (value) => {
      try {
        window.localStorage.setItem(EXPANDED_SECTIONS_STORAGE_KEY, JSON.stringify(value));
      } catch {
        // Ignore storage failures so the settings page stays usable.
      }
    },
    { deep: true },
  );

  // 折叠的分区不构建表单内容：进入设置页只挂摘要，首次展开才创建，之后保持挂载。
  const mountedSections = ref(new Set<SettingsSectionKey>());
  const pinnedHelpSection = ref<string | null>(null);

  function toggleHelp(key: string) {
    pinnedHelpSection.value = pinnedHelpSection.value === key ? null : key;
  }

  function isOpen(section: SettingsSectionKey) {
    return expandedSections.value[section];
  }

  function isMounted(section: SettingsSectionKey) {
    return mountedSections.value.has(section) || expandedSections.value[section];
  }

  function syncOpen(section: SettingsSectionKey, event: Event) {
    const element = event.currentTarget as HTMLDetailsElement;
    const open = element.open;
    if (open === expandedSections.value[section]) {
      // 挂载时浏览器会把已恢复的 open 属性再回放一次 toggle，那不是用户操作。
      return;
    }
    if (open) mountedSections.value.add(section);
    expandedSections.value[section] = open;
  }

  return {
    isMounted,
    isOpen,
    pinnedHelpSection,
    syncOpen,
    toggleHelp,
  };
}
