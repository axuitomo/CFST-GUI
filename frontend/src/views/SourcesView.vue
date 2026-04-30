<script setup lang="ts">
import { computed } from "vue";
import { PhArrowsClockwise, PhDatabase, PhEye, PhPlus, PhTrash } from "@phosphor-icons/vue";

interface SourceEntry {
  content: string;
  enabled: boolean;
  id: string;
  ip_limit: number;
  ip_mode: "traverse" | "mcis";
  kind: "inline" | "file" | "url";
  last_fetched_at: string;
  last_fetched_count: number;
  name: string;
  path: string;
  status_text: string;
  url: string;
}

interface PreviewState {
  action: string;
  entries: string[];
  invalidCount: number;
  totalCount: number;
  updatedAt: string;
  warnings: string[];
}

const props = defineProps<{
  accepted: number;
  invalid: number;
  platform: "desktop" | "mobile";
  preparedCount: number;
  previewStates: Record<string, PreviewState | undefined>;
  requestStates: Record<string, string | undefined>;
  sources: SourceEntry[];
  taskStage: string;
}>();

defineEmits<{
  (event: "add"): void;
  (event: "fetch-source", sourceId: string): void;
  (event: "preview", sourceId: string): void;
  (event: "remove", sourceId: string): void;
}>();

const enabledCount = computed(() => props.sources.filter((source) => source.enabled).length);
const mcisCount = computed(() => props.sources.filter((source) => source.ip_mode === "mcis").length);

function sourceTypeLabel(kind: SourceEntry["kind"]) {
  if (kind === "inline") {
    return "手动输入";
  }
  if (kind === "file") {
    return "本地文件";
  }
  return "URL 列表";
}

function sourceFieldLabel(kind: SourceEntry["kind"]) {
  if (kind === "inline") {
    return "输入内容";
  }
  if (kind === "file") {
    return "文件路径";
  }
  return "URL";
}

function sourceModeCopy(mode: SourceEntry["ip_mode"]) {
  return mode === "mcis" ? "独立 MCIS 搜索引擎先探索候选，再交给当前 CFST 做最终测速" : "按顺序展开并整理来源中的候选 IP";
}

function sourceStatusText(source: SourceEntry) {
  if (!source.enabled) {
    return "已停用，启动任务时不会读取该输入源。";
  }

  if (source.status_text.trim()) {
    return source.status_text.trim();
  }

  if (source.kind === "url") {
    return "尚未抓取远程列表。";
  }

  if (source.kind === "file") {
    return "尚未读取本地文件。";
  }

  return "尚未整理手动输入。";
}

function sourcePreviewState(sourceId: string) {
  return props.previewStates[sourceId];
}

function sourceRequestState(sourceId: string) {
  return props.requestStates[sourceId] || "";
}
</script>

