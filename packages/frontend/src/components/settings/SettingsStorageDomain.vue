<script setup vapor lang="ts">
import { computed } from "vue";
import { PhCaretDown, PhCloud, PhFileArrowUp, PhFolderOpen, PhQuestion } from "@phosphor-icons/vue";
import type { StorageStatus } from "../../lib/bridge";
import type { SettingsForm } from "../../lib/config/settingsForm";
import { formatSettingsTimestamp, type SettingsSectionController, type SettingsTimestampFormatOptions } from "./types";

const props = defineProps<{
  formatTimestamp: (value: string, options?: SettingsTimestampFormatOptions) => string;
  loading: boolean;
  sections: SettingsSectionController;
  settings: SettingsForm;
  storage: StorageStatus | null;
}>();

defineEmits<{
  (event: "backup-config-webdav"): void;
  (event: "check-storage-health"): void;
  (event: "export-config"): void;
  (event: "import-config"): void;
  (event: "open-storage-dir"): void;
  (event: "restore-config-webdav"): void;
  (event: "test-webdav"): void;
}>();

const storageHealthLabel = computed(() => {
  if (!props.storage) return "未读取";
  if (props.storage.permission_ok === false) return "权限失效";
  if ((props.storage.last_sync_error || "").trim()) return "存储状态异常";
  if (props.storage.writable) return props.storage.portable_mode ? "便携可写" : "可写";
  return "不可写";
});

const storageDisplayPath = computed(() => props.storage?.display_name || props.storage?.current_dir || "尚未读取应用数据目录");
</script>

