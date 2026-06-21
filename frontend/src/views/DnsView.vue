<script setup lang="ts">
import { PhArrowsClockwise, PhFunnel, PhGlobeHemisphereWest, PhMagnifyingGlass } from "@phosphor-icons/vue";
import type { DnsRecordSnapshot } from "../lib/bridge";

type DnsReadScope = "zone" | "configured" | "custom";
type DnsRecordTypeFilter = "all" | "A" | "AAAA";

const dnsReadName = defineModel<string>("dnsReadName", { required: true });
const dnsReadScope = defineModel<DnsReadScope>("dnsReadScope", { required: true });
const dnsRecordType = defineModel<DnsRecordTypeFilter>("dnsRecordType", { required: true });

const { dnsRecords, isLoadingDns, platform } = defineProps<{
  dnsRecords: DnsRecordSnapshot[];
  isLoadingDns: boolean;
  platform: "desktop" | "mobile";
}>();

const emit = defineEmits<{
  (event: "fetch"): void;
}>();

const scopeOptions: Array<{ copy: string; label: string; value: DnsReadScope }> = [
  { copy: "读取当前 Zone 下全部 DNS 记录", label: "当前域名全部记录", value: "zone" },
  { copy: "读取 Cloudflare 配置里的记录名", label: "当前配置记录", value: "configured" },
  { copy: "读取你输入的指定子域名记录", label: "指定子域名", value: "custom" },
];

const typeOptions: Array<{ label: string; value: DnsRecordTypeFilter }> = [
  { label: "全部类型", value: "all" },
  { label: "仅 A", value: "A" },
  { label: "仅 AAAA", value: "AAAA" },
];
</script>