<template>
  <section v-if="platform === 'desktop'" class="space-y-6">
    <div class="flex items-end justify-between gap-4">
      <div>
        <h2 class="text-lg font-semibold text-slate-800">输入源管理</h2>
        <p class="mt-1 text-sm text-slate-500">输入源会跟随全局配置一起保存，每个来源都可以独立设置 IP 上限与 IP 模式。</p>
      </div>
      <button type="button" class="ui-button ui-button-secondary" @click="$emit('add')">
        <PhPlus size="18" />
        新增输入源
      </button>
    </div>

    <article class="ui-card p-6">
      <div class="grid gap-4 md:grid-cols-4">
        <div>
          <p class="text-xs uppercase tracking-[0.14em] text-slate-500">全部来源</p>
          <p class="mt-2 text-2xl font-semibold text-slate-800">{{ sources.length }}</p>
        </div>
        <div>
          <p class="text-xs uppercase tracking-[0.14em] text-slate-500">已启用</p>
          <p class="mt-2 text-2xl font-semibold text-slate-800">{{ enabledCount }}</p>
        </div>
        <div>
          <p class="text-xs uppercase tracking-[0.14em] text-slate-500">待执行来源</p>
          <p class="mt-2 text-2xl font-semibold text-slate-800">{{ preparedCount }}</p>
        </div>
        <div>
          <p class="text-xs uppercase tracking-[0.14em] text-slate-500">MCIS 来源</p>
          <p class="mt-2 text-2xl font-semibold text-slate-800">{{ mcisCount }}</p>
        </div>
      </div>
      <div class="mt-4 flex flex-wrap gap-3 text-sm text-slate-500">
        <span>当前阶段：{{ taskStage || "idle" }}</span>
        <span>任务已接受：{{ accepted }}</span>
        <span>非法条目：{{ invalid }}</span>
      </div>
    </article>

    <div v-if="sources.length === 0" class="ui-card flex flex-col items-center border-dashed px-6 py-12 text-center">
      <PhDatabase class="mb-3 text-slate-300" size="44" />
      <p class="text-slate-500">暂无输入源，任务启动前至少需要配置一个来源。</p>
      <button type="button" class="ui-button ui-button-ghost mt-5" @click="$emit('add')">添加首个来源</button>
    </div>

    <article v-for="source in sources" :key="source.id" class="ui-card p-5">
      <div class="flex items-start justify-between gap-4">
        <div class="min-w-0 flex-1">
          <label class="ui-label">名称</label>
          <input v-model="source.name" type="text" placeholder="例如：优选远程源 / 自建名单" class="ui-field" />
        </div>

        <div class="flex shrink-0 items-center gap-3 pt-6">
          <div class="flex items-center gap-3 rounded-full border border-slate-200 bg-slate-50 px-3 py-2">
            <span class="text-sm font-medium text-slate-600">{{ source.enabled ? "已启用" : "已停用" }}</span>
            <button
              type="button"
              class="relative inline-flex h-6 w-11 items-center rounded-full transition"
              :class="source.enabled ? 'bg-primary' : 'bg-slate-300'"
              @click="source.enabled = !source.enabled"
            >
              <span
                class="absolute left-[2px] top-[2px] h-5 w-5 rounded-full bg-white shadow transition"
                :class="source.enabled ? 'translate-x-5' : 'translate-x-0'"
              ></span>
            </button>
          </div>

          <button type="button" class="ui-button ui-button-ghost px-3" @click="$emit('remove', source.id)">
            <PhTrash size="18" />
          </button>
        </div>
      </div>

      <div class="mt-4 grid gap-4 md:grid-cols-3">
        <label>
          <span class="ui-label">类型</span>
          <select v-model="source.kind" class="ui-field">
            <option value="url">URL 列表</option>
            <option value="file">本地文件</option>
            <option value="inline">手动输入</option>
          </select>
        </label>
        <label>
          <span class="ui-label">IP 模式</span>
          <select v-model="source.ip_mode" class="ui-field">
            <option value="traverse">遍历</option>
            <option value="mcis">MCIS</option>
          </select>
        </label>
        <label>
          <span class="ui-label">IP 上限</span>
          <input v-model.number="source.ip_limit" min="1" type="number" class="ui-field" />
        </label>
      </div>

      <div class="mt-4">
        <label class="ui-label">{{ sourceFieldLabel(source.kind) }}</label>
        <textarea
          v-if="source.kind === 'inline'"
          v-model="source.content"
          rows="6"
          placeholder="1.1.1.1&#10;1.0.0.1&#10;104.16.0.0/16"
          class="ui-field min-h-32 font-mono"
        />
        <input
          v-else-if="source.kind === 'file'"
          v-model="source.path"
          type="text"
          placeholder="/data/cfips/ip.txt"
          class="ui-field h-11 font-mono"
        />
        <input
          v-else
          v-model="source.url"
          type="url"
          placeholder="https://example.com/ips.txt"
          class="ui-field h-11 font-mono"
        />
      </div>

      <div class="mt-4 flex flex-wrap gap-3">
        <button
          type="button"
          class="ui-button ui-button-ghost px-3"
          :disabled="Boolean(sourceRequestState(source.id))"
          @click="$emit('preview', source.id)"
        >
          <PhEye size="16" />
          {{ sourceRequestState(source.id) === "preview" ? "预览中" : "预览" }}
        </button>
        <button
          type="button"
          class="ui-button ui-button-secondary px-3"
          :disabled="Boolean(sourceRequestState(source.id))"
          @click="$emit('fetch-source', source.id)"
        >
          <PhArrowsClockwise size="16" />
          {{ sourceRequestState(source.id) === "fetch" ? "抓取中" : "抓取" }}
        </button>
      </div>

      <div class="mt-4 grid gap-3 rounded-2xl border border-slate-200 bg-slate-50/70 px-4 py-4 md:grid-cols-[minmax(0,1fr)_240px]">
        <div>
          <p class="text-xs uppercase tracking-[0.14em] text-slate-500">状态</p>
          <p class="mt-2 text-sm text-slate-700">{{ sourceStatusText(source) }}</p>
        </div>
        <div>
          <p class="text-xs uppercase tracking-[0.14em] text-slate-500">模式说明</p>
          <p class="mt-2 text-sm text-slate-700">{{ sourceModeCopy(source.ip_mode) }}</p>
        </div>
      </div>

      <div
        v-if="sourcePreviewState(source.id)"
        class="mt-4 rounded-2xl border border-slate-200 bg-white px-4 py-4"
      >
        <div class="flex flex-wrap items-center justify-between gap-3">
          <div>
            <p class="text-xs uppercase tracking-[0.14em] text-slate-500">
              {{ sourcePreviewState(source.id)?.action || "预览" }}结果
            </p>
            <p class="mt-1 text-sm text-slate-700">
              共 {{ sourcePreviewState(source.id)?.totalCount || 0 }} 条候选
              <span v-if="(sourcePreviewState(source.id)?.invalidCount || 0) > 0">
                ，忽略 {{ sourcePreviewState(source.id)?.invalidCount || 0 }} 条非法输入
              </span>
            </p>
          </div>
          <p class="text-xs text-slate-500">{{ sourcePreviewState(source.id)?.updatedAt || "" }}</p>
        </div>

        <div class="mt-3 flex flex-wrap gap-2">
          <code
            v-for="entry in sourcePreviewState(source.id)?.entries || []"
            :key="entry"
            class="rounded-lg border border-slate-200 bg-slate-50 px-2 py-1 text-xs text-slate-700"
          >
            {{ entry }}
          </code>
        </div>

        <div
          v-if="(sourcePreviewState(source.id)?.warnings || []).length > 0"
          class="mt-3 space-y-1 text-xs text-amber-600"
        >
          <p v-for="warning in sourcePreviewState(source.id)?.warnings || []" :key="warning">{{ warning }}</p>
        </div>
      </div>
    </article>
  </section>

  <section v-else class="space-y-4">
    <article class="ui-card p-4">
      <div class="flex items-center justify-between">
        <div>
          <h2 class="text-base font-semibold text-slate-800">输入源管理</h2>
          <p class="mt-1 text-xs text-slate-500">输入源会跟随全局配置保存。</p>
        </div>
        <button type="button" class="ui-button ui-button-secondary px-3" @click="$emit('add')">
          <PhPlus size="18" />
          新增
        </button>
      </div>

      <div class="mt-4 grid grid-cols-2 gap-3 text-sm">
        <div class="rounded-xl border border-slate-200 bg-slate-50 px-3 py-3">
          <p class="text-xs text-slate-500">全部来源</p>
          <p class="mt-2 text-xl font-semibold text-slate-800">{{ sources.length }}</p>
        </div>
        <div class="rounded-xl border border-slate-200 bg-slate-50 px-3 py-3">
          <p class="text-xs text-slate-500">待执行来源</p>
          <p class="mt-2 text-xl font-semibold text-slate-800">{{ preparedCount }}</p>
        </div>
      </div>
    </article>

    <div v-if="sources.length === 0" class="ui-card border-2 border-dashed p-8 text-center">
      <p class="text-slate-500">暂无输入源</p>
      <button type="button" class="ui-button ui-button-ghost mt-5" @click="$emit('add')">添加首个来源</button>
    </div>

    <article v-for="source in sources" :key="source.id" class="ui-card p-4">
      <div class="flex items-start justify-between gap-3">
        <div class="min-w-0 flex-1">
          <label class="block text-xs text-slate-500">名称</label>
          <input v-model="source.name" type="text" placeholder="输入源名称" class="ui-field h-11" />
        </div>
        <button type="button" class="ui-button ui-button-ghost px-3" @click="$emit('remove', source.id)">
          <PhTrash size="18" />
        </button>
      </div>

      <div class="mt-4 grid grid-cols-2 gap-3">
        <div>
          <label class="block text-xs text-slate-500">类型</label>
          <select v-model="source.kind" class="ui-field h-11">
            <option value="url">URL 列表</option>
            <option value="file">本地文件</option>
            <option value="inline">手动输入</option>
          </select>
        </div>
        <div>
          <label class="block text-xs text-slate-500">IP 模式</label>
          <select v-model="source.ip_mode" class="ui-field h-11">
            <option value="traverse">遍历</option>
            <option value="mcis">MCIS</option>
          </select>
        </div>
        <div>
          <label class="block text-xs text-slate-500">IP 上限</label>
          <input v-model.number="source.ip_limit" min="1" type="number" class="ui-field h-11" />
        </div>
        <div class="flex items-end justify-between rounded-xl border border-slate-200 bg-slate-50 px-3 py-3">
          <div>
            <p class="text-xs text-slate-500">启用状态</p>
            <p class="mt-1 text-sm font-medium text-slate-700">{{ source.enabled ? "已启用" : "已停用" }}</p>
          </div>
          <button
            type="button"
            class="relative inline-flex h-6 w-11 items-center rounded-full transition"
            :class="source.enabled ? 'bg-primary' : 'bg-slate-300'"
            @click="source.enabled = !source.enabled"
          >
            <span
              class="absolute left-[2px] top-[2px] h-5 w-5 rounded-full bg-white shadow transition"
              :class="source.enabled ? 'translate-x-5' : 'translate-x-0'"
            ></span>
          </button>
        </div>
      </div>

      <div class="mt-4">
        <label class="block text-xs text-slate-500">{{ sourceFieldLabel(source.kind) }}</label>
        <textarea
          v-if="source.kind === 'inline'"
          v-model="source.content"
          rows="5"
          placeholder="1.1.1.1&#10;104.16.0.0/16"
          class="ui-field mt-1 min-h-28 font-mono"
        />
        <input
          v-else-if="source.kind === 'file'"
          v-model="source.path"
          type="text"
          placeholder="/data/cfips/ip.txt"
          class="ui-field mt-1 h-11 font-mono"
        />
        <input
          v-else
          v-model="source.url"
          type="url"
          placeholder="https://example.com/ips.txt"
          class="ui-field mt-1 h-11 font-mono"
        />
      </div>

      <div class="mt-4 grid grid-cols-2 gap-3">
        <button
          type="button"
          class="ui-button ui-button-ghost px-3"
          :disabled="Boolean(sourceRequestState(source.id))"
          @click="$emit('preview', source.id)"
        >
          <PhEye size="16" />
          {{ sourceRequestState(source.id) === "preview" ? "预览中" : "预览" }}
        </button>
        <button
          type="button"
          class="ui-button ui-button-secondary px-3"
          :disabled="Boolean(sourceRequestState(source.id))"
          @click="$emit('fetch-source', source.id)"
        >
          <PhArrowsClockwise size="16" />
          {{ sourceRequestState(source.id) === "fetch" ? "抓取中" : "抓取" }}
        </button>
      </div>

      <div class="mt-4 rounded-xl border border-slate-200 bg-slate-50 px-3 py-3 text-sm text-slate-600">
        <p>{{ sourceStatusText(source) }}</p>
        <p class="mt-1 text-xs text-slate-500">模式说明：{{ sourceModeCopy(source.ip_mode) }}</p>
      </div>

      <div
        v-if="sourcePreviewState(source.id)"
        class="mt-4 rounded-xl border border-slate-200 bg-white px-3 py-3"
      >
        <p class="text-xs text-slate-500">
          {{ sourcePreviewState(source.id)?.action || "预览" }}结果 · 共 {{ sourcePreviewState(source.id)?.totalCount || 0 }} 条
        </p>
        <div class="mt-3 flex flex-wrap gap-2">
          <code
            v-for="entry in sourcePreviewState(source.id)?.entries || []"
            :key="entry"
            class="rounded-lg border border-slate-200 bg-slate-50 px-2 py-1 text-xs text-slate-700"
          >
            {{ entry }}
          </code>
        </div>
        <div
          v-if="(sourcePreviewState(source.id)?.warnings || []).length > 0"
          class="mt-3 space-y-1 text-xs text-amber-600"
        >
          <p v-for="warning in sourcePreviewState(source.id)?.warnings || []" :key="warning">{{ warning }}</p>
        </div>
      </div>

      <div class="mt-3 flex items-center justify-between text-xs text-slate-500">
        <span>{{ sourceTypeLabel(source.kind) }}</span>
        <span>{{ source.enabled ? "任务启动时参与读取" : "任务启动时跳过" }}</span>
      </div>
    </article>
  </section>
</template>
