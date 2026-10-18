<template>
  <section class="min-w-0 space-y-6">
    <div class="flex flex-wrap items-start justify-between gap-3 border-b border-gray-200 pb-4 dark:border-dark-700">
      <div class="min-w-0 text-sm">
        <div class="flex flex-wrap items-center gap-2">
          <span class="text-gray-500 dark:text-gray-400">{{ t('teams.workspace.personalBalance') }}</span>
          <strong class="text-lg tabular-nums">{{ money(balance) }}</strong>
          <HelpTooltip :content="t('teams.workspace.balanceHint')" />
        </div>
        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('teams.workspace.allocated', { amount: money(team.allocated_balance || 0) }) }}</p>
      </div>
      <div v-if="team.administrators?.length" class="min-w-0 text-sm">
        <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('teams.administrators') }}</p>
        <div v-for="admin in team.administrators" :key="admin.id" class="mt-1 break-all">
          <span class="text-orange-700 dark:text-orange-400">{{ admin.username || admin.email }}</span>
          <span v-if="admin.username" class="ml-2 text-xs text-gray-500 dark:text-gray-400">{{ admin.email }}</span>
        </div>
      </div>
    </div>

    <div class="flex flex-wrap items-center justify-between gap-3">
      <h3 class="font-semibold">{{ t('teams.workspace.myKeys') }} <span class="ml-2 text-sm font-normal text-gray-500">{{ keysTotal }}</span></h3>
      <div class="w-full sm:w-80"><SearchInput v-model="keySearch" :placeholder="t('teams.searchKeys')" @search="searchKeys" /></div>
    </div>
    <div v-if="keyError" class="rounded-lg bg-red-50 p-4 text-sm text-red-700 dark:bg-red-950 dark:text-red-300" role="alert">
      {{ keyError }}<button type="button" class="btn btn-ghost btn-sm ml-2" @click="loadKeys">{{ t('common.retry') }}</button>
    </div>
    <div v-if="loadingKeys" class="flex min-h-48 items-center justify-center"><LoadingSpinner /></div>
    <div v-else-if="keyRows.length" class="grid min-w-0 gap-4 lg:grid-cols-2 2xl:grid-cols-3">
      <TeamMemberKeyCard v-for="key in keyRows" :key="key.id" :key-data="key" :team="team" :balance="balance" :base-url="app.apiBaseUrl" @analyze="analyze" />
    </div>
    <EmptyState v-else-if="!keyError" :title="t('teams.workspace.noKeys')"><template #icon><Icon name="key" size="xl" /></template></EmptyState>
    <Pagination v-if="keysTotal > 0" :total="keysTotal" :page="keyPage" :page-size="keyPageSize"
      @update:page="changeKeyPage" @update:page-size="changeKeyPageSize" />

    <div ref="analysisRef" class="flex flex-wrap items-center justify-between gap-3 border-t border-gray-200 pt-6 dark:border-dark-700">
      <h3 class="font-semibold">{{ t('teams.workspace.analysis') }}</h3>
      <button type="button" class="btn btn-secondary btn-sm" @click="openRecords()"><Icon name="document" size="sm" class="mr-1.5" />{{ t('teams.workspace.viewRecords') }}</button>
    </div>
    <div class="flex flex-wrap items-center gap-3">
      <DateRangePicker v-model:start-date="startDate" v-model:end-date="endDate" @change="filtersChanged" />
      <div class="w-full sm:w-64"><Select v-model="keyID" :options="keyOptions" :aria-label="t('teams.keys')" @update:model-value="filtersChanged" /></div>
      <div class="w-28"><Select v-model="granularity" :options="granularityOptions" :aria-label="t('dashboard.granularity')" @update:model-value="loadUsage" /></div>
      <button type="button" class="btn btn-secondary btn-icon" :title="t('common.refresh')" :aria-label="t('common.refresh')" :disabled="loadingUsage" @click="loadUsage"><Icon name="refresh" size="sm" /></button>
    </div>
    <div v-if="usageError" class="rounded-lg bg-red-50 p-4 text-sm text-red-700 dark:bg-red-950 dark:text-red-300" role="alert">
      {{ usageError }}<button type="button" class="btn btn-ghost btn-sm ml-2" @click="loadUsage">{{ t('common.retry') }}</button>
    </div>
    <div v-if="loadingUsage" class="flex min-h-64 items-center justify-center"><LoadingSpinner /></div>
    <template v-else-if="usage">
      <div class="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <StatCard v-for="card in usageCards" :key="card.title" :title="card.title" :value="card.value" :icon="card.icon" />
      </div>
      <OrganizationUsageCharts :trend="usage.trend" :models="usage.models" kind="team" interactive-models @select-model="openRecords" />
    </template>
  </section>
</template>

