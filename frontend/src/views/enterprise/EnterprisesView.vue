<template>
  <AppLayout>
    <div class="space-y-6">
      <div class="flex flex-wrap items-center justify-between gap-3">
        <div>
          <button
            v-if="platform && enterpriseID"
            class="mb-2 text-sm text-primary-600"
            @click="selectEnterprise(0)"
          >
            ← {{ t("enterprise.back") }}
          </button>
          <h1 class="text-2xl font-bold text-gray-900 dark:text-white">
            {{
              platform
                ? current?.name || t("enterprise.title")
                : t("enterprise.mine")
            }}
          </h1>
          <div v-if="!platform && current" class="mt-2 flex items-center gap-2">
            <span>{{ current.name }}</span
            ><span class="badge badge-blue">{{
              t("admin.users.roles.enterprise_admin")
            }}</span>
          </div>
          <p class="mt-2 text-sm text-gray-500">
            {{ t("enterprise.scopeHint") }}
          </p>
        </div>
        <div class="flex flex-wrap gap-2">
          <router-link
            v-if="!platform"
            to="/dashboard?scope=personal"
            class="btn btn-secondary"
            >{{ t("teams.personal") }}</router-link
          >
          <button
            class="btn btn-secondary"
            :disabled="loading"
            @click="initialize"
          >
            {{ t("teams.refresh") }}
          </button>
          <button
            v-if="platform && !enterpriseID"
            class="btn btn-primary"
            @click="open('createEnterprise')"
          >
            {{ t("enterprise.createEnterprise") }}
          </button>
        </div>
      </div>
      <div
        v-if="error"
        role="alert"
        class="rounded-xl bg-red-50 p-4 text-red-700 dark:bg-red-950"
      >
        {{ error }}
      </div>
      <div
        v-if="current?.status === 'disabled'"
        role="status"
        class="rounded-xl bg-amber-50 p-4 text-amber-800 dark:bg-amber-950 dark:text-amber-300"
      >
        {{ t("enterprise.disabledHint") }}
      </div>
      <div v-if="platform && !enterpriseID" class="card overflow-hidden">
        <DataTable
          :columns="enterpriseColumns"
          :data="enterprises"
          :loading="loading"
          :expandable-actions="false"
        >
          <template #cell-leaders="{ row }"
            ><div v-for="leader in row.leaders" :key="leader.id" class="py-1">
              <p class="font-medium text-blue-700 dark:text-blue-400">
                {{ leader.username || leader.email }}
              </p>
              <p v-if="leader.username" class="text-xs text-gray-500">
                {{ leader.email }}
              </p>
              <span
                v-if="leader.status !== 'active'"
                class="badge badge-gray"
                >{{ t("teams.disabled") }}</span
              >
            </div></template
          >
          <template #cell-status="{ row }"
            ><span
              class="badge"
              :class="row.status === 'active' ? 'badge-success' : 'badge-gray'"
              >{{ statusLabel(row.status) }}</span
            ></template
          >
          <template #cell-actions="{ row }"
            ><div class="flex items-center gap-1">
              <button class="row-action" @click="open('editEnterprise', row)">
                <Icon name="edit" size="sm" /><span>{{
                  t("common.edit")
                }}</span>
              </button>
              <button class="row-action" @click="selectEnterprise(row.id)">
                <Icon name="eye" size="sm" /><span>{{ t("teams.show") }}</span>
              </button>
              <button class="row-action" @click="open('leader', row)">
                <Icon name="users" size="sm" /><span>{{
                  t("enterprise.leaders")
                }}</span>
              </button>
              <button class="row-action" @click="open('attach', row)">
                <Icon name="link" size="sm" /><span>{{
                  t("enterprise.attach")
                }}</span>
              </button>
            </div></template
          >
        </DataTable>
      </div>
      <template v-else-if="current">
        <div class="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
          <div v-for="card in summaryCards" :key="card.label" class="card p-5">
            <p class="text-sm text-gray-500">{{ card.label }}</p>
            <p class="mt-2 text-2xl font-semibold">{{ card.value }}</p>
          </div>
        </div>
        <div v-if="platform" class="flex flex-wrap gap-2">
          <button
            class="btn btn-secondary"
            @click="open('editEnterprise', current)"
          >
            {{ t("enterprise.editEnterprise") }}
          </button>
          <button class="btn btn-secondary" @click="open('leader', current)">
            {{ t("enterprise.leader") }}
          </button>
          <button class="btn btn-secondary" @click="open('attach', current)">
            {{ t("enterprise.attach") }}
          </button>
        </div>
        <div
          class="flex flex-wrap gap-2 border-b border-gray-200 pb-3 dark:border-dark-700"
        >
          <button
            v-for="tab in tabs"
            :key="tab"
            class="btn"
            :class="section === tab ? 'btn-primary' : 'btn-secondary'"
            @click="changeSection(tab)"
          >
            {{ t(`enterprise.${tab}`) }}
          </button>
        </div>
        <template v-if="section === 'overview'">
          <div class="card space-y-4 p-5">
            <h2 class="font-semibold">{{ t("enterprise.usage") }}</h2>
            <div class="flex flex-wrap items-end gap-3">
              <label class="text-sm"
                >{{ t("teams.from")
                }}<input v-model="dates.from" type="date" class="input mt-1"
              /></label>
              <label class="text-sm"
                >{{ t("teams.to")
                }}<input v-model="dates.to" type="date" class="input mt-1"
              /></label>
              <button
                class="btn btn-secondary"
                :disabled="loading"
                @click="loadStats"
              >
                {{ t("teams.refresh") }}
              </button>
            </div>
            <div class="grid gap-4 sm:grid-cols-3">
              <div v-for="card in usageCards" :key="card.label">
                <p class="text-sm text-gray-500">{{ card.label }}</p>
                <p class="mt-2 text-xl font-semibold">{{ card.value }}</p>
              </div>
            </div>
            <p class="text-sm text-gray-500">{{ t("enterprise.usageHint") }}</p>
          </div>
          <div class="card p-5">
            <h2 class="mb-3 font-semibold">{{ t("enterprise.leaders") }}</h2>
            <div class="flex flex-wrap gap-3">
              <div
                v-for="leader in current.leaders"
                :key="leader.id"
                class="rounded-xl bg-blue-50 px-4 py-3 dark:bg-blue-900/20"
              >
                <p class="font-medium text-blue-700 dark:text-blue-400">
                  {{ leader.username || leader.email }}
                </p>
                <p class="text-sm text-gray-500">{{ leader.email }}</p>
              </div>
            </div>
          </div>
        </template>
        <template v-else>
          <div v-if="section === 'teams' && canGovern" class="flex justify-end">
            <button class="btn btn-primary" @click="open('createTeam')">
              {{ t("enterprise.createTeam") }}
            </button>
          </div>
          <div class="card overflow-hidden">
            <DataTable
              :columns="recordColumns"
              :data="records"
              :loading="loading"
              :expandable-actions="false"
            >
              <template #cell-administrators="{ row }"
                ><div v-for="a in row.administrators" :key="a.id" class="py-1">
                  <span
                    class="font-medium text-orange-700 dark:text-orange-400"
                    >{{ a.username || a.email }}</span
                  >
                  <p v-if="a.username" class="text-xs text-gray-500">
                    {{ a.email }}
                  </p>
                </div>
                <span
                  v-if="!row.administrators?.length"
                  class="text-gray-400"
                  >{{ t("teams.noAdministrators") }}</span
                ></template
              >
              <template #cell-email="{ row }"
                ><p class="font-medium text-orange-700 dark:text-orange-400">
                  {{ row.username || row.email }}
                </p>
                <p v-if="row.username" class="text-xs text-gray-500">
                  {{ row.email }}
                </p></template
              >
              <template #cell-status="{ row }"
                ><span
                  class="badge"
                  :class="
                    row.status === 'active' && row.member_status !== 'suspended'
                      ? 'badge-success'
                      : 'badge-gray'
                  "
                  >{{
                    statusLabel(
                      row.member_status === "suspended"
                        ? "suspended"
                        : row.status,
                    )
                  }}</span
                ></template
              >
              <template #cell-balance="{ row }">{{
                money(row.balance)
              }}</template>
              <template #cell-kind="{ row }">{{
                eventLabel(row.kind)
              }}</template>
              <template #cell-outcome="{ row }"
                ><span class="badge badge-success">{{
                  t(`teams.${row.outcome}`)
                }}</span></template
              >
              <template #cell-details="{ row }"
                ><span
                  class="block max-w-lg whitespace-pre-wrap break-words text-xs"
                  >{{ eventDetails(row.details) }}</span
                ></template
              >
              <template #cell-created_at="{ row }">{{
                new Date(row.created_at).toLocaleString()
              }}</template>
              <template #cell-actions="{ row }"
                ><div class="flex items-center gap-1">
                  <router-link
                    v-if="platform"
                    :to="`/admin/teams?team=${row.id}`"
                    class="row-action"
                    ><Icon name="eye" size="sm" /><span>{{
                      t("teams.show")
                    }}</span></router-link
                  >
                  <template v-else>
                    <button class="row-action" @click="open('teamDetail', row)">
                      <Icon name="eye" size="sm" /><span>{{
                        t("teams.show")
                      }}</span>
                    </button>
                    <template v-if="canGovern">
                      <button class="row-action" @click="open('editTeam', row)">
                        <Icon name="edit" size="sm" /><span>{{
                          t("common.edit")
                        }}</span>
                      </button>
                      <button
                        class="row-action"
                        @click="open('createAdmin', row)"
                      >
                        <Icon name="plus" size="sm" /><span>{{
                          t("enterprise.createAdmin")
                        }}</span>
                      </button>
                      <button class="row-action" @click="open('appoint', row)">
                        <Icon name="users" size="sm" /><span>{{
                          t("teams.appoint")
                        }}</span>
                      </button>
                    </template>
                  </template>
                </div></template
              >
            </DataTable>
          </div>
        </template>
      </template>
      <div
        v-if="
          (platform && !enterpriseID) || (current && section !== 'overview')
        "
        class="flex items-center justify-between"
      >
        <span class="text-sm text-gray-500">{{
          t("teams.total", { count: total })
        }}</span>
        <div class="flex gap-2">
          <button
            class="btn btn-secondary"
            :disabled="loading || page === 1"
            @click="paginate(-1)"
          >
            {{ t("teams.previous") }}</button
          ><button
            class="btn btn-secondary"
            :disabled="loading || page * 25 >= total"
            @click="paginate(1)"
          >
            {{ t("teams.next") }}
          </button>
        </div>
      </div>
    </div>
    <BaseDialog
      :show="!!dialog"
      :title="dialog ? t(`enterprise.${dialog}`) : ''"
      :show-close-button="!saving"
      :close-on-escape="!saving"
      :close-on-click-outside="!saving"
      @close="closeDialog"
    >
      <form
        v-if="dialog"
        id="enterprise-form"
        class="space-y-4"
        @submit.prevent="save"
      >
        <div
          v-if="formError"
          role="alert"
          class="rounded-lg bg-red-50 p-3 text-sm text-red-700 dark:bg-red-950"
        >
          {{ formError }}
        </div>
        <p v-if="formLoading" class="text-sm text-gray-500">
          {{ t("common.loading") }}
        </p>
        <template
          v-if="
            [
              'createEnterprise',
              'editEnterprise',
              'createTeam',
              'editTeam',
            ].includes(dialog)
          "
        >
          <label class="block"
            ><span class="input-label">{{ t("teams.name") }}</span
            ><input v-model="form.name" class="input" maxlength="100" required
          /></label>
          <label class="block"
            ><span class="input-label">{{ t("teams.status") }}</span
            ><Select v-model="form.status" :options="statusOptions"
          /></label>
        </template>
        <label v-if="platform && ['createEnterprise', 'editEnterprise'].includes(dialog)" class="flex items-start gap-3 rounded-xl border border-gray-200 p-4 dark:border-dark-700">
          <input v-model="form.allow_member_usage_view" type="checkbox" class="mt-1 rounded text-primary-600" />
          <span><span class="block font-medium">{{ t('orgDashboard.allowMemberView') }}</span><span class="mt-1 block text-sm text-gray-500">{{ t('orgDashboard.allowMemberHint') }}</span></span>
        </label>
        <template v-if="['leader', 'appoint'].includes(dialog)">
          <label class="block"
            ><span class="input-label">{{ t("teams.actions") }}</span
            ><Select
              v-model="form.promote"
              :options="[
                {
                  value: true,
                  label: t(
                    dialog === 'leader'
                      ? 'enterprise.promote'
                      : 'teams.promote',
                  ),
                },
                {
                  value: false,
                  label: t(
                    dialog === 'leader' ? 'enterprise.demote' : 'teams.demote',
                  ),
                },
              ]"
              @update:model-value="loadChoices"
          /></label>
          <p class="text-sm text-gray-500">
            {{
              t(
                dialog === "leader"
                  ? "enterprise.leaderHint"
                  : "enterprise.appointHint",
              )
            }}
          </p>
        </template>
        <template
          v-if="
            dialog === 'createEnterprise' ||
            (dialog === 'leader' && form.promote)
          "
        >
          <label class="block"
            ><span class="input-label">{{ t("enterprise.searchUser") }}</span>
            <div class="flex gap-2">
              <input
                v-model="search"
                class="input"
                type="search"
                @keydown.enter.prevent="loadChoices"
              /><button
                type="button"
                class="btn btn-secondary shrink-0"
                :disabled="formLoading"
                @click="loadChoices"
              >
                {{ t("common.search") }}
              </button>
            </div></label
          >
          <p class="text-sm text-gray-500">
            {{ t("enterprise.candidateHint") }}
          </p>
        </template>
        <label
          v-if="
            ['createEnterprise', 'leader', 'appoint', 'attach'].includes(dialog)
          "
          class="block"
          ><span class="input-label">{{
            t(
              dialog === "attach"
                ? "enterprise.team"
                : dialog === "appoint"
                  ? "teams.admin"
                  : "enterprise.leaders",
            )
          }}</span
          ><Select
            v-model="form.target_id"
            :options="choiceOptions"
            :disabled="formLoading"
            :searchable="true"
            :placeholder="t('enterprise.select')"
        /></label>
        <p
          v-if="dialog === 'attach'"
          class="rounded-lg bg-amber-50 p-3 text-sm text-amber-800 dark:bg-amber-950 dark:text-amber-300"
        >
          {{ t("enterprise.attachHint") }}
        </p>
        <template v-if="['createTeam', 'createAdmin'].includes(dialog)">
          <h3 class="font-medium text-orange-700 dark:text-orange-400">
            {{ t("enterprise.newAdmin") }}
          </h3>
          <p class="text-sm text-gray-500">
            {{ t("enterprise.newAdminHint") }}
          </p>
          <label class="block"
            ><span class="input-label">{{ t("teams.email") }}</span
            ><input
              v-model="form.email"
              type="email"
              class="input"
              autocomplete="off"
              maxlength="320"
              required
          /></label>
          <label class="block"
            ><span class="input-label">{{ t("enterprise.username") }}</span
            ><input v-model="form.username" class="input" maxlength="100"
          /></label>
          <label class="block"
            ><span class="input-label">{{ t("teams.password") }}</span
            ><input
              v-model="form.password"
              type="password"
              class="input"
              autocomplete="new-password"
              minlength="8"
              maxlength="72"
              required
          /></label>
        </template>
        <div v-if="dialog === 'teamDetail' && detail" class="space-y-4">
          <h3 class="text-lg font-semibold">{{ detail.name }}</h3>
          <span
            class="badge"
            :class="detail.status === 'active' ? 'badge-success' : 'badge-gray'"
            >{{ statusLabel(detail.status) }}</span
          >
          <div class="grid grid-cols-2 gap-4">
            <div v-for="card in teamCards" :key="card.label">
              <p class="text-sm text-gray-500">{{ card.label }}</p>
              <p class="mt-1 text-lg font-semibold">{{ card.value }}</p>
            </div>
          </div>
          <div>
            <p class="mb-2 text-sm text-gray-500">{{ t("teams.groups") }}</p>
            <div class="flex flex-wrap gap-2">
              <span
                v-for="group in detail.groups"
                :key="group.id"
                class="badge badge-gray"
                >{{ group.name }}</span
              ><span v-if="!detail.groups.length">{{
                t("enterprise.noGroups")
              }}</span>
            </div>
          </div>
          <p class="text-sm text-gray-500">{{ t("enterprise.usageHint") }}</p>
        </div>
      </form>
      <template #footer
        ><button
          class="btn btn-secondary"
          :disabled="saving"
          @click="closeDialog"
        >
          {{ t("common.cancel") }}</button
        ><button
          v-if="dialog !== 'teamDetail'"
          class="btn btn-primary"
          type="submit"
          form="enterprise-form"
          :disabled="saving || formLoading || !formReady || selectionMissing"
        >
          {{ t("common.save") }}
        </button></template
      >
    </BaseDialog>
    <TotpStepUpDialog :controller="stepUp" />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import AppLayout from "@/components/layout/AppLayout.vue";
