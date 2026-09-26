<template>
  <section class="min-w-0 space-y-6">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div class="flex flex-wrap items-center gap-3">
        <h2 class="text-xl font-semibold">{{ data?.scope.name || t(`orgDashboard.${kind}`) }}</h2>
        <span class="badge" :class="kind === 'enterprise' ? 'badge-blue' : 'badge-orange'">{{
          t(`admin.users.roles.${kind === 'enterprise' ? 'enterprise_admin' : 'team_admin'}`)
        }}</span>
      </div>
      <router-link
        :to="kind === 'enterprise' ? '/my-enterprise' : '/my-team'"
        class="btn btn-secondary"
        >{{ t('orgDashboard.manage') }}</router-link
      >
    </div>
    <p class="text-sm text-gray-500">{{ t('orgDashboard.scopeHint') }}</p>
    <div
      v-if="paused"
      class="rounded-xl bg-amber-50 p-4 text-sm text-amber-800 dark:bg-amber-950 dark:text-amber-300"
      role="status"
    >
      {{ t('orgDashboard.paused') }}
    </div>
    <div class="card flex flex-wrap items-center gap-4 p-4">
      <DateRangePicker
        v-model:start-date="startDate"
        v-model:end-date="endDate"
        @change="filtersChanged"
      />
      <div v-if="kind === 'enterprise'" class="w-full sm:w-56">
        <Select
          v-model="teamID"
          :options="teamOptions"
          :aria-label="t('orgDashboard.teamFilter')"
          @update:model-value="filtersChanged"
        />
      </div>
      <div class="w-28">
        <Select
          v-model="granularity"
          :options="granularityOptions"
          :aria-label="t('orgDashboard.granularity')"
          @update:model-value="filtersChanged"
        />
      </div>
      <button class="btn btn-secondary sm:ml-auto" :disabled="loading" @click="refresh">
        {{ t('common.refresh') }}
      </button>
    </div>
    <div v-if="error" class="rounded-xl bg-red-50 p-4 text-red-700 dark:bg-red-950" role="alert">
      {{ error }} <button class="ml-3 underline" @click="refresh">{{ t('common.retry') }}</button>
    </div>
    <div v-if="loading" class="flex justify-center py-12"><LoadingSpinner /></div>
    <template v-else-if="data">
      <div class="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <div v-for="card in resourceCards" :key="card.label" class="card p-5">
          <p class="text-sm text-gray-500">{{ card.label }}</p>
          <p class="mt-3 text-2xl font-semibold">{{ card.value }}</p>
          <p class="mt-2 text-xs text-gray-400">{{ t('orgDashboard.current') }}</p>
        </div>
      </div>
      <div class="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <div v-for="card in usageCards(data.usage)" :key="card.label" class="card p-5">
          <p class="text-sm text-gray-500">{{ card.label }}</p>
          <p class="mt-3 text-2xl font-semibold">{{ card.value }}</p>
          <p class="mt-2 text-xs text-gray-400">{{ t('orgDashboard.period') }}</p>
        </div>
      </div>
      <p class="text-sm text-gray-500">
        {{ t('orgDashboard.allocated') }}:
        <span class="font-medium">{{ money(data.resources.allocated_balance) }}</span> ·
        {{ t('orgDashboard.costHint') }}
      </p>
      <OrganizationUsageCharts :trend="data.trend" :models="data.models" :kind="kind" />
      <div v-if="kind === 'enterprise'" class="card overflow-hidden">
        <h3 class="px-5 py-4 font-semibold">
          {{ t('orgDashboard.teamRanking') }}
          <span class="text-xs font-normal text-gray-500">Top 12</span>
        </h3>
        <DataTable :columns="teamColumns" :data="data.teams" :expandable-actions="false">
          <template #cell-name="{ row }"
            ><button
              class="font-medium text-blue-600 dark:text-blue-400"
              @click="selectTeam(row.team_id)"
            >
              {{ row.name }}
            </button></template
          >
          <template #cell-cost="{ value }">{{ money(value) }}</template
          ><template #cell-tokens="{ value }">{{ count(value) }}</template
          ><template #cell-requests="{ value }">{{ count(value) }}</template>
        </DataTable>
      </div>
      <div v-if="data.scope.can_view_members" class="card overflow-hidden">
        <div class="flex flex-wrap items-center justify-between gap-3 px-5 py-4">
          <h3 class="font-semibold">
            {{ allMembers ? t('orgDashboard.members') : t('orgDashboard.memberRanking') }}
            <span v-if="!allMembers" class="text-xs font-normal text-gray-500">Top 12</span>
          </h3>
          <button class="btn btn-secondary btn-sm" @click="toggleMembers">
            {{ allMembers ? t('orgDashboard.showRanking') : t('orgDashboard.showAll') }}
          </button>
        </div>
        <DataTable
          :columns="memberColumns"
          :data="allMembers ? members?.items || [] : data.members"
          :loading="membersLoading"
          :expandable-actions="false"
        >
          <template #cell-username="{ row }"
            ><button class="text-left" @click="openMember(row)">
              <p class="font-medium text-primary-600">{{ row.username || row.email }}</p>
              <p v-if="row.username" class="text-xs text-gray-500">{{ row.email }}</p>
            </button></template
          >
          <template #cell-status="{ value }"
            ><span class="badge" :class="value === 'active' ? 'badge-success' : 'badge-gray'">{{
              t(`teams.${value}`)
            }}</span></template
          >
          <template #cell-cost="{ value }">{{ money(value) }}</template
          ><template #cell-tokens="{ value }">{{ count(value) }}</template
          ><template #cell-requests="{ value }">{{ count(value) }}</template>
        </DataTable>
        <Pagination
          v-if="allMembers && members"
          :total="members.total"
          :page="memberPage"
          :page-size="20"
          :show-page-size-selector="false"
          @update:page="changeMemberPage"
        />
      </div>
      <p v-else class="card p-4 text-sm text-gray-500">{{ t('orgDashboard.memberRestricted') }}</p>
      <p class="text-xs text-gray-400">{{ t('orgDashboard.historyHint') }}</p>
    </template>
    <BaseDialog
      :show="detailOpen"
      :title="t('orgDashboard.memberDetail')"
      width="extra-wide"
      @close="closeDetail"
    >
      <div v-if="detailLoading" class="flex justify-center py-12"><LoadingSpinner /></div>
      <div v-else-if="detail" class="space-y-5">
        <div>
          <p class="text-lg font-semibold">{{ detail.member.username || detail.member.email }}</p>
          <p class="mt-1 text-sm text-gray-500">
            {{ detail.member.email }} · {{ detail.member.team_name }} ·
            {{ t(`teams.${detail.member.status}`) }}
          </p>
        </div>
        <div class="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
          <div
            v-for="card in usageCards(detail.member)"
            :key="card.label"
            class="rounded-xl bg-gray-50 p-4 dark:bg-dark-800"
          >
            <p class="text-xs text-gray-500">{{ card.label }}</p>
            <p class="mt-2 text-xl font-semibold">{{ card.value }}</p>
          </div>
        </div>
        <OrganizationUsageCharts :trend="detail.trend" :models="detail.models" :kind="kind" />
        <p class="text-xs text-gray-500">{{ t('orgDashboard.detailHint') }}</p>
      </div>
    </BaseDialog>
  </section>
