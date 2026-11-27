<script setup vapor lang="ts">
import { computed } from "vue";
import { PhArrowsClockwise, PhBroadcast, PhBuildings, PhCaretDown, PhChatCircleText, PhEnvelopeSimple, PhLink, PhQuestion, PhTelegramLogo } from "@phosphor-icons/vue";
import type { SettingsForm } from "../../lib/config/settingsForm";
import { telegramRecipientModeText, type SettingsSectionController } from "./types";

const props = defineProps<{
  loading: boolean;
  sections: SettingsSectionController;
  settings: SettingsForm;
  showToken: boolean;
  telegramTesting: boolean;
}>();

defineEmits<{
  (event: "test-telegram-notification"): void;
}>();

const telegramChannelStatusLabel = computed(() => (props.settings.telegramNotificationEnabled ? "已启用" : "未启用"));
const telegramUploadRecipientModeLabel = computed(() => `上传目标 ${telegramRecipientModeText(props.settings.telegramUploadRecipientMode)}`);
const telegramTopNLabel = computed(() => (props.settings.telegramIncludeTopN ? `Top ${props.settings.telegramTopN || 5} ${telegramRecipientModeText(props.settings.telegramTopNRecipientMode)}` : "未推送 Top N 列表"));
</script>

<template>
  <section class="settings-domain">
    <div class="settings-domain-header">
      <div class="settings-domain-title-wrap" :class="{ pinned: sections.pinnedHelpSection === 'notify' }">
        <h3 class="settings-domain-title flex items-center">
          通知配置
          <button type="button" class="settings-help-btn" aria-label="查看说明" @click="sections.toggleHelp('notify')">
            <PhQuestion size="13" />
          </button>
        </h3>
        <p class="settings-domain-copy">Telegram、Webhook 和邮件可报告手动推送、测速后自动上传及定时任务的上传结论；也可提示任务失败。</p>
      </div>
    </div>
    <div class="overflow-hidden rounded-2xl border border-slate-200 bg-white shadow-sm">
      <details :open="sections.isOpen('telegram')" class="border-b border-slate-200 last:border-b-0" @toggle="sections.syncOpen('telegram', $event)">
        <summary class="settings-summary flex cursor-pointer items-center justify-between gap-3 bg-slate-50/70 px-4 py-2.5 transition hover:bg-slate-100/70 sm:px-5 sm:py-3 lg:px-5 lg:py-2.5">
          <h3 class="flex min-w-0 items-center text-sm font-semibold text-slate-800 sm:text-lg">
            <PhTelegramLogo class="mr-2 shrink-0 text-primary" size="20" weight="fill" />
            Telegram
          </h3>
          <div class="flex shrink-0 items-center gap-3">
            <span class="ui-pill ui-pill-subtle">{{ telegramChannelStatusLabel }}</span>
            <span class="ui-pill ui-pill-subtle">{{ telegramUploadRecipientModeLabel }}</span>
            <PhCaretDown class="text-slate-400 transition" :class="sections.isOpen('telegram') ? 'rotate-180' : ''" size="18" />
          </div>
        </summary>
        <div v-if="sections.isMounted('telegram')" class="border-t border-slate-100">
          <div class="grid gap-4 p-3 sm:p-4 lg:grid-cols-2 lg:p-4">
            <div class="space-y-4">
              <label class="flex items-start gap-3">
                <input v-model="settings.telegramNotificationEnabled" type="checkbox" class="mt-1 h-4 w-4 rounded border-slate-300 text-primary focus:ring-primary" />
                <span class="min-w-0">
                  <span class="block text-sm font-medium text-slate-700">Telegram 上传通知</span>
                  <span class="text-xs text-slate-500">默认推送上传结论。</span>
                </span>
              </label>
              <label class="block min-w-0">
                <span class="ui-label">网络代理</span>
                <select v-model="settings.telegramUseSystemProxy" class="ui-field">
                  <option :value="false">不跟随系统代理（默认）</option>
                  <option :value="true">跟随系统代理</option>
                </select>
              </label>

              <div class="space-y-3 border-t border-slate-100 pt-4">
                <h4 class="text-sm font-semibold text-slate-700">基础连接</h4>
                <label class="block min-w-0">
                  <span class="ui-label">Bot Token</span>
                  <input v-model="settings.telegramBotToken" :type="showToken ? 'text' : 'password'" class="ui-field font-mono" autocomplete="off" />
                </label>
                <label class="block min-w-0">
                  <span class="ui-label">API 地址（可选）</span>
                  <input v-model="settings.telegramAPIBaseURL" class="ui-field font-mono" autocomplete="off" placeholder="https://api.telegram.org" />
                  <span class="text-xs text-slate-500">留空使用官方地址；可填自建反代或 Bot API 网关，省略协议头时按 https 处理。</span>
                </label>
                <div class="grid gap-3 sm:grid-cols-2">
                  <label class="block min-w-0">
                    <span class="ui-label">群组/频道 Chat ID</span>
                    <input v-model="settings.telegramChatId" class="ui-field font-mono" autocomplete="off" />
                  </label>
                  <label class="block min-w-0">
                    <span class="ui-label">个人 Chat ID</span>
                    <input v-model="settings.telegramPersonalChatId" class="ui-field font-mono" autocomplete="off" />
                  </label>
                </div>
              </div>
            </div>

            <div class="space-y-4">
              <div class="space-y-3">
                <h4 class="text-sm font-semibold text-slate-700">上传结论</h4>
                <label class="block min-w-0">
                  <span class="ui-label">上传目标模式</span>
                  <select v-model="settings.telegramUploadRecipientMode" class="ui-field">
                    <option value="chat">群组/频道</option>
                    <option value="personal">仅个人</option>
                    <option value="both">个人+群组/频道</option>
                  </select>
                </label>
              </div>

              <div class="space-y-3 border-t border-slate-100 pt-4">
                <div class="flex flex-wrap items-center justify-between gap-2">
                  <h4 class="text-sm font-semibold text-slate-700">Top N 列表</h4>
                  <span v-if="settings.telegramIncludeTopN" class="ui-pill ui-pill-subtle">{{ telegramTopNLabel }}</span>
                </div>
                <label class="flex items-start gap-3">
                  <input v-model="settings.telegramIncludeTopN" type="checkbox" class="mt-1 h-4 w-4 rounded border-slate-300 text-primary focus:ring-primary" />
                  <span class="min-w-0">
                    <span class="block text-sm font-medium text-slate-700">推送 Top N 列表</span>
                    <span class="text-xs text-slate-500">使用上传筛选后的结果。</span>
                  </span>
                </label>
                <div v-if="settings.telegramIncludeTopN" class="grid gap-3 sm:grid-cols-2">
                  <label class="block min-w-0">
                    <span class="ui-label">Top N 数量</span>
                    <input v-model.number="settings.telegramTopN" min="1" max="50" type="number" class="ui-field" />
                  </label>
                  <label class="block min-w-0">
                    <span class="ui-label">Top N 目标模式</span>
                    <select v-model="settings.telegramTopNRecipientMode" class="ui-field">
                      <option value="chat">群组/频道</option>
                      <option value="personal">仅个人</option>
                      <option value="both">个人+群组/频道</option>
                    </select>
                  </label>
                </div>
                <p v-else class="text-xs text-slate-500">未推送 Top N 列表</p>
              </div>
            </div>
          </div>

          <div class="flex flex-col gap-3 border-t border-slate-100 px-4 py-4 sm:flex-row sm:items-center sm:justify-between sm:px-6 lg:px-5">
            <p class="text-xs text-slate-500">手动推送、测速后自动上传和定时任务自动上传共用此渠道。</p>
            <button type="button" class="ui-button ui-button-secondary w-full sm:w-auto" :disabled="loading || telegramTesting" @click="$emit('test-telegram-notification')">
              <PhArrowsClockwise size="18" />
              {{ telegramTesting ? "测试中" : "测试 Telegram" }}
            </button>
          </div>
        </div>
      </details>

      <details :open="sections.isOpen('webhook')" class="border-b border-slate-200 last:border-b-0" @toggle="sections.syncOpen('webhook', $event)">
        <summary class="settings-summary flex cursor-pointer items-center justify-between gap-3 bg-slate-50/70 px-4 py-2.5 transition hover:bg-slate-100/70 sm:px-5 sm:py-3 lg:px-5 lg:py-2.5">
          <h3 class="flex min-w-0 items-center text-sm font-semibold text-slate-800 sm:text-lg">
            <PhBroadcast class="mr-2 shrink-0 text-primary" size="20" weight="fill" />
            其他通知 Webhook
          </h3>
          <div class="flex shrink-0 items-center gap-3">
            <span class="ui-pill ui-pill-subtle">{{ settings.webhookEnabled ? "已启用" : "未启用" }}</span>
            <PhCaretDown class="text-slate-400 transition" :class="sections.isOpen('webhook') ? 'rotate-180' : ''" size="18" />
          </div>
        </summary>
        <div v-if="sections.isMounted('webhook')" class="border-t border-slate-100">
          <div class="space-y-4 p-3 sm:p-4 lg:p-5">
            <div class="grid gap-4 rounded-xl border border-slate-200 bg-slate-50/60 p-4 sm:grid-cols-[minmax(0,1fr)_minmax(15rem,0.8fr)]">
              <label class="flex items-start gap-3">
                <input v-model="settings.webhookEnabled" type="checkbox" class="mt-1 h-4 w-4 rounded border-slate-300 text-primary focus:ring-primary" />
                <span class="min-w-0"><span class="block text-sm font-medium text-slate-700">启用其他通知 Webhook</span><span class="mt-1 block text-xs text-slate-500">可同时发送到通用地址、钉钉和企业微信。</span></span>
              </label>
              <label class="block min-w-0">
                <span class="ui-label">网络代理</span>
                <select v-model="settings.webhookUseSystemProxy" class="ui-field">
                  <option :value="false">不跟随系统代理（默认）</option>
                  <option :value="true">跟随系统代理</option>
                </select>
              </label>
            </div>

            <div class="rounded-xl border border-slate-200 bg-white p-4">
              <div class="mb-3 flex items-center gap-2">
                <PhLink class="text-primary" size="18" />
                <h4 class="text-sm font-semibold text-slate-700">通用 Webhook</h4>
              </div>
              <div class="grid gap-3">
                <label class="block"><span class="ui-label">Webhook 地址</span><span class="mb-1 block text-xs text-slate-500">每行一个地址，可配置多个接收端。</span><textarea v-model="settings.webhookURLs" rows="3" class="ui-field font-mono" placeholder="https://example.com/webhook"></textarea></label>
                <label class="block"><span class="ui-label">自定义请求头（JSON）</span><textarea v-model="settings.webhookHeadersJSON" rows="3" class="ui-field font-mono" placeholder='{"Authorization":"Bearer ..."}'></textarea></label>
              </div>
            </div>

            <div class="grid gap-4 lg:grid-cols-2">
              <div class="rounded-xl border border-slate-200 bg-white p-4">
                <div class="mb-3 flex items-center gap-2">
                  <PhBroadcast class="text-sky-600" size="18" />
                  <h4 class="text-sm font-semibold text-slate-700">钉钉机器人</h4>
                </div>
                <div class="space-y-3">
                  <label class="block"><span class="ui-label">机器人地址</span><input v-model="settings.webhookDingTalkURL" class="ui-field font-mono" autocomplete="off" placeholder="https://oapi.dingtalk.com/robot/send?..." /></label>
                  <label class="block"><span class="ui-label">加签密钥（可选）</span><input v-model="settings.webhookDingTalkSecret" type="password" class="ui-field font-mono" autocomplete="off" /></label>
                </div>
              </div>
              <div class="rounded-xl border border-slate-200 bg-white p-4">
                <div class="mb-3 flex items-center gap-2">
                  <PhChatCircleText class="text-emerald-600" size="18" />
                  <h4 class="text-sm font-semibold text-slate-700">企业微信机器人</h4>
                </div>
                <label class="block"><span class="ui-label">机器人地址</span><span class="mb-1 block text-xs text-slate-500">每行一个地址。</span><textarea v-model="settings.webhookWeComURLs" rows="3" class="ui-field font-mono" placeholder="https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=..."></textarea></label>
              </div>
            </div>

            <div class="rounded-xl border border-slate-200 bg-white p-4">
              <div class="mb-3 flex items-center gap-2">
                <PhBuildings class="text-emerald-600" size="18" />
                <h4 class="text-sm font-semibold text-slate-700">企业微信应用消息</h4>
                <span class="text-xs text-slate-500">手机号目标</span>
              </div>
              <div class="grid gap-3 sm:grid-cols-2">
                <label class="block"><span class="ui-label">企业 ID</span><input v-model="settings.webhookWeComCorpID" class="ui-field" /></label>
                <label class="block"><span class="ui-label">应用 AgentID</span><input v-model="settings.webhookWeComAgentID" class="ui-field" /></label>
                <label class="block"><span class="ui-label">应用 Secret</span><input v-model="settings.webhookWeComSecret" type="password" class="ui-field" /></label>
                <label class="block"><span class="ui-label">手机号</span><input v-model="settings.webhookWeComMobiles" class="ui-field" placeholder="每行一个手机号" /></label>
              </div>
            </div>

            <div class="rounded-xl border border-slate-200 bg-white p-4">
              <div class="mb-3 flex items-center gap-2">
                <PhEnvelopeSimple class="text-amber-600" size="18" />
                <h4 class="text-sm font-semibold text-slate-700">邮件通知</h4>
              </div>
              <div class="grid gap-3 sm:grid-cols-2">
                <label class="flex items-center gap-2 sm:col-span-2"><input v-model="settings.emailEnabled" type="checkbox" class="h-4 w-4 rounded border-slate-300 text-primary focus:ring-primary" />启用邮件通知</label>
                <label class="block"><span class="ui-label">SMTP 主机</span><input v-model="settings.emailHost" class="ui-field" /></label>
                <label class="block"><span class="ui-label">端口</span><input v-model.number="settings.emailPort" type="number" class="ui-field" /></label>
                <label class="block"><span class="ui-label">用户名</span><input v-model="settings.emailUsername" class="ui-field" /></label>
                <label class="block"><span class="ui-label">密码</span><input v-model="settings.emailPassword" type="password" class="ui-field" /></label>
                <label class="block"><span class="ui-label">发件人</span><input v-model="settings.emailFrom" class="ui-field" /></label>
                <label class="block"><span class="ui-label">收件人</span><input v-model="settings.emailTo" class="ui-field" placeholder="每行一个邮箱" /></label>
              </div>
            </div>
          </div>
        </div>
      </details>
    </div>
  </section>
</template>