import BaseDialog from "@/components/common/BaseDialog.vue";
import DataTable from "@/components/common/DataTable.vue";
import Select from "@/components/common/Select.vue";
import Icon from "@/components/icons/Icon.vue";
import TotpStepUpDialog from "@/components/auth/TotpStepUpDialog.vue";
import { useStepUp, isStepUpCancelled } from "@/composables/useStepUp";
import { useAuthStore } from "@/stores/auth";
import { useAppStore } from "@/stores/app";
import {
  enterprisesAPI,
  type Enterprise,
  type EnterpriseAccount,
  type EnterpriseEvent,
  type EnterpriseStats,
  type GovernTeamDetail,
} from "@/api/enterprises";
import { teamsAPI, type TeamRow } from "@/api/teams";

const { t, te } = useI18n(),
  route = useRoute(),
  router = useRouter(),
  auth = useAuthStore(),
  app = useAppStore(),
  stepUp = useStepUp();
const platform = computed(() => route.name === "AdminEnterprises");
const enterpriseID = computed(() =>
  platform.value ? Number(route.query.enterprise || 0) : 0,
);
const current = ref<Enterprise | null>(null),
  stats = ref<EnterpriseStats | null>(null),
  enterprises = ref<Enterprise[]>([]);
const records = ref<Array<TeamRow | EnterpriseAccount | EnterpriseEvent>>([]);
const loading = ref(false),
  error = ref(""),
  page = ref(1),
  total = ref(0),
  section = ref("overview");
