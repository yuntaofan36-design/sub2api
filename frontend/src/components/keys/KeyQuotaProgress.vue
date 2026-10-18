<template>
  <div class="min-w-0 space-y-1.5">
    <div class="flex flex-wrap items-baseline justify-between gap-x-3 gap-y-1 text-xs">
      <span class="text-gray-500 dark:text-gray-400">{{ label }}</span>
      <span class="font-medium tabular-nums" :class="tone">{{ money(effectiveUsed) }} / {{ money(limit) }}</span>
    </div>
    <div role="progressbar" :aria-label="label" :aria-valuenow="percent" aria-valuemin="0" aria-valuemax="100"
      class="h-1.5 overflow-hidden rounded-full bg-gray-100 dark:bg-dark-700">
      <div class="h-full rounded-full" :class="barTone" :style="{ width: `${percent}%` }" />
    </div>
    <div class="flex flex-wrap justify-between gap-1 text-xs text-gray-500 dark:text-gray-400">
      <span>{{ t('teams.workspace.remaining', { amount: money(Math.max(0, limit - effectiveUsed)) }) }}</span>
      <span v-if="resetsAt" :title="new Date(resetsAt).toLocaleString()">
        {{ resetExpired ? t('teams.workspace.windowReset') : t('teams.workspace.resetsAt', { time: new Date(resetsAt).toLocaleString() }) }}
      </span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useNow } from '@vueuse/core'
import { useI18n } from 'vue-i18n'
import { formatCostFixed } from '@/utils/format'

const props = defineProps<{ label: string; limit: number; used: number; resetsAt?: string | null }>()
const { t } = useI18n()
const now = useNow({ interval: 60_000 })
const resetExpired = computed(() => !!props.resetsAt && new Date(props.resetsAt).getTime() <= now.value.getTime())
const effectiveUsed = computed(() => resetExpired.value ? 0 : props.used)
const percent = computed(() => Math.min(100, Math.max(0, effectiveUsed.value / props.limit * 100)))
const tone = computed(() => percent.value >= 100 ? 'text-red-600 dark:text-red-400' : percent.value >= 80 ? 'text-amber-600 dark:text-amber-400' : 'text-gray-700 dark:text-gray-300')
const barTone = computed(() => percent.value >= 100 ? 'bg-red-500' : percent.value >= 80 ? 'bg-amber-500' : 'bg-primary-500')
const money = (value: number) => `$${formatCostFixed(value)}`
</script>
