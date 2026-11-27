<script setup vapor lang="ts">
import { computed } from "vue";
import "../components/settings/settings-theme.css";
import "../components/settings/settings-layout.css";
import type { AndroidBatteryStatus, AndroidKeepAliveStatus, AndroidNotificationPermissionStatus, AppInfo, ColoDictionaryStatus, DNSSubdomainSnapshot, SchedulerStatus, StorageStatus } from "../lib/bridge";
import type { SettingsForm } from "../lib/config/settingsForm";
import type { ViewportPreset, ViewportSize } from "../lib/app/types";
import type { SettingsSectionController, SettingsTimestampFormatOptions, SettingsUpdateState } from "../components/settings/types";
import { useSettingsSections } from "../components/settings/useSettingsSections";
import SettingsAutomationDomain from "../components/settings/SettingsAutomationDomain.vue";
import SettingsGeneralDomain from "../components/settings/SettingsGeneralDomain.vue";
import SettingsNetworkDomain from "../components/settings/SettingsNetworkDomain.vue";
import SettingsNotifyDomain from "../components/settings/SettingsNotifyDomain.vue";
import SettingsSecurityDomain from "../components/settings/SettingsSecurityDomain.vue";
import SettingsStorageDomain from "../components/settings/SettingsStorageDomain.vue";

defineProps<{
  appInfo: AppInfo;
  coloDictionaryProcessing: boolean;
  coloDictionaryStatus: ColoDictionaryStatus | null;
  coloDictionaryUpdating: boolean;
  dnsSubdomainErrors: Record<string, string>;
  dnsSubdomains: Record<string, DNSSubdomainSnapshot[]>;
  dnsSubdomainsLoading: Record<string, boolean>;
  loading: boolean;
  formatTimestamp: (value: string, options?: SettingsTimestampFormatOptions) => string;
  githubTesting: boolean;
  telegramTesting: boolean;
  maskedTokenHint: string;
  platform: "desktop" | "mobile";
  dohTesting: boolean;
  dohTestResult: { message: string; ok: boolean } | null;
  androidBatteryStatus?: AndroidBatteryStatus | null;
  androidKeepAliveStatus?: AndroidKeepAliveStatus | null;
  androidNotificationStatus?: AndroidNotificationPermissionStatus | null;
  settings: SettingsForm;
  showToken: boolean;
  schedulerStatus: SchedulerStatus | null;
  storage: StorageStatus | null;
  updateState: SettingsUpdateState;
  viewportAdaptiveActive: boolean;
  viewportPresets: ViewportPreset[];
  viewportRuntimeSupported: boolean;
  viewportSize: ViewportSize;
  viewportSwitching: boolean;
  utcOffsetLabel: string;
}>();

defineEmits<{
  (event: "apply-viewport-preset", presetId: string): void;
  (event: "open-battery-settings", mode: "request" | "settings" | "details"): void;
  (event: "set-keep-alive-enabled", enabled: boolean): void;
  (event: "open-notification-settings"): void;
  (event: "request-notification-permission"): void;
  (event: "backup-config-webdav"): void;
  (event: "check-storage-health"): void;
  (event: "check-update"): void;
  (event: "export-config"): void;
  (event: "export-debug-log"): void;
  (event: "export-diagnostic-package"): void;
  (event: "import-config"): void;
  (event: "load-dns-subdomains", providerId: string): void;
  (event: "open-log-directory"): void;
  (event: "open-storage-dir"): void;
  (event: "open-release-page"): void;
  (event: "auto-save"): void;
  (event: "select-export-target"): void;
  (event: "scheduler-daily-times-blur"): void;
  (event: "restore-config-webdav"): void;
  (event: "test-webdav"): void;
  (event: "test-doh"): void;
  (event: "test-github-export"): void;
  (event: "test-telegram-notification"): void;
  (event: "toggle-token"): void;
  (event: "install-update"): void;
  (event: "process-colo-dictionary"): void;
  (event: "refresh-colo-dictionary"): void;
}>();

