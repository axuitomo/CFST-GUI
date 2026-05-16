<script setup lang="ts">
import { computed } from "vue";
import {
  PhArrowsClockwise,
  PhBroadcast,
  PhCloudArrowUp,
  PhGlobeHemisphereWest,
} from "@phosphor-icons/vue";
import type { DnsRecordSnapshot } from "../lib/bridge";

interface DnsPushSummary {
  created: number;
  deleted: number;
  hasRun: boolean;
  ignored: number;
  message: string;
  updated: number;
}

const props = defineProps<{
  dnsPushSummary: DnsPushSummary;
  dnsPushText: string;
  dnsRecords: DnsRecordSnapshot[];
  hasResultRows?: boolean;
  isLoadingDns: boolean;
  loading: boolean;
  platform: "desktop" | "mobile";
}>();

const emit = defineEmits<{
  (event: "fetch"): void;
  (event: "push"): void;
  (event: "push-current-results"): void;
  (event: "update:dnsPushText", value: string): void;
}>();

const dnsPushTextModel = computed({
  get: () => props.dnsPushText,
  set: (value: string) => emit("update:dnsPushText", value),
});

const summaryToneClass = computed(() => {
  if (!props.dnsPushSummary.hasRun) {
    return "ui-pill ui-pill-subtle";
  }

  if (props.dnsPushSummary.ignored > 0) {
    return "ui-pill ui-pill-warning";
  }

  return "ui-pill ui-pill-success";
});
</script>

