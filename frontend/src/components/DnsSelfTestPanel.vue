<script setup vapor lang="ts">
import { computed, ref, watch } from "vue";
import { PhArrowsClockwise, PhCheckCircle, PhFlask, PhMinusCircle, PhXCircle } from "@phosphor-icons/vue";
import type { DnsSelfTestProviderOption, DnsSelfTestResult, DnsSelfTestStage } from "../lib/bridge";

const { providers, running, result, compact } = defineProps<{
  providers: DnsSelfTestProviderOption[];
  running: boolean;
  result: DnsSelfTestResult | null;
  compact?: boolean;
}>();

const emit = defineEmits<{
  (event: "run", payload: { providerId: string; testName: string }): void;
}>();

const providerId = ref(providers[0]?.id ?? "");
const testName = ref(defaultTestName(providers[0]));
const confirmed = ref(false);

function defaultTestName(provider: DnsSelfTestProviderOption | undefined) {
  const base = provider?.recordName.trim().replace(/\.$/, "").toLowerCase() ?? "";
  return base ? `cfst-selftest.${base}` : "";
}

const selectedProvider = computed(() => providers.find((provider) => provider.id === providerId.value));
const normalizedName = computed(() => testName.value.trim().replace(/\.$/, "").toLowerCase());
const baseName = computed(() => selectedProvider.value?.recordName.trim().replace(/\.$/, "").toLowerCase() ?? "");
const nameProblem = computed(() => {
  if (!selectedProvider.value) {
    return "请先选择 DNS 推送平台";
  }
  if (!baseName.value) {
    return "当前平台没有填写推送记录名，无法自检";
  }
  if (!normalizedName.value) {
    return "请填写测试记录名";
  }
  if (normalizedName.value === baseName.value) {
    return "测试记录名不能等于推送记录名";
  }
  if (!normalizedName.value.endsWith(`.${baseName.value}`)) {
    return `测试记录名必须是 ${baseName.value} 的子域名`;
  }
  if (normalizedName.value.includes("*")) {
    return "测试记录名不能包含通配符";
  }
  const label = normalizedName.value.slice(0, -(baseName.value.length + 1));
  if (label !== "cfst-selftest" && !label.startsWith("cfst-selftest.")) {
    return "测试记录名必须以 cfst-selftest 开头，避免误用生产子域名";
  }
  if (label.endsWith(".")) {
    return "测试记录名格式无效";
  }
  return "";
});
const canRun = computed(() => !running && !nameProblem.value && confirmed.value);

watch(
  () => providers.map((provider) => `${provider.id}:${provider.recordName}`).join("|"),
  () => {
    if (!providers.some((provider) => provider.id === providerId.value)) {
      providerId.value = providers[0]?.id ?? "";
    }
    testName.value = defaultTestName(selectedProvider.value);
    confirmed.value = false;
  },
);

watch(providerId, () => {
  testName.value = defaultTestName(selectedProvider.value);
  confirmed.value = false;
});

function runSelfTest() {
  if (!canRun.value || !selectedProvider.value) {
    return;
  }
  emit("run", { providerId: providerId.value, testName: normalizedName.value });
}

function stageClass(stage: DnsSelfTestStage) {
  switch (stage.status) {
    case "passed":
      return "border-emerald-100 bg-emerald-50/70";
    case "failed":
      return "border-rose-100 bg-rose-50/70";
    default:
      return "border-slate-200 bg-slate-50/70";
  }
}

function stageIconClass(stage: DnsSelfTestStage) {
  switch (stage.status) {
    case "passed":
      return "text-emerald-600";
    case "failed":
      return "text-rose-600";
    default:
      return "text-slate-400";
  }
}

function statusLabel(stage: DnsSelfTestStage) {
  switch (stage.status) {
    case "passed":
      return "通过";
    case "failed":
      return "失败";
    default:
      return "跳过";
  }
}
</script>