const { isMounted, isOpen, pinnedHelpSection, syncOpen, toggleHelp } = useSettingsSections();
const sections = computed<SettingsSectionController>(() => ({
  isMounted,
  isOpen,
  pinnedHelpSection: pinnedHelpSection.value,
  syncOpen,
  toggleHelp,
}));
</script>

<template>
  <section class="settings-view-root" :class="platform === 'desktop' ? 'space-y-4' : 'space-y-3'" @click="$emit('auto-save')" @focusout="$emit('auto-save')">
    <SettingsGeneralDomain
      :app-info="appInfo"
      :colo-dictionary-processing="coloDictionaryProcessing"
      :colo-dictionary-status="coloDictionaryStatus"
      :colo-dictionary-updating="coloDictionaryUpdating"
      :format-timestamp="formatTimestamp"
      :loading="loading"
      :platform="platform"
      :sections="sections"
      :settings="settings"
      :update-state="updateState"
      :utc-offset-label="utcOffsetLabel"
      :viewport-adaptive-active="viewportAdaptiveActive"
      :viewport-presets="viewportPresets"
      :viewport-runtime-supported="viewportRuntimeSupported"
      :viewport-size="viewportSize"
      :viewport-switching="viewportSwitching"
      @apply-viewport-preset="$emit('apply-viewport-preset', $event)"
      @check-update="$emit('check-update')"
      @install-update="$emit('install-update')"
      @open-release-page="$emit('open-release-page')"
      @process-colo-dictionary="$emit('process-colo-dictionary')"
      @refresh-colo-dictionary="$emit('refresh-colo-dictionary')"
    />

    <SettingsStorageDomain
      :format-timestamp="formatTimestamp"
      :loading="loading"
      :sections="sections"
      :settings="settings"
      :storage="storage"
      @backup-config-webdav="$emit('backup-config-webdav')"
      @check-storage-health="$emit('check-storage-health')"
      @export-config="$emit('export-config')"
      @import-config="$emit('import-config')"
      @open-storage-dir="$emit('open-storage-dir')"
      @restore-config-webdav="$emit('restore-config-webdav')"
      @test-webdav="$emit('test-webdav')"
    />

    <SettingsNetworkDomain
      :app-info="appInfo"
      :dns-subdomain-errors="dnsSubdomainErrors"
      :dns-subdomains="dnsSubdomains"
      :dns-subdomains-loading="dnsSubdomainsLoading"
      :doh-test-result="dohTestResult"
      :doh-testing="dohTesting"
      :format-timestamp="formatTimestamp"
      :github-testing="githubTesting"
      :loading="loading"
      :sections="sections"
      :settings="settings"
      @load-dns-subdomains="$emit('load-dns-subdomains', $event)"
      @test-doh="$emit('test-doh')"
      @test-github-export="$emit('test-github-export')"
    />

    <SettingsAutomationDomain :app-info="appInfo" :format-timestamp="formatTimestamp" :scheduler-status="schedulerStatus" :sections="sections" :settings="settings" @scheduler-daily-times-blur="$emit('scheduler-daily-times-blur')" @select-export-target="$emit('select-export-target')" />

    <SettingsNotifyDomain :loading="loading" :sections="sections" :settings="settings" :show-token="showToken" :telegram-testing="telegramTesting" @test-telegram-notification="$emit('test-telegram-notification')" />

    <SettingsSecurityDomain
      :android-battery-status="androidBatteryStatus"
      :android-keep-alive-status="androidKeepAliveStatus"
      :android-notification-status="androidNotificationStatus"
      :loading="loading"
      :masked-token-hint="maskedTokenHint"
      :platform="platform"
      :sections="sections"
      :settings="settings"
      @export-debug-log="$emit('export-debug-log')"
      @export-diagnostic-package="$emit('export-diagnostic-package')"
      @open-battery-settings="$emit('open-battery-settings', $event)"
      @open-log-directory="$emit('open-log-directory')"
      @open-notification-settings="$emit('open-notification-settings')"
      @request-notification-permission="$emit('request-notification-permission')"
      @set-keep-alive-enabled="$emit('set-keep-alive-enabled', $event)"
    />
  </section>
</template>