<template>
  <section class="settings-domain">
    <div class="settings-domain-header">
      <div class="settings-domain-title-wrap" :class="{ pinned: sections.pinnedHelpSection === 'storage' }">
        <h3 class="settings-domain-title flex items-center">
          数据与存储
          <button type="button" class="settings-help-btn" aria-label="查看说明" @click="sections.toggleHelp('storage')">
            <PhQuestion size="13" />
          </button>
        </h3>
        <p class="settings-domain-copy">查看应用数据目录，管理配置导入导出和 WebDAV 手动备份。</p>
      </div>
    </div>
    <div class="overflow-hidden rounded-2xl border border-slate-200 bg-white shadow-sm">
      <details :open="sections.isOpen('storage')" class="border-b border-slate-200 last:border-b-0" @toggle="sections.syncOpen('storage', $event)">
        <summary class="settings-summary flex cursor-pointer items-center justify-between gap-3 bg-slate-50/70 px-4 py-2.5 transition hover:bg-slate-100/70 sm:px-5 sm:py-3 lg:px-5 lg:py-2.5">
          <h3 class="flex min-w-0 items-center text-sm font-semibold text-slate-800 sm:text-lg">
            <PhFolderOpen class="mr-2 shrink-0 text-slate-600" size="20" />
            应用数据目录
          </h3>
          <div class="flex shrink-0 items-center gap-3">
            <span class="ui-pill ui-pill-subtle">{{ storageHealthLabel }}</span>
            <PhCaretDown class="text-slate-400 transition" :class="sections.isOpen('storage') ? 'rotate-180' : ''" size="18" />
          </div>
        </summary>
        <div v-if="sections.isMounted('storage')" class="space-y-4 border-t border-slate-100 p-3 sm:p-4 lg:p-4">
          <div>
            <span class="ui-label">固定目录</span>
            <p class="break-all rounded-xl border border-slate-200 bg-slate-50 px-3 py-3 font-mono text-xs text-slate-600">
              {{ storageDisplayPath }}
            </p>
            <p v-if="storage?.runtime_dir && storage.runtime_dir !== storage.current_dir" class="mt-2 break-all text-xs text-slate-500">运行时目录：{{ storage.runtime_dir }}</p>
            <p v-if="storage?.last_sync_error" class="mt-2 text-xs text-amber-600">最近同步：{{ storage.last_sync_error }}</p>
            <p v-if="storage?.health?.message" class="mt-2 text-xs text-slate-500">{{ storage.health.message }}</p>
          </div>
          <div class="grid gap-2 sm:flex sm:flex-wrap sm:gap-3">
            <button type="button" class="ui-button ui-button-ghost" :disabled="loading" @click="$emit('open-storage-dir')">打开目录</button>
            <button type="button" class="ui-button ui-button-ghost" :disabled="loading" @click="$emit('check-storage-health')">健康检查</button>
          </div>
          <p class="text-xs text-slate-500">存储目录不再支持自定义；导出 CSV 和测速文件请在“结果导出”里设置目录。Android 的调试日志和诊断包会在导出时单独选择 SAF 目录。</p>
        </div>
      </details>

      <details :open="sections.isOpen('backup')" class="border-b border-slate-200 last:border-b-0" @toggle="sections.syncOpen('backup', $event)">
        <summary class="settings-summary flex cursor-pointer items-center justify-between gap-3 bg-slate-50/70 px-4 py-2.5 transition hover:bg-slate-100/70 sm:px-5 sm:py-3 lg:px-5 lg:py-2.5">
          <h3 class="flex min-w-0 items-center text-sm font-semibold text-slate-800 sm:text-lg">
            <PhCloud class="mr-2 shrink-0 text-cf" size="20" weight="fill" />
            配置备份与同步
          </h3>
          <div class="flex shrink-0 items-center gap-3">
            <span class="ui-pill ui-pill-subtle">WebDAV {{ settings.webdavEnabled ? "已启用" : "未启用" }}</span>
            <PhCaretDown class="text-slate-400 transition" :class="sections.isOpen('backup') ? 'rotate-180' : ''" size="18" />
          </div>
        </summary>
        <div v-if="sections.isMounted('backup')" class="grid gap-4 border-t border-slate-100 p-3 sm:p-4 md:grid-cols-2 lg:p-4">
          <div class="md:col-span-2 grid gap-2 sm:grid-cols-2">
            <button type="button" class="ui-button ui-button-ghost" :disabled="loading" @click="$emit('export-config')">
              <PhFileArrowUp size="18" />
              导出配置包
            </button>
            <button type="button" class="ui-button ui-button-ghost" :disabled="loading" @click="$emit('import-config')">
              <PhFileArrowUp size="18" />
              加载配置包
            </button>
          </div>

          <label class="md:col-span-2">
            <span class="ui-label">WebDAV 地址</span>
            <input v-model="settings.webdavServerURL" placeholder="https://example.com/dav/backups/" type="url" class="ui-field font-mono" />
          </label>
          <label>
            <span class="ui-label">用户名</span>
            <input v-model="settings.webdavUsername" type="text" class="ui-field" autocomplete="username" />
          </label>
          <label>
            <span class="ui-label">密码 / Token</span>
            <input v-model="settings.webdavPassword" type="password" class="ui-field" autocomplete="current-password" />
          </label>
          <label>
            <span class="ui-label">远端文件</span>
            <input v-model="settings.webdavRemotePath" placeholder="cfst-gui-config.zip" type="text" class="ui-field font-mono" />
          </label>
          <label>
            <span class="ui-label">超时（秒）</span>
            <input v-model.number="settings.webdavTimeoutSeconds" min="1" type="number" class="ui-field" />
          </label>

          <button type="button" class="md:col-span-2 flex items-center justify-between gap-4 rounded-xl border border-slate-200 bg-slate-50/70 px-4 py-3 text-left" @click="settings.webdavEnabled = !settings.webdavEnabled">
            <span class="min-w-0">
              <span class="block text-sm font-medium text-slate-700">启用 WebDAV 备份配置</span>
              <span class="text-xs text-slate-500">只影响手动测试、备份和还原，不会自动同步。</span>
            </span>
            <span class="relative inline-flex h-6 w-11 shrink-0 items-center rounded-full transition" :class="settings.webdavEnabled ? 'bg-primary' : 'bg-slate-300'">
              <span class="absolute left-[2px] top-[2px] h-5 w-5 rounded-full bg-white shadow transition" :class="settings.webdavEnabled ? 'translate-x-5' : 'translate-x-0'"></span>
            </span>
          </button>

          <div class="md:col-span-2 grid gap-2 sm:grid-cols-3">
            <button type="button" class="ui-button ui-button-ghost" :disabled="loading" @click="$emit('test-webdav')">测试 WebDAV</button>
            <button type="button" class="ui-button ui-button-primary" :disabled="loading || !settings.webdavEnabled" @click="$emit('backup-config-webdav')">备份到 WebDAV</button>
            <button type="button" class="ui-button ui-button-secondary" :disabled="loading || !settings.webdavEnabled" @click="$emit('restore-config-webdav')">从 WebDAV 还原</button>
          </div>

          <div class="md:col-span-2 rounded-xl border border-amber-200 bg-amber-50 px-4 py-3 text-xs text-amber-700">
            配置包完整包含 Cloudflare Token 和 WebDAV 凭据。最近 WebDAV 备份：{{ formatSettingsTimestamp(formatTimestamp, settings.webdavLastBackupAt, "无") }}；最近还原：{{ formatSettingsTimestamp(formatTimestamp, settings.webdavLastRestoreAt, "无") }}。
          </div>
        </div>
      </details>
    </div>
  </section>
</template>
