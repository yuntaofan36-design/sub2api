<template>
  <div class="grid min-w-0 grid-cols-1 gap-6 xl:grid-cols-2">
    <div class="card min-w-0 p-5">
      <div class="mb-5 flex flex-wrap items-center justify-between gap-3">
        <h3 class="font-semibold">{{ t('orgDashboard.trend') }}</h3>
        <div class="tabs inline-flex shrink-0">
          <button
            v-for="item in metrics"
            :key="item"
            type="button"
            class="tab !px-2 !py-1 text-xs"
            :class="metric === item ? 'tab-active' : ''"
            @click="metric = item"
          >
            {{ t(`orgDashboard.${item}`) }}
          </button>
        </div>
      </div>
      <div v-if="trend.length" class="h-64"><Line :data="lineData" :options="lineOptions" /></div>
      <div v-else class="flex h-64 items-center justify-center text-gray-500">
        {{ t('orgDashboard.empty') }}
      </div>
    </div>
    <div class="card min-w-0 p-5">
      <h3 class="mb-5 font-semibold">{{ t('orgDashboard.models') }}</h3>
      <div v-if="models.length" class="flex flex-col gap-5 sm:flex-row">
        <div class="h-48 w-full sm:w-40 sm:shrink-0">
          <Doughnut :data="modelData" :options="modelOptions" />
        </div>
        <div class="max-h-64 min-w-0 flex-1 overflow-auto">
          <table class="w-full min-w-[360px] text-sm">
            <thead>
              <tr class="text-xs text-gray-500">
                <th class="min-w-[220px] pb-3 text-left">{{ t('orgDashboard.model') }}</th>
                <th class="min-w-[120px] pb-3 text-right">{{ t('orgDashboard.requests') }}</th>
                <th class="min-w-[120px] pb-3 text-right">{{ t('orgDashboard.cost') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="model in models"
                :key="model.model"
                class="border-t border-gray-100 dark:border-dark-700"
              >
                <td class="min-w-[220px] max-w-48 break-words py-2 pr-3">{{ model.model }}</td>
                <td class="min-w-[120px] py-2 text-right">{{ model.requests.toLocaleString() }}</td>
                <td class="min-w-[120px] py-2 pl-3 text-right">{{ money(model.cost) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
      <div v-else class="flex h-64 items-center justify-center text-gray-500">
        {{ t('orgDashboard.empty') }}
      </div>
    </div>
  </div>
</template>
<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  Chart as ChartJS,
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  ArcElement,
  Tooltip,
  Legend,
  Filler,
  type ChartOptions
} from 'chart.js'
import { Line, Doughnut } from 'vue-chartjs'
import type { OrganizationPoint, OrganizationModel } from '@/api/organizationDashboard'
ChartJS.register(
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  ArcElement,
  Tooltip,
  Legend,
  Filler
)
const props = defineProps<{
  trend: OrganizationPoint[]
  models: OrganizationModel[]
  kind: 'enterprise' | 'team'
}>()
const { t } = useI18n()
const metrics = ['requests', 'tokens', 'cost'] as const
const metric = ref<(typeof metrics)[number]>('tokens')
const money = (value: number) => `$${value.toLocaleString(undefined, { maximumFractionDigits: 8 })}`
const lineData = computed(() => ({
  labels: props.trend.map((row) => row.date),
  datasets: [
    {
      label: t(`orgDashboard.${metric.value}`),
      data: props.trend.map((row) => row[metric.value]),
      borderColor: props.kind === 'enterprise' ? '#3b82f6' : '#f97316',
      backgroundColor: props.kind === 'enterprise' ? '#3b82f61a' : '#f973161a',
      fill: true,
      tension: 0.25,
      pointRadius: 2
    }
  ]
}))
const lineOptions: ChartOptions<'line'> = {
  responsive: true,
  maintainAspectRatio: false,
  plugins: { legend: { display: false } },
  scales: { y: { beginAtZero: true }, x: { ticks: { maxTicksLimit: 7 } } }
}
const modelData = computed(() => ({
  labels: props.models.map((row) => row.model),
  datasets: [
    {
      data: props.models.map((row) => row.requests),
      backgroundColor: [
        '#3b82f6',
        '#10b981',
        '#f97316',
        '#8b5cf6',
        '#06b6d4',
        '#ec4899',
        '#64748b'
      ],
      borderWidth: 0
    }
  ]
}))
const modelOptions: ChartOptions<'doughnut'> = {
  responsive: true,
  maintainAspectRatio: false,
  cutout: '70%',
  plugins: { legend: { display: false } }
}
</script>