<template>
  <article class="ui-card overflow-hidden">
    <div class="border-b border-slate-200 bg-slate-50/70 px-5 py-4">
      <div class="flex flex-wrap items-start justify-between gap-4">
        <div class="min-w-0">
          <h3 class="flex items-center text-base font-semibold text-slate-800">
            <PhFlask class="mr-2 text-primary" size="20" />
            DNS 平台自检
          </h3>
          <p class="mt-1 text-sm text-slate-500">读取、上传、幂等、换血、属性更新与 CNAME 拦截/替换，全部只落在测试记录名上，跑完自动清理。</p>
        </div>
        <span class="ui-pill ui-pill-subtle">{{ running ? "执行中" : "不会触碰推送记录名" }}</span>
      </div>
    </div>

    <div class="space-y-4 p-5">
      <div class="grid gap-3" :class="compact ? '' : 'md:grid-cols-2'">
        <label class="block">
          <span class="ui-label">测试平台</span>
          <select v-model="providerId" class="ui-field" :disabled="running || providers.length === 0">
            <option v-for="provider in providers" :key="provider.id" :value="provider.id">{{ provider.label }}</option>
          </select>
        </label>
        <label class="block">
          <span class="ui-label">测试记录名</span>
          <input v-model="testName" type="text" class="ui-field font-mono" :disabled="running" placeholder="cfst-selftest.example.com" />
        </label>
      </div>

      <p v-if="nameProblem" class="text-xs text-amber-600">{{ nameProblem }}</p>
      <p v-else class="text-xs text-slate-500">只在 {{ normalizedName }} 上写入 A / AAAA / CNAME；TXT、MX 等非托管记录与同 zone 其它名字不会被改动。</p>

      <div class="rounded-xl border border-amber-200 bg-amber-50/80 px-4 py-3 text-xs leading-relaxed text-amber-800">自检会真实调用平台 API，并临时写入、更新、删除上述测试记录名下的记录。若中途失败，会在阶段详情里列出残留记录，按 id 手动清理即可。</div>

      <div class="flex flex-wrap items-center justify-between gap-3">
        <label class="flex min-w-0 items-center gap-2 text-sm text-slate-600">
          <input v-model="confirmed" type="checkbox" class="h-4 w-4 accent-sky-600" :disabled="running" />
          我确认在测试记录名上执行真实写入
        </label>
        <button type="button" class="ui-button ui-button-cf" :disabled="!canRun" @click="runSelfTest">
          <PhArrowsClockwise v-if="running" class="animate-spin" size="16" />
          <PhFlask v-else size="16" />
          {{ running ? "自检执行中" : "开始自检" }}
        </button>
      </div>

      <div v-if="result" class="space-y-3 border-t border-slate-200 pt-4">
        <div class="flex flex-wrap items-center gap-2 text-sm">
          <span class="font-semibold text-slate-800">{{ result.test_name }}</span>
          <span class="ui-pill bg-emerald-50 text-emerald-700">通过 {{ result.passed }}</span>
          <span v-if="result.failed > 0" class="ui-pill bg-rose-50 text-rose-700">失败 {{ result.failed }}</span>
          <span v-if="result.skipped > 0" class="ui-pill bg-slate-100 text-slate-600">跳过 {{ result.skipped }}</span>
          <span class="ui-pill" :class="result.wrote_records ? (result.cleanup_ok ? 'bg-sky-50 text-sky-700' : 'bg-rose-50 text-rose-700') : 'bg-slate-100 text-slate-600'">
            {{ result.wrote_records ? (result.cleanup_ok ? "已清理测试记录" : "测试记录可能残留") : "未写入测试记录" }}
          </span>
        </div>

        <div class="space-y-2">
          <div v-for="stage in result.stages" :key="stage.id" class="rounded-xl border px-3 py-2.5" :class="stageClass(stage)">
            <div class="flex items-start gap-2.5">
              <PhCheckCircle v-if="stage.status === 'passed'" class="mt-0.5 shrink-0" :class="stageIconClass(stage)" size="18" />
              <PhXCircle v-else-if="stage.status === 'failed'" class="mt-0.5 shrink-0" :class="stageIconClass(stage)" size="18" />
              <PhMinusCircle v-else class="mt-0.5 shrink-0" :class="stageIconClass(stage)" size="18" />
              <div class="min-w-0 flex-1">
                <div class="flex flex-wrap items-center gap-2">
                  <span class="text-sm font-semibold text-slate-800">{{ stage.title }}</span>
                  <span class="text-xs text-slate-500">{{ statusLabel(stage) }}</span>
                  <span v-if="stage.duration_ms > 0" class="text-xs text-slate-400">{{ stage.duration_ms }} ms</span>
                </div>
                <p class="mt-1 break-words text-xs leading-relaxed text-slate-600">{{ stage.message }}</p>
              </div>
            </div>
          </div>
        </div>

        <div v-if="result.warnings.length > 0" class="rounded-xl border border-slate-200 bg-slate-50/70 px-4 py-3">
          <p class="text-xs font-semibold text-slate-700">警告</p>
          <ul class="mt-1 space-y-1 text-xs text-slate-600">
            <li v-for="warning in result.warnings" :key="warning" class="break-words">· {{ warning }}</li>
          </ul>
        </div>

        <p v-if="result.wrote_records && result.failed > 0 && !result.cleanup_ok" class="text-xs text-rose-600">清理未完成，测试记录可能仍在线上；请按失败阶段里的记录 id 手动删除，避免留下测试解析。</p>
      </div>
    </div>
  </article>
</template>