</template>
<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import {
  organizationDashboardAPI as api,
  type OrganizationKind,
  type OrganizationDashboardData,
  type OrganizationMetrics,
  type OrganizationMember,
  type OrganizationMemberPage,
  type OrganizationMemberDetail,
  type OrganizationParams
} from '@/api/organizationDashboard'
import { enterprisesAPI } from '@/api/enterprises'
import type { TeamRow } from '@/api/teams'
import { formatDateLocalInput } from '@/utils/format'
import DateRangePicker from '@/components/common/DateRangePicker.vue'
import Select from '@/components/common/Select.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import OrganizationUsageCharts from './OrganizationUsageCharts.vue'
const props = defineProps<{ kind: OrganizationKind }>()
const emit = defineEmits<{ (event: 'permission-change'): void }>()
const { t } = useI18n(),
  auth = useAuthStore()
const data = ref<OrganizationDashboardData | null>(null),
  loading = ref(false),
  error = ref('')
const startDate = ref(formatDateLocalInput(new Date(Date.now() - 6 * 86400000))),
  endDate = ref(formatDateLocalInput(new Date()))
const granularity = ref<'day' | 'hour'>('day'),
  teamID = ref<number | string>(0),
  teams = ref<TeamRow[]>([])
const members = ref<OrganizationMemberPage | null>(null),
  membersLoading = ref(false),
  allMembers = ref(false),
  memberPage = ref(1)