<template>
  <section v-if="platform === 'desktop'" class="space-y-5">
    <div class="grid gap-4 xl:grid-cols-[minmax(0,1.2fr)_minmax(20rem,0.8fr)]">
      <article class="ui-card p-5">
        <div class="mb-3 flex flex-wrap items-center justify-between gap-4">
          <div class="min-w-0">
            <h3 class="text-base font-semibold text-slate-800">覆盖推送</h3>
            <p class="mt-1 text-sm text-slate-500">粘贴需要覆盖到 Cloudflare 的 IP 列表，支持空格、逗号和换行混排。</p>
          </div>
          <div class="flex flex-wrap items-center justify-end gap-2">
            <button type="button" class="ui-button ui-button-ghost" :disabled="isLoadingDns" @click="$emit('fetch')">
              <PhArrowsClockwise size="16" />
              读取记录
            </button>
            <button type="button" class="ui-button ui-button-secondary" :disabled="loading || !hasResultRows" @click="$emit('push-current-results')">
              <PhBroadcast size="16" />
              从当前结果推送
            </button>
            <button type="button" class="ui-button ui-button-cf" :disabled="loading" @click="$emit('push')">
              <PhCloudArrowUp size="16" />
              推送到 DNS
            </button>
          </div>
        </div>

        <textarea
          v-model="dnsPushTextModel"
          class="ui-field min-h-40 font-mono lg:min-h-32"
          placeholder="粘贴需要覆盖到 Cloudflare 的 IP 列表，支持空格、逗号和换行混排。"
        />
        <p class="mt-3 text-sm text-slate-500">这里已经接入桌面端真实调用链，不再使用原型阶段的本地模拟数据。</p>
      </article>

      <article class="ui-card p-5">
        <div class="mb-3 flex items-center justify-between gap-4">
          <div class="min-w-0">
            <h3 class="text-base font-semibold text-slate-800">最近一次推送</h3>
            <p class="mt-1 text-sm text-slate-500">创建 / 更新 / 删除 / 忽略统计会在成功返回后刷新。</p>
          </div>
          <span :class="summaryToneClass">
            {{ dnsPushSummary.hasRun ? (dnsPushSummary.ignored > 0 ? "最近部分完成" : "最近推送成功") : "未执行" }}
          </span>
        </div>

        <div class="grid grid-cols-2 gap-3">
          <article class="ui-card-subtle p-3">
            <p class="text-xs tracking-[0.14em] text-slate-500">新建</p>
            <strong class="mt-1 block text-xl font-bold text-slate-800">{{ dnsPushSummary.created }}</strong>
          </article>
          <article class="ui-card-subtle p-3">
            <p class="text-xs tracking-[0.14em] text-slate-500">更新</p>
            <strong class="mt-1 block text-xl font-bold text-slate-800">{{ dnsPushSummary.updated }}</strong>
          </article>
          <article class="ui-card-subtle p-3">
            <p class="text-xs tracking-[0.14em] text-slate-500">删除</p>
            <strong class="mt-1 block text-xl font-bold text-slate-800">{{ dnsPushSummary.deleted }}</strong>
          </article>
          <article class="ui-card-subtle p-3">
            <p class="text-xs tracking-[0.14em] text-slate-500">忽略</p>
            <strong class="mt-1 block text-xl font-bold text-slate-800">{{ dnsPushSummary.ignored }}</strong>
          </article>
        </div>

        <pre class="overflow-safe mt-3 whitespace-pre-wrap rounded-xl border border-slate-200 bg-slate-50/70 p-3 text-sm text-slate-600">{{ dnsPushSummary.message }}</pre>
      </article>
    </div>

    <article class="ui-card overflow-hidden">
      <div class="flex flex-wrap items-center justify-between gap-4 border-b border-slate-200 bg-slate-50/70 px-5 py-3">
        <div class="min-w-0">
          <h3 class="text-base font-semibold text-slate-800">线上记录</h3>
          <p class="mt-1 text-sm text-slate-500">当前配置下匹配的 Cloudflare DNS 记录快照。</p>
        </div>
        <span class="ui-pill ui-pill-subtle">{{ isLoadingDns ? "同步中..." : `${dnsRecords.length} 条记录` }}</span>
      </div>

      <div class="table-scroll">
        <table class="min-w-[52rem] text-sm">
          <thead class="bg-slate-50 text-left text-slate-500">
            <tr>
              <th class="px-4 py-2.5 font-semibold">类型</th>
              <th class="px-4 py-2.5 font-semibold">名称</th>
              <th class="px-4 py-2.5 font-semibold">内容</th>
              <th class="px-4 py-2.5 font-semibold">TTL</th>
              <th class="px-4 py-2.5 font-semibold">代理</th>
              <th class="px-4 py-2.5 font-semibold">备注</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-slate-100">
            <tr v-for="record in dnsRecords" :key="record.id" class="bg-white hover:bg-slate-50/80">
              <td class="whitespace-nowrap px-4 py-3 text-slate-600">{{ record.type }}</td>
              <td class="max-w-[13rem] truncate px-4 py-3 font-medium text-slate-800">{{ record.name }}</td>
              <td class="max-w-[13rem] truncate px-4 py-3 font-mono text-xs text-slate-700">{{ record.content }}</td>
              <td class="whitespace-nowrap px-4 py-3 text-slate-600">{{ record.ttl }}</td>
              <td class="whitespace-nowrap px-4 py-3 text-slate-600">{{ record.proxied ? "是" : "否" }}</td>
              <td class="max-w-[14rem] truncate px-4 py-3 text-slate-600">{{ record.comment || "-" }}</td>
            </tr>
            <tr v-if="dnsRecords.length === 0">
              <td colspan="6" class="px-4 py-8 text-center text-sm text-slate-400">点击“读取记录”后，这里会显示当前配置下匹配的 DNS 记录。</td>
            </tr>
          </tbody>
        </table>
      </div>
    </article>
  </section>

  <section v-else class="space-y-4">
    <article class="ui-card p-4">
      <div class="mb-3 flex items-center justify-between gap-3">
        <div class="min-w-0 flex items-center">
          <PhBroadcast class="mr-2 text-primary" size="18" weight="fill" />
          <h3 class="text-sm font-semibold text-slate-800">CF 记录推送</h3>
        </div>
        <button type="button" class="ui-button ui-button-ghost px-3 py-2 text-xs" :disabled="isLoadingDns" @click="$emit('fetch')">
          读取
        </button>
      </div>
      <textarea
        v-model="dnsPushTextModel"
        class="ui-field min-h-32 font-mono"
        placeholder="粘贴要覆盖推送的 IP 列表"
      />
      <button type="button" class="ui-button ui-button-secondary mt-3 h-11 w-full" :disabled="loading || !hasResultRows" @click="$emit('push-current-results')">
        <PhBroadcast size="16" />
        从当前结果推送
      </button>
      <button type="button" class="ui-button ui-button-cf mt-4 h-12 w-full" :disabled="loading" @click="$emit('push')">
        <PhCloudArrowUp size="18" />
        推送到 Cloudflare
      </button>
    </article>

    <article class="ui-card p-4">
      <div class="mb-4 flex items-center justify-between gap-3">
        <div class="min-w-0 flex items-center">
          <PhGlobeHemisphereWest class="mr-2 text-primary" size="18" />
          <h3 class="text-sm font-semibold text-slate-800">线上记录 ({{ dnsRecords.length }})</h3>
        </div>
        <span :class="summaryToneClass">
          {{ dnsPushSummary.hasRun ? (dnsPushSummary.ignored > 0 ? "部分完成" : "推送成功") : "未执行" }}
        </span>
      </div>

      <div v-if="dnsRecords.length === 0" class="py-8 text-center text-sm text-slate-400">
        暂无记录，请先读取。
      </div>

      <div v-else class="space-y-3">
        <article v-for="record in dnsRecords" :key="record.id" class="ui-card-subtle p-3">
          <div class="flex items-start justify-between gap-3">
            <div class="min-w-0">
              <p class="break-all font-mono text-sm font-semibold text-slate-800">{{ record.content }}</p>
              <p class="mt-1 break-all text-xs text-slate-500">{{ record.name }}</p>
            </div>
            <span class="ui-pill shrink-0" :class="record.proxied ? 'bg-emerald-50 text-emerald-700' : 'bg-slate-100 text-slate-600'">
              {{ record.proxied ? "已代理" : "直连" }}
            </span>
          </div>
          <div class="mt-3 flex flex-wrap gap-2 text-xs text-slate-500">
            <span>{{ record.type }}</span>
            <span>TTL {{ record.ttl }}</span>
            <span class="break-all">{{ record.comment || "无备注" }}</span>
          </div>
        </article>
      </div>
    </article>
  </section>
</template>