<script setup lang="ts">
import { computed, h, nextTick, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { useAppStore } from '@/stores/app'
import SearchInput from '@/components/common/SearchInput.vue'
import Select from '@/components/common/Select.vue'
import DateRangePicker from '@/components/common/DateRangePicker.vue'
import Pagination from '@/components/common/Pagination.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import StatCard from '@/components/common/StatCard.vue'
import HelpTooltip from '@/components/common/HelpTooltip.vue'
import Icon from '@/components/icons/Icon.vue'
import OrganizationUsageCharts from '@/components/organization/OrganizationUsageCharts.vue'
import TeamMemberKeyCard from './TeamMemberKeyCard.vue'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import { formatCostFixed, formatDateLocalInput } from '@/utils/format'
import { teamsAPI, type TeamOverview, type TeamRow, type TeamMemberUsage } from '@/api/teams'

const props = defineProps<{ team: TeamOverview; availableKeys: TeamRow[] }>()
const emit = defineEmits<{ records: [filters: { keyID: number; model: string; startDate: string; endDate: string }] }>()
const { t } = useI18n()
const auth = useAuthStore(), app = useAppStore()
const balance = computed(() => auth.user?.balance || 0)
const keySearch = ref(''), searchQuery = ref(''), keyPage = ref(1), keyPageSize = ref(getPersistedPageSize()), keysTotal = ref(0)
const keyRows = ref<TeamRow[]>([]), keyError = ref(''), loadingKeys = ref(false)
const today = new Date(), firstDay = new Date(today)
firstDay.setDate(firstDay.getDate() - 6)
const startDate = ref(formatDateLocalInput(firstDay)), endDate = ref(formatDateLocalInput(today))
const granularity = ref<'day' | 'hour'>('day'), keyID = ref(0)
const usage = ref<TeamMemberUsage | null>(null), usageError = ref(''), loadingUsage = ref(false)
const analysisRef = ref<HTMLElement | null>(null)
let keysController: AbortController | undefined, usageController: AbortController | undefined
let keyEpoch = 0, usageEpoch = 0
const money = (value: number) => `$${formatCostFixed(value)}`
const keyOptions = computed(() => [{ value: 0, label: t('teams.workspace.allMyKeys') }, ...props.availableKeys.map(key => ({ value: key.id, label: key.name || `#${key.id}` }))])
const granularityOptions = computed(() => [{ value: 'day', label: t('dashboard.day') }, { value: 'hour', label: t('dashboard.hour') }])
const usageCards = computed(() => {
  const metrics = usage.value!.usage
  return [
    { title: t('teams.requests'), value: metrics.requests, icon: () => h(Icon, { name: 'bolt' }) },
    { title: t('teams.tokens'), value: metrics.tokens, icon: () => h(Icon, { name: 'chart' }) },
    { title: t('teams.cost'), value: money(metrics.cost), icon: () => h(Icon, { name: 'dollar' }) },
    { title: t('orgDashboard.duration'), value: `${(metrics.average_duration_ms / 1000).toFixed(2)}s`, icon: () => h(Icon, { name: 'clock' }) }
  ]
})
async function loadKeys() {
  keysController?.abort()
  keysController = new AbortController()
  const stamp = ++keyEpoch
  loadingKeys.value = true
  keyError.value = ''
  keyRows.value = []
  try {
    const data = await teamsAPI.rows(props.team.id, 'keys', { mine: true, page: keyPage.value, page_size: keyPageSize.value, search: searchQuery.value }, keysController.signal)
    if (stamp !== keyEpoch) return
    keysTotal.value = data.total
    keyPageSize.value = data.page_size
    const lastPage = Math.max(1, Math.ceil(data.total / data.page_size))
    if (keyPage.value > lastPage) { keyPage.value = lastPage; await loadKeys(); return }
    keyRows.value = data.items
  } catch (error) {
    if (stamp === keyEpoch && !keysController.signal.aborted) keyError.value = (error as Error).message || t('teams.failed')
  } finally {
    if (stamp === keyEpoch) loadingKeys.value = false
  }
}
async function loadUsage() {
  usageController?.abort()
  usageController = new AbortController()
  const stamp = ++usageEpoch
  usage.value = null
  usageError.value = ''
  loadingUsage.value = true
  try {
    const data = await teamsAPI.memberDashboard(props.team.id, {
      start_date: startDate.value, end_date: endDate.value, granularity: granularity.value,
      timezone: Intl.DateTimeFormat().resolvedOptions().timeZone, key_id: keyID.value
    }, usageController.signal)
    if (stamp === usageEpoch) usage.value = data
  } catch (error) {
    if (stamp === usageEpoch && !usageController.signal.aborted) usageError.value = (error as Error).message || t('teams.failed')
  } finally {
    if (stamp === usageEpoch) loadingUsage.value = false
  }
}
function filtersChanged() {
  if (granularity.value === 'hour' && (new Date(endDate.value).getTime() - new Date(startDate.value).getTime()) / 86400000 >= 31) granularity.value = 'day'
  void loadUsage()
}
function searchKeys(value: string) { if (value !== keySearch.value) return; searchQuery.value = value.trim(); keyPage.value = 1; void loadKeys() }
function changeKeyPage(value: number) { keyPage.value = value; void loadKeys() }
function changeKeyPageSize(value: number) { keyPageSize.value = value; keyPage.value = 1; void loadKeys() }
async function analyze(id: number) {
  keyID.value = id
  await loadUsage()
  await nextTick()
  analysisRef.value?.scrollIntoView({ behavior: 'smooth', block: 'start' })
}
function openRecords(model = '') { emit('records', { keyID: keyID.value, model, startDate: startDate.value, endDate: endDate.value }) }
onMounted(() => { void loadKeys(); void loadUsage() })
onUnmounted(() => { keyEpoch++; usageEpoch++; keysController?.abort(); usageController?.abort() })
</script>