const dates = reactive({ from: "", to: "" });
const tabs = ["overview", "teams", "administrators", "events"];
const canGovern = computed(
  () =>
    !platform.value &&
    auth.isEnterpriseAdmin &&
    current.value?.can_govern === true &&
    auth.user?.enterprise?.status === "active",
);
const dialog = ref(""),
  formError = ref(""),
  formLoading = ref(false),
  formReady = ref(false),
  saving = ref(false),
  search = ref("");
const dialogEntityID = ref(0),
  choices = ref<
    Array<{
      id: number;
      email?: string;
      name?: string;
      username?: string;
      role?: string;
    }>
  >([]);
const detail = ref<GovernTeamDetail | null>(null);
const form = reactive({
  name: "",
  status: "active",
  allow_member_usage_view: false,
  promote: true,
  target_id: 0,
  email: "",
  username: "",
  password: "",
});
let epoch = 0,
  dialogEpoch = 0,
  choicesEpoch = 0;
const money = (v?: number) =>
  `$${Number(v || 0).toLocaleString(undefined, { maximumFractionDigits: 8 })}`;
const statusLabel = (v?: string) => t(`teams.${v || "disabled"}`);
const message = (e: unknown) =>
  (e as { message?: string })?.message || t("teams.failed");
const statusOptions = computed(() =>
  ["active", "disabled"].map((value) => ({ value, label: statusLabel(value) })),
);
const choiceOptions = computed(() =>
  choices.value
    .filter(
      (c) =>
        dialog.value !== "appoint" ||
        c.role === (form.promote ? "user" : "team_admin"),
    )
    .map((c) => ({
      value: c.id,
      label: c.email
        ? `${c.username ? c.username + " · " : ""}${c.email}`
        : c.name || String(c.id),
    })),
);
const selectionMissing = computed(
  () =>
    ["createEnterprise", "leader", "appoint", "attach"].includes(
      dialog.value,
    ) && !choiceOptions.value.some((c) => c.value === form.target_id),
);
const enterpriseColumns = computed(() => [
  { key: "name", label: t("enterprise.name") },
  { key: "leaders", label: t("enterprise.leaders") },
  { key: "status", label: t("teams.status") },
  { key: "team_count", label: t("enterprise.teamCount") },
  { key: "actions", label: t("teams.actions") },
]);
const recordColumns = computed(() =>
  section.value === "teams"
    ? [
        { key: "name", label: t("teams.name") },
        { key: "administrators", label: t("teams.administrators") },
        { key: "status", label: t("teams.status") },
        { key: "member_count", label: t("teams.members") },
        { key: "balance", label: t("teams.available") },
        { key: "actions", label: t("teams.actions") },
      ]
    : section.value === "administrators"
      ? [
          { key: "email", label: t("teams.admin") },
          { key: "team_name", label: t("enterprise.team") },
          { key: "status", label: t("teams.status") },
        ]
      : [
          { key: "created_at", label: t("teams.when") },
          { key: "team_name", label: t("enterprise.team") },
          { key: "actor_email", label: t("teams.actor") },
          { key: "kind", label: t("teams.actions") },
          { key: "details", label: t("enterprise.details") },
          { key: "outcome", label: t("teams.result") },
        ],
);
const summaryCards = computed(() => [
  { label: t("enterprise.teamCount"), value: stats.value?.team_count || 0 },
  { label: t("teams.members"), value: stats.value?.member_count || 0 },
  { label: t("teams.available"), value: money(stats.value?.balance) },
  {
    label: t("enterprise.remaining"),
    value: stats.value?.remaining_subscriptions || 0,
  },
]);
const usageCards = computed(() => usage(stats.value));
const teamCards = computed(() => [
  { label: t("teams.members"), value: detail.value?.stats?.member_count || 0 },
  { label: t("teams.available"), value: money(detail.value?.stats?.balance) },
  {
    label: t("enterprise.remaining"),
    value: detail.value?.stats?.remaining_subscriptions || 0,
  },
  ...usage(detail.value?.stats),
]);
function usage(s?: EnterpriseStats | null) {
  return [
    { label: t("teams.requests"), value: s?.requests || 0 },
    { label: "Token", value: s?.tokens || 0 },
    { label: t("teams.cost"), value: money(s?.cost) },
  ];
}
function eventLabel(kind: string) {
  const key = `enterprise.eventKinds.${kind.replace(/\./g, "_")}`;
  return te(key) ? t(key) : kind;
}
function eventDetails(details: Record<string, unknown>): string {
  return Object.entries(details || {})
    .map(([key, value]) => {
      const label = te(`enterprise.fields.${key}`)
        ? t(`enterprise.fields.${key}`)
        : key;
      const formatted =
        value && typeof value === "object"
          ? eventDetails(value as Record<string, unknown>)
          : typeof value === "string" && te(`teams.${value}`)
            ? t(`teams.${value}`)
            : typeof value === "string" && te(`admin.users.roles.${value}`)
              ? t(`admin.users.roles.${value}`)
              : String(value ?? "—");
      return `${label}: ${formatted}`;
    })
    .join(" · ");
}
function dateParams() {
  return {
    from: dates.from
      ? new Date(`${dates.from}T00:00:00`).toISOString()
      : undefined,
    to: dates.to
      ? new Date(
          new Date(`${dates.to}T00:00:00`).getTime() + 86400000,
        ).toISOString()
      : undefined,
  };
}
async function initialize() {
  const stamp = ++epoch;
  loading.value = true;
  error.value = "";
  current.value = null;
  stats.value = null;
  records.value = [];
  try {
    await auth.refreshUser();
    if (stamp !== epoch) return;
    if (
      (platform.value && !auth.isAdmin) ||
      (!platform.value && !auth.isEnterpriseAdmin)
    ) {
      await router.replace("/dashboard");
      return;
    }
    if (platform.value && !enterpriseID.value) {
      const result = await enterprisesAPI.list({
        page: page.value,
        page_size: 25,
      });
      if (stamp === epoch) {
        enterprises.value = result.items;
        total.value = result.total;
      }
    } else {
      const [company, aggregate] = await Promise.all([
        enterprisesAPI.current(enterpriseID.value || undefined),
        enterprisesAPI.stats(enterpriseID.value || undefined, dateParams()),
      ]);
      if (stamp !== epoch) return;
      current.value = company;
      stats.value = aggregate;
      if (section.value !== "overview") await loadRows();
    }
  } catch (e) {
    if (stamp === epoch) error.value = message(e);
  } finally {
    if (stamp === epoch) loading.value = false;
  }
}
async function loadStats() {
  const stamp = epoch;
  loading.value = true;
  error.value = "";
  try {
    const data = await enterprisesAPI.stats(
      enterpriseID.value || undefined,
      dateParams(),
    );
    if (stamp === epoch) stats.value = data;
  } catch (e) {
    if (stamp === epoch) error.value = message(e);
  } finally {
    if (stamp === epoch) loading.value = false;
  }
}
async function loadRows() {
  const stamp = ++epoch;
  loading.value = true;
  error.value = "";
  records.value = [];
  try {
    const result = await enterprisesAPI.rows<
      TeamRow | EnterpriseAccount | EnterpriseEvent
    >(enterpriseID.value || undefined, section.value, {
      page: page.value,
      page_size: 25,
    });
    if (stamp === epoch) {
      records.value = result.items;
      total.value = result.total;
    }
  } catch (e) {
    if (stamp === epoch) error.value = message(e);
  } finally {
    if (stamp === epoch) loading.value = false;
  }
}
async function changeSection(tab: string) {
  section.value = tab;
  page.value = 1;
  if (tab !== "overview") await loadRows();
  else {
    epoch++;
    await loadStats();
  }
}
async function paginate(delta: number) {
  page.value += delta;
  if (platform.value && !enterpriseID.value) await initialize();
  else await loadRows();
}
function selectEnterprise(id: number) {
  page.value = 1;
  section.value = "overview";
  void router.replace({
    name: "AdminEnterprises",
    query: id ? { enterprise: id } : {},
  });
}
function closeDialog() {
  if (saving.value) return;
  dialog.value = "";
  form.password = "";
  dialogEpoch++;
  choicesEpoch++;
  formLoading.value = false;
}
async function open(
  kind: string,
  entity?: { id: number; name?: string; status?: string; allow_member_usage_view?: boolean },
) {
  const stamp = ++dialogEpoch;
  dialog.value = kind;
  dialogEntityID.value = entity?.id || 0;
  formError.value = "";
  formReady.value = false;
  formLoading.value = true;
  choices.value = [];
  search.value = "";
  detail.value = null;
  Object.assign(form, {
    name: entity?.name || "",
    status: entity?.status || "active",
    allow_member_usage_view: entity?.allow_member_usage_view ?? false,
    promote: true,
    target_id: 0,
    email: "",
    username: "",
    password: "",
  });
  try {
    if (kind === "teamDetail") {
      const data = await enterprisesAPI.team(dialogEntityID.value);
      if (stamp === dialogEpoch) detail.value = data;
    } else if (
      ["createEnterprise", "leader", "appoint", "attach"].includes(kind)
    )
      await loadChoices();
    else formReady.value = true;
  } catch (e) {
    if (stamp === dialogEpoch) formError.value = message(e);
  } finally {
    if (stamp === dialogEpoch) formLoading.value = false;
  }
}
async function loadChoices() {
  const stamp = ++choicesEpoch,
    currentDialog = dialogEpoch;
  formLoading.value = true;
  formReady.value = false;
  formError.value = "";
  form.target_id = 0;
  choices.value = [];
  try {
    let result: typeof choices.value = [];
    if (dialog.value === "leader" && !form.promote)
      result = (await enterprisesAPI.current(dialogEntityID.value)).leaders;
    else {
      for (let p = 1; ; p++) {
        const data =
          dialog.value === "attach"
            ? await teamsAPI.list(p, -1)
            : dialog.value === "appoint"
              ? await enterprisesAPI.teamCandidates(dialogEntityID.value, p)
              : await enterprisesAPI.candidates(search.value, p);
        if (stamp !== choicesEpoch || currentDialog !== dialogEpoch) return;
        result.push(...data.items);
        if (result.length >= data.total || !data.items.length) break;
      }
    }
    if (stamp === choicesEpoch && currentDialog === dialogEpoch) {
      choices.value = result;
      formReady.value = true;
    }
  } catch (e) {
    if (stamp === choicesEpoch && currentDialog === dialogEpoch)
      formError.value = message(e);
  } finally {
    if (stamp === choicesEpoch && currentDialog === dialogEpoch)
      formLoading.value = false;
  }
}
async function save() {
  if (
    saving.value ||
    formLoading.value ||
    !formReady.value ||
    selectionMissing.value
  )
    return;
  saving.value = true;
  formError.value = "";
  const id = dialogEntityID.value,
    kind = dialog.value;
  try {
    const metadata = { name: form.name.trim(), status: form.status };
    const administrator = {
      email: form.email.trim(),
      username: form.username.trim(),
      password: form.password,
    };
    await stepUp.run(() => {
      switch (kind) {
        case "createEnterprise":
          return enterprisesAPI.save("post", "/admin/enterprises", {
            ...metadata,
            leader_id: form.target_id,
            allow_member_usage_view: form.allow_member_usage_view,
          });
        case "editEnterprise":
          return enterprisesAPI.save(
            "put",
            `/admin/enterprises/${id}`,
            { ...metadata, allow_member_usage_view: form.allow_member_usage_view },
          );
        case "leader":
          return enterprisesAPI.save(
            "put",
            `/admin/enterprises/${id}/leaders/${form.target_id}`,
            { promote: form.promote },
          );
        case "attach":
          return enterprisesAPI.save(
            "put",
            `/admin/enterprises/${id}/teams/${form.target_id}`,
          );
        case "createTeam":
          return enterprisesAPI.save("post", "/enterprise/teams", {
            ...metadata,
            administrator,
          });
        case "editTeam":
          return enterprisesAPI.save(
            "patch",
            `/enterprise/teams/${id}`,
            metadata,
          );
        case "createAdmin":
          return enterprisesAPI.save(
            "post",
            `/enterprise/teams/${id}/administrators`,
            administrator,
          );
        case "appoint":
          return enterprisesAPI.save(
            "put",
            `/enterprise/teams/${id}/administrators/${form.target_id}`,
            { promote: form.promote },
          );
        default:
          return Promise.reject(new Error(t("teams.failed")));
      }
    });
    saving.value = false;
    closeDialog();
    app.showSuccess(t("teams.success"));
    await initialize();
  } catch (e) {
    if (!isStepUpCancelled(e)) formError.value = message(e);
    // Re-read role/status after failed writes so revoked or paused permissions disappear immediately.
    await initialize();
  } finally {
    saving.value = false;
  }
}
watch(
  () => route.fullPath,
  () => {
    closeDialog();
    page.value = 1;
    section.value = "overview";
    void initialize();
  },
);
watch(
  [
    () => auth.user?.role,
    () => auth.user?.enterprise?.id,
    () => auth.user?.enterprise?.status,
  ],
  (_next, previous) => {
    if (!previous[0] || platform.value) return;
    closeDialog();
    void initialize();
  },
);
onMounted(initialize);
</script>

<style scoped>
.row-action {
  @apply flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-xs text-gray-500 transition-colors hover:bg-gray-100 hover:text-primary-600 dark:hover:bg-dark-700 dark:hover:text-primary-400;
}
</style>
