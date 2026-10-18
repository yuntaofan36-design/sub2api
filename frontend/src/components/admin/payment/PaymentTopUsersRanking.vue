<template>
  <section class="min-w-0 space-y-2" :aria-busy="loading">
    <p class="text-xs font-semibold text-gray-500 dark:text-gray-400">{{ currency }}</p>
    <div v-if="loading" class="flex h-32 items-center justify-center"><LoadingSpinner /></div>
    <div v-else-if="error" role="alert" class="py-4 text-sm text-red-600 dark:text-red-400">
      {{ error }}
      <button type="button" class="btn btn-secondary btn-sm ml-2" @click="loadRanking">
        <Icon name="refresh" size="sm" />{{ t('common.retry') }}
      </button>
    </div>
    <div v-else-if="!users.length" class="flex h-32 items-center justify-center text-sm text-gray-500 dark:text-gray-400">
      {{ t('payment.admin.noData') }}
    </div>
    <div v-else class="space-y-2">
      <div
        v-for="(user, idx) in users"
        :key="user.user_id"
        class="flex items-center justify-between gap-3 rounded-lg px-3 py-2 hover:bg-gray-50 dark:hover:bg-dark-700"
      >
        <div class="flex min-w-0 items-center gap-3">
          <span :class="['flex h-7 min-w-7 shrink-0 items-center justify-center rounded-full px-1 text-xs font-bold tabular-nums', rankClass(rankOffset + idx)]">
            {{ rankOffset + idx + 1 }}
          </span>
          <span class="min-w-0 break-all text-sm text-gray-700 dark:text-gray-300">{{ user.email }}</span>
        </div>
        <span class="shrink-0 text-sm font-medium tabular-nums text-gray-900 dark:text-white">{{ formatMoney(user.amount) }}</span>
      </div>
    </div>
    <Pagination
      v-if="!loading && !error && total > 0"
      class="ranking-pagination"
      :total="total"
      :page="page"
      :page-size="pageSize"
      @update:page="changePage"
      @update:page-size="changePageSize"
    />
  </section>
</template>

<script setup lang="ts">
import { computed, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminPaymentAPI } from '@/api/admin/payment'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import { extractI18nErrorMessage } from '@/utils/apiError'
import type { TopUserPaymentStats } from '@/types/payment'
import Icon from '@/components/icons/Icon.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import Pagination from '@/components/common/Pagination.vue'

const props = defineProps<{ days: number; currency: string }>()
const { t } = useI18n()
const users = ref<TopUserPaymentStats[]>([])
const page = ref(1)
const pageSize = ref(getPersistedPageSize())
const total = ref(0)
const loading = ref(false)
const error = ref('')
const rankOffset = computed(() => (page.value - 1) * pageSize.value)
let requestID = 0
let controller: AbortController | undefined

function rankClass(idx: number): string {
  if (idx === 0) return 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-400'
  if (idx === 1) return 'bg-gray-200 text-gray-600 dark:bg-gray-700 dark:text-gray-300'
  if (idx === 2) return 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400'
  return 'bg-gray-100 text-gray-500 dark:bg-dark-700 dark:text-gray-400'
}

function formatMoney(amount: number): string {
  return new Intl.NumberFormat(undefined, { style: 'currency', currency: props.currency }).format(amount)
}

async function loadRanking() {
  const stamp = ++requestID
  controller?.abort()
  controller = new AbortController()
  loading.value = true
  error.value = ''
  users.value = []
  try {
    const { data } = await adminPaymentAPI.getDashboardTopUsers({
      days: props.days,
      currency: props.currency,
      page: page.value,
      page_size: pageSize.value
    }, controller.signal)
    if (stamp !== requestID) return
    total.value = data.total
    pageSize.value = data.page_size
    const lastPage = Math.max(1, data.pages)
    if (page.value > lastPage) {
      page.value = lastPage
      await loadRanking()
      return
    }
    users.value = data.items
  } catch (err) {
    if (stamp === requestID) {
      error.value = extractI18nErrorMessage(err, t, 'payment.errors', t('common.error'))
    }
  } finally {
    if (stamp === requestID) loading.value = false
  }
}

function changePage(value: number) {
  page.value = value
  void loadRanking()
}

function changePageSize(value: number) {
  pageSize.value = value
  page.value = 1
  void loadRanking()
}

watch(() => [props.days, props.currency], () => {
  page.value = 1
  total.value = 0
  void loadRanking()
}, { immediate: true })

onUnmounted(() => {
  requestID++
  controller?.abort()
})
</script>

<style scoped>
.ranking-pagination :deep([class~='sm:flex']) {
  @apply flex-wrap gap-3;
}
.ranking-pagination :deep([class~='sm:flex'] > div) {
  @apply flex-wrap gap-y-2;
}
</style>