const detail = ref<OrganizationMemberDetail | null>(null),
  detailOpen = ref(false),
  detailLoading = ref(false)
let epoch = 0,
  memberEpoch = 0,
  detailEpoch = 0,
  disposed = false,
  controller = new AbortController()
const params = (): OrganizationParams => ({
  start_date: startDate.value,
  end_date: endDate.value,
  granularity: granularity.value,
  ...(Number(teamID.value) > 0 ? { team_id: Number(teamID.value) } : {})
})
const teamOptions = computed(() => [
  { value: 0, label: t('orgDashboard.allTeams') },
  ...teams.value.map((team) => ({ value: team.id, label: team.name }))
])
const granularityOptions = computed(() => [
  { value: 'day', label: t('orgDashboard.day') },
  { value: 'hour', label: t('orgDashboard.hour') }
])
const paused = computed(
  () =>
    data.value?.scope.status === 'disabled' || data.value?.scope.enterprise_status === 'disabled'
)
const money = (value: number) =>
  `$${Number(value || 0).toLocaleString(undefined, { maximumFractionDigits: 8 })}`
const count = (value: number) => Number(value || 0).toLocaleString()
const resourceCards = computed(() => {
  const r = data.value?.resources
  if (!r) return []
  return [
    {
      label: t(`orgDashboard.${props.kind === 'enterprise' ? 'teamCount' : 'teamKeys'}`),
      value: count(props.kind === 'enterprise' ? r.team_count : r.team_key_count)
    },
    { label: t('orgDashboard.memberCount'), value: count(r.member_count) },
    { label: t('orgDashboard.balance'), value: money(r.balance) },
    { label: t('orgDashboard.subscriptions'), value: count(r.remaining_subscriptions) }
  ]
})
const usageCards = (usage: OrganizationMetrics) => [
  { label: t('orgDashboard.requests'), value: count(usage.requests) },
  { label: t('orgDashboard.tokens'), value: count(usage.tokens) },
  { label: t('orgDashboard.cost'), value: money(usage.cost) },
  {
    label: t('orgDashboard.duration'),
    value: `${Math.round(usage.average_duration_ms).toLocaleString()} ms`
  }
]
const metricColumns = computed(() =>
  ['requests', 'tokens', 'cost'].map((key) => ({ key, label: t(`orgDashboard.${key}`) }))
)
const teamColumns = computed(() => [
  { key: 'name', label: t('orgDashboard.team') },
  ...metricColumns.value
])
const memberColumns = computed(() => [
  { key: 'username', label: t('orgDashboard.member') },
  ...(props.kind === 'enterprise' ? [{ key: 'team_name', label: t('orgDashboard.team') }] : []),
  { key: 'status', label: t('teams.status') },
  ...metricColumns.value
])
function closeDetail() {
  detailEpoch++
  detail.value = null
  detailOpen.value = false
  detailLoading.value = false
}
function clearMembers() {
  memberEpoch++
  members.value = null
  allMembers.value = false
  membersLoading.value = false
  closeDetail()
}
async function handleError(e: unknown) {
  if (disposed || (e as { code?: string })?.code === 'ERR_CANCELED') return
  error.value = (e as { message?: string })?.message || t('orgDashboard.failed')
  const status = (e as { status?: number })?.status
  if (status === 403 || status === 401) {
    clearMembers()
    data.value = null
    emit('permission-change')
    await load()
  }
}
async function load() {
  const stamp = ++epoch
  controller.abort()
  controller = new AbortController()
  memberEpoch++
  closeDetail()
  members.value = null
  data.value = null
  loading.value = true
  try {
    const result = await api.dashboard(props.kind, params(), controller.signal)
    if (disposed || stamp !== epoch) return
    data.value = result
    error.value = ''
    if (!result.scope.can_view_members) clearMembers()
    else if (allMembers.value) await loadMembers()
  } catch (e) {
    if (!disposed && stamp === epoch) {
      error.value = (e as { message?: string })?.message || t('orgDashboard.failed')
      clearMembers()
      if ((e as { status?: number })?.status === 403) emit('permission-change')
    }
  } finally {
    if (!disposed && stamp === epoch) loading.value = false
  }
}
async function loadTeams() {
  if (props.kind !== 'enterprise') return
  try {
    const result: TeamRow[] = []
    let page = 1,
      total = 1
    while (result.length < total && !disposed) {
      const next = await enterprisesAPI.rows<TeamRow>(undefined, 'teams', { page, page_size: 200 })
      result.push(...next.items)
      total = next.total
      if (!next.items.length) break
      page++
    }
    if (!disposed) teams.value = result
  } catch (e) {
    if (!disposed) await handleError(e)
  }
}
async function refresh() {
  await auth.refreshUser().catch(() => emit('permission-change'))
  if (!disposed) await Promise.all([load(), loadTeams()])
}
function filtersChanged() {
  memberPage.value = 1
  if (
    granularity.value === 'hour' &&
    (new Date(endDate.value).getTime() - new Date(startDate.value).getTime()) / 86400000 >= 31
  )
    granularity.value = 'day'
  void load()
}
function selectTeam(id: number) {
  teamID.value = id
  filtersChanged()
}
async function loadMembers() {
  if (!data.value?.scope.can_view_members) return
  const stamp = ++memberEpoch
  membersLoading.value = true
  try {
    const result = await api.members(
      props.kind,
      { ...params(), page: memberPage.value, page_size: 20 },
      controller.signal
    )
    if (!disposed && stamp === memberEpoch) members.value = result
  } catch (e) {
    if (!disposed && stamp === memberEpoch) await handleError(e)
  } finally {
    if (!disposed && stamp === memberEpoch) membersLoading.value = false
  }
}
function toggleMembers() {
  allMembers.value = !allMembers.value
  if (allMembers.value) {
    memberPage.value = 1
    void loadMembers()
  }
}
function changeMemberPage(page: number) {
  memberPage.value = page
  void loadMembers()
}
async function openMember(member: OrganizationMember) {
  if (!data.value?.scope.can_view_members) return
  const stamp = ++detailEpoch
  detail.value = null
  detailOpen.value = true
  detailLoading.value = true
  try {
    const result = await api.member(
      props.kind,
      member.user_id,
      { ...params(), team_id: member.team_id },
      controller.signal
    )
    if (!disposed && stamp === detailEpoch) detail.value = result
  } catch (e) {
    if (!disposed && stamp === detailEpoch) {
      closeDetail()
      await handleError(e)
    }
  } finally {
    if (!disposed && stamp === detailEpoch) detailLoading.value = false
  }
}
onMounted(() => {
  void Promise.all([load(), loadTeams()])
  window.addEventListener('focus', refresh)
})
onUnmounted(() => {
  disposed = true
  epoch++
  clearMembers()
  controller.abort()
  window.removeEventListener('focus', refresh)
})
</script>