<template>
  <section v-if="platform === 'desktop'" class="space-y-5">
    <article class="ui-card overflow-hidden">
      <div class="border-b border-slate-200 bg-slate-50/70 px-5 py-4">
        <div class="flex flex-wrap items-start justify-between gap-4">
          <div class="min-w-0">
            <h3 class="flex items-center text-base font-semibold text-slate-800">
              <PhGlobeHemisphereWest class="mr-2 text-primary" size="20" />
              DNS 记录读取
            </h3>
            <p class="mt-1 text-sm text-slate-500">通过 Cloudflare 官方 API 读取当前 Zone 或指定记录名下的 DNS 记录；此页面不执行推送。</p>
          </div>
          <button type="button" class="ui-button ui-button-cf" :disabled="isLoadingDns || (dnsReadScope === 'custom' && !dnsReadName.trim())" @click="emit('fetch')">
            <PhArrowsClockwise size="16" />
            {{ isLoadingDns ? "读取中" : "读取记录" }}
          </button>
        </div>
      </div>

      <div class="grid gap-4 p-5 lg:grid-cols-[minmax(0,1fr)_minmax(16rem,0.42fr)]">
        <div class="space-y-4">
          <div class="grid gap-3 md:grid-cols-3">
            <button
              v-for="option in scopeOptions"
              :key="option.value"
              type="button"
              class="rounded-xl border px-4 py-3 text-left transition"
              :class="dnsReadScope === option.value ? 'border-primary bg-primary/10 text-slate-900 shadow-sm' : 'border-slate-200 bg-white text-slate-600 hover:border-slate-300'"
              @click="dnsReadScope = option.value"
            >
              <span class="block text-sm font-semibold">{{ option.label }}</span>
              <span class="mt-1 block text-xs text-slate-500">{{ option.copy }}</span>
            </button>
          </div>

          <label v-if="dnsReadScope === 'custom'" class="block">
            <span class="ui-label">指定子域名 / 记录名</span>
            <div class="relative">
              <PhMagnifyingGlass class="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-slate-400" size="16" />
              <input v-model="dnsReadName" type="text" class="ui-field pl-9 font-mono" placeholder="sub.example.com" />
            </div>
            <p class="mt-2 text-xs text-slate-500">Cloudflare API 的 DNS Records 列表接口会按完整记录名精确筛选。</p>
          </label>
        </div>

        <div class="rounded-xl border border-slate-200 bg-slate-50/70 p-4">
          <div class="mb-3 flex items-center gap-2 text-sm font-semibold text-slate-700">
            <PhFunnel size="17" />
            筛选
          </div>
          <label>
            <span class="ui-label">记录类型</span>
            <select v-model="dnsRecordType" class="ui-field">
              <option v-for="option in typeOptions" :key="option.value" :value="option.value">{{ option.label }}</option>
            </select>
          </label>
          <p class="mt-3 text-xs text-slate-500">默认读取所有类型；选择 A/AAAA 时会把类型条件直接传给 Cloudflare 官方 API。</p>
        </div>
      </div>
    </article>

    <article class="ui-card overflow-hidden">
      <div class="flex flex-wrap items-center justify-between gap-4 border-b border-slate-200 bg-slate-50/70 px-5 py-3">
        <div class="min-w-0">
          <h3 class="text-base font-semibold text-slate-800">线上记录</h3>
          <p class="mt-1 text-sm text-slate-500">当前读取条件下匹配的 Cloudflare DNS 记录快照。</p>
        </div>
        <span class="ui-pill ui-pill-subtle">{{ isLoadingDns ? "同步中..." : `${dnsRecords.length} 条记录` }}</span>
      </div>

      <div class="table-scroll desktop-data-table-scroll">
        <table class="desktop-data-table desktop-dns-table text-sm">
          <thead class="text-left text-slate-500">
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
            <tr v-for="record in dnsRecords" :key="record.id || `${record.type}-${record.name}-${record.content}`" class="desktop-data-row">
              <td class="whitespace-nowrap px-4 py-3 text-slate-600">{{ record.type }}</td>
              <td class="max-w-[13rem] truncate px-4 py-3 font-medium text-slate-800">{{ record.name }}</td>
              <td class="max-w-[13rem] truncate px-4 py-3 font-mono text-xs text-slate-700">{{ record.content }}</td>
              <td class="whitespace-nowrap px-4 py-3 text-slate-600">{{ record.ttl }}</td>
              <td class="whitespace-nowrap px-4 py-3 text-slate-600">{{ record.proxied ? "是" : "否" }}</td>
              <td class="max-w-[14rem] truncate px-4 py-3 text-slate-600">{{ record.comment || "-" }}</td>
            </tr>
            <tr v-if="dnsRecords.length === 0">
              <td colspan="6" class="px-4 py-8 text-center text-sm text-slate-400">点击“读取记录”后，这里会显示 Cloudflare API 返回的 DNS 记录。</td>
            </tr>
          </tbody>
        </table>
      </div>
    </article>
  </section>

  <section v-else class="space-y-4">
    <article class="ui-card p-4">
      <div class="mb-4 flex items-start justify-between gap-3">
        <div class="min-w-0">
          <h3 class="flex items-center text-sm font-semibold text-slate-800">
            <PhGlobeHemisphereWest class="mr-2 text-primary" size="18" />
            DNS 记录读取
          </h3>
          <p class="mt-1 text-xs text-slate-500">只读取 Cloudflare 官方 API 记录，不执行推送。</p>
        </div>
        <button type="button" class="ui-button ui-button-cf px-3 py-2 text-xs" :disabled="isLoadingDns || (dnsReadScope === 'custom' && !dnsReadName.trim())" @click="emit('fetch')">
          {{ isLoadingDns ? "读取中" : "读取" }}
        </button>
      </div>

      <div class="space-y-3">
        <button v-for="option in scopeOptions" :key="option.value" type="button" class="w-full rounded-xl border px-3 py-2.5 text-left transition" :class="dnsReadScope === option.value ? 'border-primary bg-primary/10 text-slate-900' : 'border-slate-200 bg-white text-slate-600'" @click="dnsReadScope = option.value">
          <span class="block text-sm font-semibold">{{ option.label }}</span>
          <span class="mt-1 block text-xs text-slate-500">{{ option.copy }}</span>
        </button>
      </div>

      <label v-if="dnsReadScope === 'custom'" class="mt-4 block">
        <span class="ui-label">指定子域名 / 记录名</span>
        <input v-model="dnsReadName" type="text" class="ui-field font-mono" placeholder="sub.example.com" />
      </label>

      <label class="mt-4 block">
        <span class="ui-label">记录类型</span>
        <select v-model="dnsRecordType" class="ui-field">
          <option v-for="option in typeOptions" :key="option.value" :value="option.value">{{ option.label }}</option>
        </select>
      </label>
    </article>

    <article class="ui-card p-4">
      <div class="mb-4 flex items-center justify-between gap-3">
        <h3 class="text-sm font-semibold text-slate-800">线上记录 ({{ dnsRecords.length }})</h3>
        <span class="ui-pill ui-pill-subtle">{{ isLoadingDns ? "同步中" : "只读" }}</span>
      </div>

      <div v-if="dnsRecords.length === 0" class="py-8 text-center text-sm text-slate-400">暂无记录，请先读取。</div>

      <div v-else class="space-y-3">
        <article v-for="record in dnsRecords" :key="record.id || `${record.type}-${record.name}-${record.content}`" class="ui-card-subtle p-3">
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
