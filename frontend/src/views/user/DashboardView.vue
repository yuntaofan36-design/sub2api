<template>
  <AppLayout>
    <div class="space-y-6">
      <div v-if="organizationKind" class="flex flex-wrap items-center justify-between gap-3">
        <h1 class="text-2xl font-bold text-gray-900 dark:text-white">{{ t('nav.dashboard') }}</h1>
        <div
          class="inline-flex rounded-xl bg-gray-100 p-1 dark:bg-dark-800"
          role="tablist"
          :aria-label="t('orgDashboard.scope')"
        >
          <button
            v-for="tab in tabs"
            :key="tab.value"
            class="rounded-lg px-4 py-2 text-sm font-medium transition-colors"
            role="tab"
            :aria-selected="scope === tab.value"
            :class="
              scope === tab.value
                ? 'bg-white text-gray-900 shadow-sm dark:bg-dark-700 dark:text-white'
                : 'text-gray-500'
            "
            @click="setScope(tab.value)"
          >
            {{ tab.label }}
          </button>
        </div>
      </div>
      <div v-if="!ready" class="flex justify-center py-12"><LoadingSpinner /></div>
      <div v-else-if="identityError" class="card p-6" role="alert">
        <p>{{ identityError }}</p>
        <button class="btn btn-secondary mt-4" @click="refreshIdentity">
          {{ t('common.retry') }}
        </button>
      </div>
      <OrganizationDashboard
        v-else-if="organizationKind && scope === 'organization'"
        :key="identity"
        :kind="organizationKind"
        @permission-change="refreshIdentity"
      />
      <PersonalDashboard v-else :key="auth.user?.id" />
    </div>
  </AppLayout>
</template>
<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { useAppStore } from '@/stores/app'
import AppLayout from '@/components/layout/AppLayout.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import PersonalDashboard from '@/components/user/dashboard/PersonalDashboard.vue'
import OrganizationDashboard from '@/components/organization/OrganizationDashboard.vue'
const auth = useAuthStore(),
  app = useAppStore(),
  route = useRoute(),
  router = useRouter(),
  { t } = useI18n()
const ready = ref(false),
  identityError = ref('')
const organizationKind = computed(() => {
  if (auth.isSimpleMode || app.backendModeEnabled) return null
  if (auth.isEnterpriseAdmin) return 'enterprise'
  if (auth.isTeamAdmin) return 'team'
  return null
})
const identity = computed(
  () => `${auth.user?.id}:${auth.user?.role}:${auth.user?.enterprise?.id}:${auth.user?.team?.id}`
)
const scope = computed(() =>
  organizationKind.value && route.query.scope !== 'personal' ? 'organization' : 'personal'
)
const tabs = computed(() => [
  { value: 'organization', label: t(`orgDashboard.${organizationKind.value || 'team'}`) },
  { value: 'personal', label: t('orgDashboard.personal') }
])
function setScope(value: string) {
  const query = { ...route.query }
  if (value === 'personal') query.scope = 'personal'
  else delete query.scope
  void router.replace({ query })
}
async function refreshIdentity() {
  try {
    await Promise.all([
      auth.refreshUser(),
      app.publicSettingsLoaded ? Promise.resolve() : app.fetchPublicSettings()
    ])
    identityError.value = ''
  } catch (e) {
    identityError.value = (e as { message?: string })?.message || t('orgDashboard.failed')
  } finally {
    ready.value = true
  }
}
watch(identity, () => {
  if (!organizationKind.value && route.query.scope) {
    const query = { ...route.query }
    delete query.scope
    void router.replace({ query })
  }
})
onMounted(() => {
  void refreshIdentity()
  window.addEventListener('focus', refreshIdentity)
})
onUnmounted(() => window.removeEventListener('focus', refreshIdentity))
</script>
