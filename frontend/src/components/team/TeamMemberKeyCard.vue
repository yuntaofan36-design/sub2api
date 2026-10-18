<template>
  <article class="card flex min-w-0 flex-col !rounded-lg p-5">
    <div class="flex items-start justify-between gap-3">
      <h3 class="min-w-0 break-words font-semibold text-gray-900 dark:text-white">{{ keyData.name }}</h3>
      <span class="badge shrink-0" :class="blockedReason ? 'badge-gray' : 'badge-success'">
        {{ blockedReason || t('teams.active') }}
      </span>
    </div>
    <div class="mt-2 flex flex-wrap items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
      <span class="badge badge-gray max-w-full whitespace-normal break-words">{{ keyData.group_name || t('teams.workspace.noGroup') }}</span>
      <span v-if="keyData.platform" class="inline-flex items-center gap-1"><PlatformIcon :platform="keyData.platform" size="xs" />{{ keyData.platform }}</span>
      <span>{{ t(keyData.subscription_type === 'subscription' ? 'teams.workspace.subscriptionBilling' : 'teams.workspace.balanceBilling') }}</span>
    </div>
    <div class="mt-4 flex min-w-0 items-start gap-1">
      <code class="h-12 min-w-0 flex-1 overflow-y-auto break-all py-2 text-xs text-gray-500 dark:text-gray-400">{{ revealed ? secret : keyData.masked_key }}</code>
      <button type="button" class="btn btn-ghost btn-icon !p-2 shrink-0" :title="t(revealed ? 'teams.workspace.hideKey' : 'teams.workspace.showKey')"
        :aria-label="t(revealed ? 'teams.workspace.hideKey' : 'teams.workspace.showKey')" :disabled="busy || !canReadSecret" @click="act('reveal')">
        <Icon :name="revealed ? 'eyeOff' : 'eye'" size="sm" />
      </button>
      <button type="button" class="btn btn-ghost btn-icon !p-2 shrink-0" :title="t('teams.copy')" :aria-label="t('teams.copy')"
        :disabled="busy || !canReadSecret" @click="act('copy')"><Icon name="copy" size="sm" /></button>
    </div>
    <p class="mt-2 text-xs text-gray-500 dark:text-gray-400">
      {{ keyData.expires_at ? t('teams.workspace.expiresAt', { time: new Date(keyData.expires_at).toLocaleString() }) : t('teams.workspace.noExpiry') }}
    </p>
    <div class="my-4 flex-1 space-y-4">
      <KeyQuotaProgress v-if="(keyData.quota || 0) > 0" :label="t('teams.workspace.quota')" :limit="keyData.quota!" :used="keyData.quota_used || 0" />
      <KeyQuotaProgress v-for="window in windows" :key="window.label" :label="window.label" :limit="window.limit" :used="window.used" :resets-at="window.resetsAt" />
      <p v-if="!keyData.quota && !windows.length" class="text-xs text-gray-500 dark:text-gray-400">{{ t('teams.workspace.unlimited') }}</p>
    </div>
    <div class="flex flex-wrap items-center justify-end gap-2 border-t border-gray-100 pt-4 dark:border-dark-700">
      <button type="button" class="btn btn-secondary btn-sm" @click="$emit('analyze', keyData.id)">
        <Icon name="chart" size="sm" class="mr-1.5" />{{ t('teams.workspace.analyze') }}
      </button>
      <button type="button" class="btn btn-primary btn-sm" :disabled="busy || !canReadSecret" @click="act('configure')">
        <Icon name="terminal" size="sm" class="mr-1.5" />{{ t('teams.workspace.configure') }}
      </button>
    </div>
    <UseKeyModal :show="configuring" :api-key="secret" :base-url="baseUrl" :platform="keyData.platform || null"
      :allow-messages-dispatch="keyData.allow_messages_dispatch || false" @close="closeConfiguration" />
  </article>
</template>

<script setup lang="ts">
import { computed, onUnmounted, ref, watch } from 'vue'
import { useNow } from '@vueuse/core'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import UseKeyModal from '@/components/keys/UseKeyModal.vue'
import KeyQuotaProgress from '@/components/keys/KeyQuotaProgress.vue'
import { useClipboard } from '@/composables/useClipboard'
import { useAppStore } from '@/stores/app'
import { teamsAPI, type TeamRow, type TeamOverview } from '@/api/teams'

const props = defineProps<{ keyData: TeamRow; team: TeamOverview; balance: number; baseUrl: string }>()
defineEmits<{ analyze: [id: number] }>()
const { t } = useI18n()
const app = useAppStore()
const { copyToClipboard } = useClipboard()
const now = useNow({ interval: 60_000 })
const secret = ref(''), revealed = ref(false), configuring = ref(false), busy = ref(false)
let epoch = 0
const windows = computed(() => [
  { label: t('teams.rate5h'), limit: props.keyData.rate_limit_5h || 0, used: props.keyData.usage_5h || 0, resetsAt: props.keyData.reset_5h_at },
  { label: t('teams.rate1d'), limit: props.keyData.rate_limit_1d || 0, used: props.keyData.usage_1d || 0, resetsAt: props.keyData.reset_1d_at },
  { label: t('teams.rate7d'), limit: props.keyData.rate_limit_7d || 0, used: props.keyData.usage_7d || 0, resetsAt: props.keyData.reset_7d_at }
].filter(window => window.limit > 0))
const canReadSecret = computed(() => props.team.member_status === 'active' && props.keyData.member_status === 'active' && props.team.status === 'active' && props.team.enterprise_status !== 'disabled')
const blockedReason = computed(() => {
  const key = props.keyData
  if (!canReadSecret.value || key.status !== 'active') return t('teams.disabled')
  if (key.expires_at && new Date(key.expires_at).getTime() <= now.value.getTime()) return t('teams.expired')
  if (!key.can_use_group) return t('teams.workspace.groupUnavailable')
  if ((key.quota || 0) > 0 && (key.quota_used || 0) >= key.quota!) return t('teams.workspace.quotaExhausted')
  if (windows.value.some(window => window.used >= window.limit && (!window.resetsAt || new Date(window.resetsAt).getTime() > now.value.getTime()))) return t('teams.workspace.windowExhausted')
  if (key.subscription_type !== 'subscription' && props.balance <= 0) return t('teams.workspace.balanceInsufficient')
  return ''
})
function clearSecret() {
  epoch++
  secret.value = ''
  revealed.value = false
  configuring.value = false
  busy.value = false
}
watch(() => [props.keyData.id, canReadSecret.value], clearSecret)
onUnmounted(clearSecret)
function closeConfiguration() {
  configuring.value = false
  if (!revealed.value) secret.value = ''
}
async function act(action: 'reveal' | 'copy' | 'configure') {
  if (action === 'reveal' && revealed.value) { clearSecret(); return }
  if (!canReadSecret.value || busy.value) return
  const stamp = ++epoch
  busy.value = true
  try {
    // Always authorize retrieval again, including copy and configuration actions.
    const value = await teamsAPI.secret(props.team.id, props.keyData.id)
    if (stamp !== epoch || !canReadSecret.value) return
    if (action === 'copy') await copyToClipboard(value)
    else { secret.value = value; revealed.value = action === 'reveal'; configuring.value = action === 'configure' }
  } catch (error) {
    if (stamp === epoch) app.showError((error as { message?: string }).message || t('teams.failed'))
  } finally {
    if (stamp === epoch) busy.value = false
  }
}
</script>
