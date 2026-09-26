<template>
  <AppLayout>
    <div class="space-y-6">
      <div class="flex flex-wrap items-center justify-between gap-3">
        <div>
          <button
            v-if="platform && teamID"
            class="mb-2 text-sm text-primary-600"
            @click="selectTeam(0)"
          >
            ← {{ t("teams.back") }}
          </button>
          <h1 class="text-2xl font-bold text-gray-900 dark:text-white">
            {{
              platform ? overview?.name || t("teams.title") : t("teams.mine")
            }}
          </h1>
          <div
            v-if="!platform && overview"
            class="mt-2 flex flex-wrap items-center gap-2"
          >
            <span
              class="text-sm font-medium text-gray-700 dark:text-gray-300"
              >{{ overview.name }}</span
            >
            <span
              class="badge"
              :class="auth.isTeamAdmin ? 'badge-orange' : 'badge-gray'"
              >{{ t(`admin.users.roles.${auth.user?.role || "user"}`) }}</span
            >
          </div>
          <p v-if="overview" class="mt-1 text-sm text-gray-500">
            {{ t(canViewTeam ? "orgDashboard.managementHint" : "teams.costHint") }}
          </p>
        </div>
        <div class="flex flex-wrap gap-2">
          <router-link
            v-if="auth.isTeamAdmin"
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
            v-if="platform && !teamID"
            class="btn btn-primary"
            :disabled="!!actionLoadingID"
            @click="open('createTeam')"
          >
            {{ t("teams.createTeam") }}
          </button>
        </div>
      </div>
      <div
        v-if="error"
        role="alert"
        class="rounded-xl border border-red-200 bg-red-50 p-4 text-red-700 dark:border-red-900 dark:bg-red-950"
      >
        {{ error }}
      </div>
      <div v-if="route.name === 'TeamInvitation'" class="card space-y-4 p-6">
        <h2 class="text-lg font-semibold">{{ t("teams.accept") }}</h2>
        <p class="text-sm text-gray-500">{{ t("teams.acceptHint") }}</p>
        <button
          class="btn btn-primary"
          :disabled="saving"
          @click="acceptInvite"
        >
          {{ t("teams.accept") }}
        </button>
      </div>
      <div v-else-if="platform && !teamID" class="card overflow-hidden">
        <div class="max-w-sm p-4">
          <label class="input-label">{{ t("enterprise.filter") }}</label
          ><Select
            v-model="enterpriseFilter"
            :options="[
              { value: 0, label: t('enterprise.all') },
              { value: -1, label: t('enterprise.standalone') },
              ...enterpriseOptions.map((e) => ({ value: e.id, label: e.name })),
            ]"
            @update:model-value="
              page = 1;
              initialize();
            "
          />
        </div>
        <DataTable
          :columns="teamColumns"
          :data="teams"
          :loading="loading"
          :expandable-actions="false"
        >
          <template #cell-name="{ row }">
            <span class="font-medium text-gray-900 dark:text-white">{{
              row.name
            }}</span>
          </template>
          <template #cell-administrators="{ row }">
            <div v-if="row.administrators?.length" class="space-y-3">
              <div
                v-for="administrator in row.administrators"
                :key="administrator.id"
                class="flex items-center gap-2.5"
              >
                <span
                  class="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-orange-100 text-orange-700 dark:bg-orange-900/30 dark:text-orange-400"
                  :title="t('teams.admin')"
                >
                  <Icon name="shield" size="sm" />
                </span>
                <div class="min-w-0">
                  <div class="flex items-center gap-2">
                    <span
                      class="max-w-64 truncate font-medium text-orange-700 dark:text-orange-400"
                      :title="administrator.username || administrator.email"
                      >{{ administrator.username || administrator.email }}</span
                    >
                    <span
                      v-if="
                        administrator.status !== 'active' ||
                        administrator.member_status !== 'active'
                      "
                      class="badge badge-gray"
                      >{{
                        label(
                          administrator.status !== "active"
                            ? administrator.status
                            : administrator.member_status,
                        )
                      }}</span
                    >
                  </div>
                  <p
                    v-if="administrator.username"
                    class="mt-0.5 max-w-64 truncate text-xs text-gray-500 dark:text-gray-400"
                    :title="administrator.email"
                  >
                    {{ administrator.email }}
                  </p>
                </div>
              </div>
            </div>
            <span v-else class="text-gray-400">{{
              t("teams.noAdministrators")
            }}</span>
          </template>
          <template #cell-enterprise_name="{ row }"
            ><router-link
              v-if="row.enterprise_id"
              :to="`/admin/enterprises?enterprise=${row.enterprise_id}`"
              class="text-blue-700 dark:text-blue-400"
              >{{ row.enterprise_name }}</router-link
            ><span v-else class="text-gray-400">{{
              t("enterprise.standalone")
            }}</span></template
          >
          <template #cell-status="{ row }">
            <span
              class="badge"
              :class="row.status === 'active' ? 'badge-success' : 'badge-gray'"
              >{{ label(row.status) }}</span
            >
          </template>
          <template #cell-balance="{ row }">{{ money(row.balance) }}</template>
          <template #cell-actions="{ row }">
            <div class="flex items-center gap-1">
              <button
                :disabled="!!actionLoadingID"
                :title="t('teams.editTeam')"
                class="flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-gray-100 hover:text-primary-600 disabled:opacity-50 dark:hover:bg-dark-700 dark:hover:text-primary-400"
                @click="openTeamAction('editTeam', row)"
              >
                <Icon name="edit" size="sm" /><span class="text-xs">{{
                  t("common.edit")
                }}</span>
              </button>
              <button
                :disabled="!!actionLoadingID"
                class="flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-gray-100 hover:text-primary-600 disabled:opacity-50 dark:hover:bg-dark-700 dark:hover:text-primary-400"
                @click="selectTeam(row.id)"
              >
                <Icon name="eye" size="sm" /><span class="text-xs">{{
                  t("teams.show")
                }}</span>
              </button>
              <button
                :disabled="!!actionLoadingID"
                aria-haspopup="menu"
                :aria-expanded="actionMenuTeam?.id === row.id"
                :aria-controls="
                  actionMenuTeam?.id === row.id ? 'team-action-menu' : undefined
                "
                class="team-action-trigger flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-gray-100 hover:text-gray-900 disabled:opacity-50 dark:hover:bg-dark-700 dark:hover:text-white"
                :class="{
                  'bg-gray-100 text-gray-900 dark:bg-dark-700 dark:text-white':
                    actionMenuTeam?.id === row.id,
                }"
                @click="toggleActionMenu(row, $event)"
              >
                <Icon name="more" size="sm" /><span class="text-xs">{{
                  t("common.more")
                }}</span>
              </button>
            </div>
          </template>
        </DataTable>
        <div class="flex items-center justify-between p-4">
          <span>{{ t("teams.total", { count: total }) }}</span>
          <div class="flex gap-2">
            <button
              class="btn btn-secondary"
              :disabled="page === 1 || loading"
              @click="
                page--;
                initialize();
              "
            >
              {{ t("teams.previous") }}</button
            ><button
              class="btn btn-secondary"
              :disabled="page * 25 >= total || loading"
              @click="
                page++;
                initialize();
              "
            >
              {{ t("teams.next") }}
            </button>
          </div>
        </div>
      </div>
      <div
        v-else-if="!loading && !overview && !error"
        class="card p-12 text-center"
      >
        <h2 class="text-lg font-semibold">{{ t("teams.noTeam") }}</h2>
        <p class="mt-2 text-gray-500">{{ t("teams.noTeamHint") }}</p>
      </div>
      <template v-else-if="overview">
        <div
          v-if="overview.enterprise_status === 'disabled'"
          role="status"
          class="rounded-xl bg-amber-50 p-4 text-amber-800 dark:bg-amber-950 dark:text-amber-300"
        >
          {{ t("enterprise.disabledHint") }}
        </div>
        <div
          v-if="
            overview.status !== 'active' ||
            overview.member_status === 'suspended'
          "
          class="rounded-xl bg-amber-50 p-4 text-amber-800"
        >
          {{ t("teams.inactiveHint") }}
        </div>
        <div class="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
          <div class="card p-5">
            <p class="text-sm text-gray-500">{{ t("teams.requests") }}</p>
            <p class="mt-2 text-2xl font-semibold">
              {{ overview.usage.requests.toLocaleString() }}
            </p>
          </div>
          <div class="card p-5">
            <p class="text-sm text-gray-500">{{ t("teams.tokens") }}</p>
            <p class="mt-2 text-2xl font-semibold">
              {{ overview.usage.tokens.toLocaleString() }}
            </p>
          </div>
          <div class="card p-5">
            <p class="text-sm text-gray-500">{{ t("teams.cost") }}</p>
            <p class="mt-2 text-2xl font-semibold">
              {{ money(overview.usage.cost) }}
            </p>
          </div>
          <div v-if="canViewTeam" class="card p-5">
            <p class="text-sm text-gray-500">{{ t("teams.available") }}</p>
            <p class="mt-2 text-2xl font-semibold">
              {{ money(overview.balance) }}
            </p>
            <p class="mt-2 text-sm text-gray-500">
              {{ t("teams.allocated") }}:
              {{ money(overview.allocated_balance) }}
            </p>
          </div>
          <div v-else class="card p-5">
            <p class="text-sm text-gray-500">{{ t("teams.status") }}</p>
            <p class="mt-2 text-xl font-semibold">
              {{ label(overview.member_status) }}
            </p>
          </div>
          <template v-if="canViewTeam">
            <div class="card p-5">
              <p class="text-sm text-gray-500">{{ t("teams.members") }}</p>
              <p class="mt-2 text-2xl font-semibold">
                {{ overview.member_count }}
              </p>
            </div>
            <div class="card p-5">
              <p class="text-sm text-gray-500">{{ t("teams.remaining") }}</p>
              <p class="mt-2 text-2xl font-semibold">
                {{ overview.remaining_subscriptions }}
              </p>
            </div>
          </template>
        </div>
        <div class="flex flex-wrap gap-2 border-b pb-3 dark:border-dark-600">
          <button
            v-for="tab in tabs"
            :key="tab"
            class="btn"
            :class="tab === section ? 'btn-primary' : 'btn-secondary'"
            @click="changeSection(tab)"
          >
            {{ t(`teams.${tab}`) }}
          </button>
        </div>
        <div class="flex flex-wrap justify-between gap-3">
          <div class="flex flex-wrap gap-2">
            <template v-if="section === 'members' && canManage"
              ><button class="btn btn-primary" @click="open('createMember')">
                {{ t("teams.createMember") }}</button
              ><button class="btn btn-secondary" @click="open('invite')">
                {{ t("teams.invite") }}</button
              ><button class="btn btn-secondary" @click="open('allocate')">
                {{ t("teams.allocate") }}
              </button></template
            >
            <button
              v-if="section === 'keys' && canManage"
              class="btn btn-primary"
              @click="open('createKey')"
            >
              {{ t("teams.createKey") }}
            </button>
            <button
              v-if="section === 'invitations' && canManage"
              class="btn btn-primary"
              @click="open('invite')"
            >
              {{ t("teams.invite") }}
            </button>
          </div>
          <button
            v-if="
              !platform &&
              auth.user?.role === 'user' &&
              overview.enterprise_status !== 'disabled'
            "
            class="btn btn-secondary text-red-600"
            @click="
              confirmAction(t('teams.leaveHint'), () =>
                request('post', `/teams/${teamID}/leave`),
              )
            "
          >
            {{ t("teams.leave") }}
          </button>
        </div>
        <div v-if="section === 'keys' && canViewTeam" class="max-w-sm">
          <Select
            v-model="filters.member_id"
            :options="[{ value: 0, label: t('teams.all') }, ...memberOptions]"
            @update:model-value="
              page = 1;
              loadRows();
            "
          />
        </div>
        <div
          v-if="section === 'usage'"
          class="card flex flex-wrap items-end gap-3 p-4"
        >
          <label class="min-w-36 flex-1"
            ><span class="input-label">{{ t("teams.from") }}</span
            ><input v-model="filters.from" type="date" class="input"
          /></label>
          <label class="min-w-36 flex-1"
            ><span class="input-label">{{ t("teams.to") }}</span
            ><input v-model="filters.to" type="date" class="input"
          /></label>
          <label v-if="canViewTeam" class="min-w-40 flex-1"
            ><span class="input-label">{{ t("teams.member") }}</span
            ><Select
              v-model="filters.member_id"
              :options="[
                { value: 0, label: t('teams.all') },
                ...memberOptions,
              ]"
          /></label>
          <label class="min-w-40 flex-1"
            ><span class="input-label">{{ t("teams.keys") }}</span
            ><Select
              v-model="filters.key_id"
              :options="[{ value: 0, label: t('teams.all') }, ...keyOptions]"
          /></label>
          <label class="min-w-32 flex-1"
            ><span class="input-label">{{ t("teams.model") }}</span
            ><input v-model="filters.model" class="input"
          /></label>
          <button
            class="btn btn-primary"
            @click="
              page = 1;
              loadRows();
            "
          >
            {{ t("teams.refresh") }}</button
          ><button
            class="btn btn-secondary"
            :disabled="saving"
            @click="exportUsage"
          >
            {{ t("teams.export") }}
          </button>
        </div>
        <p v-if="section === 'keys'" class="text-sm text-gray-500">
          {{ t("teams.keyHint") }}
        </p>
        <div v-if="section !== 'overview'" class="card overflow-x-auto">
          <table class="w-full whitespace-nowrap text-left text-sm">
            <thead>
              <tr
                class="border-b bg-gray-50 dark:border-dark-600 dark:bg-dark-800"
              >
                <th v-for="col in columns" :key="col.key" class="px-4 py-3">
                  {{ t(`teams.${col.label}`) }}
                </th>
                <th
                  v-if="
                    section !== 'usage' &&
                    section !== 'ledger' &&
                    section !== 'grants'
                  "
                  class="px-4 py-3"
                >
                  {{ t("teams.actions") }}
                </th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="row in records"
                :key="row.id"
                class="border-b dark:border-dark-700"
              >
                <td
                  v-for="col in columns"
                  :key="col.key"
                  class="px-4 py-4"
                  :class="
                    col.key === 'details'
                      ? 'max-w-md whitespace-normal break-words'
                      : ''
                  "
                >
                  <span
                    v-if="col.key === 'role'"
                    :class="[
                      'badge',
                      row.role === 'team_admin'
                        ? 'badge-orange'
                        : row.role === 'admin'
                          ? 'badge-purple'
                          : 'badge-gray',
                    ]"
                    >{{ cell(row, col.key) }}</span
                  >
                  <template v-else>{{ cell(row, col.key) }}</template>
                </td>
                <td v-if="section === 'members' && canManage" class="px-4 py-3">
                  <div class="flex gap-2">
                    <button
                      v-if="row.status === 'active'"
                      class="btn btn-secondary"
                      @click="open('allocate', row)"
                    >
                      {{ t("teams.allocate") }}</button
                    ><template
                      v-if="row.role === 'user' && row.status !== 'left'"
                      ><button
                        class="btn btn-secondary"
                        @click="
                          memberStatus(
                            row,
                            row.status === 'active' ? 'suspended' : 'active',
                          )
                        "
                      >
                        {{
                          t(
                            row.status === "active"
                              ? "teams.pause"
                              : "teams.resume",
                          )
                        }}</button
                      ><button
                        class="btn btn-secondary text-red-600"
                        @click="memberStatus(row, 'left')"
                      >
                        {{ t("teams.remove") }}
                      </button></template
                    >
                  </div>
                </td>
                <td v-if="section === 'keys'" class="px-4 py-3">
                  <div class="flex gap-2">
                    <button
                      v-if="canManage && row.member_status === 'active'"
                      class="btn btn-secondary"
                      @click="open('editKey', row)"
                    >
                      {{ t("teams.edit") }}</button
                    ><button
                      v-if="
                        row.user_id === auth.user?.id &&
                        row.member_status === 'active'
                      "
                      class="btn btn-secondary"
                      @click="showSecret(row)"
                    >
                      {{ t("teams.copy") }}
                    </button>
                  </div>
                </td>
                <td
                  v-if="section === 'invitations' && canManage"
                  class="px-4 py-3"
                >
                  <button
                    v-if="!row.accepted_at && !row.revoked_at"
                    class="btn btn-secondary"
                    @click="
                      confirmAction(t('teams.confirmHint'), () =>
                        request(
                          'delete',
                          `/teams/${teamID}/invitations/${row.id}`,
                        ),
                      )
                    "
                  >
                    {{ t("teams.revoke") }}
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
          <p
            v-if="!loading && !records.length"
            class="p-10 text-center text-gray-500"
          >
            {{ t("teams.empty") }}
          </p>
        </div>
        <div
          v-if="section !== 'overview'"
          class="flex items-center justify-between"
        >
          <span class="text-sm text-gray-500">{{
            t("teams.total", { count: total })
          }}</span>
          <div class="flex gap-2">
            <button
              class="btn btn-secondary"
              :disabled="page === 1 || loading"
              @click="
                page--;
                loadRows();
              "
            >
              {{ t("teams.previous") }}</button
            ><button
              class="btn btn-secondary"
              :disabled="page * 25 >= total || loading"
              @click="
                page++;
                loadRows();
              "
            >
              {{ t("teams.next") }}
            </button>
          </div>
        </div>
      </template>
    </div>
    <Teleport to="body">
      <div
        v-if="actionMenuTeam"
        id="team-action-menu"
        ref="actionMenuRef"
        role="menu"
        :aria-label="`${actionMenuTeam.name} · ${t('teams.actions')}`"
        class="fixed z-[9999] w-60 max-w-[calc(100vw-16px)] overflow-y-auto rounded-xl bg-white py-1 shadow-lg ring-1 ring-black/5 dark:bg-dark-800 dark:ring-white/10"
        :style="{
          top: `${menuPosition.top}px`,
          left: `${menuPosition.left}px`,
          maxHeight: 'calc(100vh - 16px)',
        }"
        @keydown="handleMenuKeydown"
      >
        <button
          v-for="action in teamMenuActions"
          :key="action.key"
          role="menuitem"
          class="flex w-full items-center gap-2 px-4 py-2 text-left text-sm text-gray-700 hover:bg-gray-100 focus-visible:bg-gray-100 focus-visible:outline-none dark:text-gray-300 dark:hover:bg-dark-700 dark:focus-visible:bg-dark-700"
          @click="openTeamAction(action.key, actionMenuTeam)"
        >
          <Icon :name="action.icon" size="sm" :class="action.color" />
          {{ t(`teams.${action.key}`) }}
        </button>
      </div>
    </Teleport>
    <BaseDialog
      :show="!!dialog"
      :title="dialog ? t(`teams.${dialog}`) : ''"
      width="normal"
      @close="closeDialog"
    >
      <form id="team-form" class="space-y-4" @submit.prevent="submit">
        <div
          v-if="platform && dialogTeam"
          class="rounded-lg bg-gray-50 px-4 py-3 text-sm dark:bg-dark-700"
        >
          <p class="text-gray-500 dark:text-gray-400">
            {{ t("teams.targetTeam") }}
          </p>
          <p class="mt-1 break-words font-medium text-gray-900 dark:text-white">
            {{ dialogTeam.name }}
          </p>
          <p
            v-if="dialog === 'fund'"
            class="mt-2 text-gray-500 dark:text-gray-400"
          >
            {{ t("teams.available") }}: {{ money(dialogTeam.balance) }}
          </p>
        </div>
        <p v-if="formError" role="alert" class="text-sm text-red-600">
          {{ formError }}
        </p>
        <template v-if="dialog === 'createTeam' || dialog === 'editTeam'">
          <label class="block"
            ><span class="input-label">{{ t("teams.name") }}</span
            ><input v-model="form.name" class="input" required maxlength="100"
          /></label>
          <label class="block"
            ><span class="input-label">{{ t("teams.status") }}</span
            ><Select v-model="form.status" :options="statusOptions"
          /></label>
          <p v-if="dialog === 'createTeam'" class="text-sm text-gray-500">
            {{ t("teams.adminSelectionHint") }}
            <router-link
              to="/admin/users"
              target="_blank"
              rel="noopener noreferrer"
              class="text-primary-600 hover:underline"
              >{{ t("teams.openUserManagement") }}</router-link
            >
          </p>
          <label v-if="dialog === 'createTeam'" class="block"
            ><span class="input-label">{{ t("teams.adminUser") }}</span
            ><input
              v-model="userSearch"
              class="input mb-2"
              :placeholder="t('teams.searchUser')"
              @input="searchUsers" /><Select
              v-model="form.user_id"
              :options="userOptions"
              :loading="usersLoading"
              :disabled="usersLoading"
              :placeholder="t('teams.selectAdmin')"
              :empty-text="t('teams.noAdminCandidates')"
          /></label>
          <p
            v-if="dialog === 'createTeam' && adminSearchWarning"
            role="status"
            class="text-sm text-amber-700 dark:text-amber-400"
          >
            {{ adminSearchWarning }}
          </p>
          <div>
            <span class="input-label">{{ t("teams.groups") }}</span>
            <div
              class="max-h-48 space-y-2 overflow-y-auto rounded-lg border p-3 dark:border-dark-600"
            >
              <label
                v-for="group in platformGroups"
                :key="group.id"
                class="flex items-center gap-2 text-sm"
                ><input
                  v-model="form.group_ids"
                  type="checkbox"
                  :value="group.id"
                />{{ group.name }}</label
              >
            </div>
          </div>
        </template>
        <template v-if="dialog === 'appoint'"
          ><p class="text-sm text-gray-500">{{ t("teams.adminHint") }}</p>
          <input
            v-model="userSearch"
            class="input"
            :placeholder="t('teams.searchUser')"
            @input="searchUsers" /><Select
            v-model="form.user_id"
            :options="userOptions"
            :loading="usersLoading"
            :disabled="usersLoading"
            :placeholder="t('teams.selectAdmin')"
            :empty-text="t('teams.noAdminCandidates')" />
          <p
            v-if="adminSearchWarning"
            role="status"
            class="text-sm text-amber-700 dark:text-amber-400"
          >
            {{ adminSearchWarning }}
          </p>
          <Select
            v-model="form.promote"
            :options="[
              { value: 'yes', label: t('teams.promote') },
              { value: 'no', label: t('teams.demote') },
            ]"
        /></template>
        <template v-if="dialog === 'fund'"
          ><p class="text-sm text-gray-500">{{ t("teams.fundHint") }}</p>
          <label class="block"
            ><span class="input-label">{{ t("teams.amount") }}</span
            ><input
              v-model="form.amount"
              type="number"
              step="0.00000001"
              class="input"
              required /></label
        ></template>
        <template v-if="dialog === 'grant'"
          ><label class="block"
            ><span class="input-label">{{ t("teams.group") }}</span
            ><Select
              v-model="form.group_id"
              :options="groupOptions.filter((g) => g.subscription)" /></label
          ><label class="block"
            ><span class="input-label">{{ t("teams.days") }}</span
            ><input
              v-model.number="form.days"
              type="number"
              min="1"
              max="36500"
              class="input"
              required /></label
          ><label class="block"
            ><span class="input-label">{{ t("teams.quantity") }}</span
            ><input
              v-model.number="form.quantity"
              type="number"
              min="1"
              max="100000"
              class="input"
              required /></label
        ></template>
        <template v-if="dialog === 'createMember' || dialog === 'invite'"
          ><label class="block"
            ><span class="input-label">{{ t("teams.email") }}</span
            ><input
              v-model="form.email"
              type="email"
              class="input"
              required
              maxlength="255" /></label
          ><template v-if="dialog === 'createMember'"
            ><label class="block"
              ><span class="input-label">{{ t("teams.username") }}</span
              ><input
                v-model="form.username"
                class="input"
                maxlength="100" /></label
            ><label class="block"
              ><span class="input-label">{{ t("teams.password") }}</span
              ><input
                v-model="form.password"
                type="password"
                autocomplete="new-password"
                minlength="8"
                maxlength="72"
                class="input"
                required /></label
          ></template>
          <p v-else class="text-sm text-gray-500">
            {{ t("teams.inviteHint") }}
          </p></template
        >
        <template v-if="dialog === 'allocate'"
          ><p class="text-sm text-gray-500">{{ t("teams.allocationHint") }}</p>
          <label class="block"
            ><span class="input-label">{{ t("teams.member") }}</span
            ><Select
              v-model="form.member_id"
              :options="activeMemberOptions" /></label
          ><Select
            v-model="form.kind"
            :options="[
              { value: 'balance', label: t('teams.balance') },
              { value: 'subscription', label: t('teams.subscription') },
            ]"
          /><label v-if="form.kind === 'balance'" class="block"
            ><span class="input-label">{{ t("teams.amount") }}</span
            ><input
              v-model="form.amount"
              type="number"
              min="0.00000001"
              step="0.00000001"
              class="input"
              required /></label
          ><label v-else class="block"
            ><span class="input-label">{{ t("teams.grantSelect") }}</span
            ><Select v-model="form.grant_id" :options="grantOptions"
          /></label>
          <div class="rounded-xl bg-gray-50 p-4 text-sm dark:bg-dark-700">
            <p>{{ t("teams.source") }}: {{ overview?.name }}</p>
            <p class="mt-2">
              {{ t("teams.after") }}: {{ allocationRemaining }}
            </p>
            <p
              v-if="
                allocationInvalid &&
                form.member_id &&
                (form.amount || form.grant_id)
              "
              class="mt-2 text-red-600"
            >
              {{ t("teams.insufficient") }}
            </p>
          </div></template
        >
        <template v-if="dialog === 'createKey' || dialog === 'editKey'">
          <label class="block"
            ><span class="input-label">{{ t("teams.member") }}</span
            ><Select
              v-model="form.member_id"
              :options="activeMemberOptions"
              :disabled="dialog === 'editKey'"
          /></label>
          <label class="block"
            ><span class="input-label">{{ t("teams.name") }}</span
            ><input v-model="form.name" class="input" required maxlength="100"
          /></label>
          <label class="block"
            ><span class="input-label">{{ t("teams.group") }}</span
            ><Select v-model="form.group_id" :options="groupOptions"
          /></label>
          <Select v-model="form.status" :options="statusOptions" />
          <div class="grid grid-cols-2 gap-3">
            <label v-for="limit in limitFields" :key="limit.key"
              ><span class="input-label">{{ t(`teams.${limit.label}`) }}</span
              ><input
                v-model.number="form[limit.key]"
                type="number"
                min="0"
                step="0.00000001"
                class="input"
                required
            /></label>
          </div>
          <label class="block"
            ><span class="input-label">{{ t("teams.expires") }}</span
            ><input v-model="form.expires" type="datetime-local" class="input"
          /></label>
        </template>
      </form>
      <template #footer
        ><button
          class="btn btn-secondary"
          :disabled="saving"
          @click="closeDialog"
        >
          {{ t("teams.cancel") }}</button
        ><button
          type="submit"
          form="team-form"
          class="btn btn-primary"
          :disabled="
            saving || !formReady || allocationInvalid || adminSelectionInvalid
          "
        >
          {{ t("teams.save") }}
        </button></template
      >
    </BaseDialog>
    <BaseDialog :show="!!reveal" :title="revealTitle" @close="reveal = ''"
      ><p class="mb-4 text-sm text-gray-500">
        {{
          revealTitle === t("teams.inviteLink")
            ? t("teams.inviteHint")
            : t("teams.keyHint")
        }}
      </p>
      <textarea
        :value="reveal"
        readonly
        class="input min-h-24 font-mono text-sm"
      /><template #footer
        ><button class="btn btn-primary" @click="copy(reveal)">
          {{ t("teams.copy") }}
        </button></template
      ></BaseDialog
    >
    <BaseDialog
      :show="!!confirmation"
      :title="t('teams.confirm')"
      @close="confirmation = null"
      ><p>{{ confirmation?.text }}</p>
      <template #footer
        ><button
          class="btn btn-secondary"
          :disabled="saving"
          @click="confirmation = null"
        >
          {{ t("teams.cancel") }}</button
        ><button
          class="btn btn-primary"
          :disabled="saving"
          @click="runConfirmation"
        >
          {{ t("teams.confirm") }}
        </button></template
      ></BaseDialog
    >
    <TotpStepUpDialog :controller="stepUp" />
  </AppLayout>
</template>

<script setup lang="ts">
import {
  computed,
  nextTick,
  onMounted,
  onUnmounted,
  reactive,
  ref,
  watch,
} from "vue";
import { onClickOutside, useEventListener } from "@vueuse/core";
import { useRoute, useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import AppLayout from "@/components/layout/AppLayout.vue";
import BaseDialog from "@/components/common/BaseDialog.vue";
import Select from "@/components/common/Select.vue";
import DataTable from "@/components/common/DataTable.vue";
import Icon from "@/components/icons/Icon.vue";
import TotpStepUpDialog from "@/components/auth/TotpStepUpDialog.vue";
import { useStepUp, isStepUpCancelled } from "@/composables/useStepUp";
import { useAuthStore } from "@/stores/auth";
import { useAppStore } from "@/stores/app";
import { enterprisesAPI, type Enterprise } from "@/api/enterprises";
import { apiClient } from "@/api/client";
import {
  teamsAPI,
  type TeamRow,
  type TeamOverview,
  type TeamGroup,
} from "@/api/teams";

const { t, te } = useI18n();
const auth = useAuthStore(),
  app = useAppStore(),
  route = useRoute(),
  router = useRouter(),
  stepUp = useStepUp();
const platform = computed(() => route.name === "AdminTeams");
const mine = computed(() => !platform.value && !auth.isTeamAdmin);
const enterpriseFilter = ref(0),
  enterpriseOptions = ref<Enterprise[]>([]);
const teamID = ref(0),
  teams = ref<TeamRow[]>([]),
  overview = ref<TeamOverview | null>(null);
const loading = ref(false),
  saving = ref(false),
  error = ref(""),
  formError = ref(""),
  section = ref("overview"),
  page = ref(1),
  total = ref(0);
const records = ref<TeamRow[]>([]),
  members = ref<TeamRow[]>([]),
  grants = ref<TeamRow[]>([]),
  keys = ref<TeamRow[]>([]);
const platformGroups = ref<TeamGroup[]>([]),
  users = ref<TeamRow[]>([]),
  usersLoading = ref(false),
  userSearch = ref("");
const dialogTeam = ref<TeamOverview | null>(null);
const formReady = ref(false);
const actionLoadingID = ref(0);
const actionMenuTeam = ref<TeamRow | null>(null);
const actionMenuRef = ref<HTMLElement | null>(null);
const menuPosition = ref({ top: 0, left: 0 });
let actionMenuTrigger: HTMLElement | null = null;
let actionMenuAnchor: DOMRect | null = null;
let actionEpoch = 0;
let dialogEpoch = 0;
const teamColumns = computed(() => [
  { key: "name", label: t("teams.name") },
  { key: "administrators", label: t("teams.administrators") },
  { key: "enterprise_name", label: t("enterprise.filter") },
  { key: "status", label: t("teams.status") },
  { key: "member_count", label: t("teams.members") },
  { key: "balance", label: t("teams.available") },
  { key: "actions", label: t("teams.actions") },
]);
const teamMenuActions = [
  { key: "appoint", icon: "users", color: "text-orange-500" },
  { key: "fund", icon: "dollar", color: "text-emerald-500" },
  { key: "grant", icon: "gift", color: "text-indigo-500" },
] as const;
const dialog = ref(""),
  reveal = ref(""),
  revealTitle = ref(""),
  editingID = ref(0);
const confirmation = ref<{
  text: string;
  action: () => Promise<unknown>;
} | null>(null);
const filters = reactive({
  from: "",
  to: "",
  member_id: 0,
  key_id: 0,
  model: "",
});
const form = reactive({
  name: "",
  status: "active",
  user_id: 0,
  group_ids: [] as number[],
  promote: "yes",
  amount: "",
  group_id: 0,
  days: 30,
  quantity: 1,
  email: "",
  username: "",
  password: "",
  member_id: 0,
  kind: "balance",
  grant_id: 0,
  quota: 0,
  rate_limit_5h: 0,
  rate_limit_1d: 0,
  rate_limit_7d: 0,
  expires: "",
});
let operationID = "",
  epoch = 0,
  searchEpoch = 0;
const canManage = computed(
  () =>
    overview.value?.can_manage === true &&
    (platform.value || auth.user?.team?.enterprise_status !== "disabled") &&
    (platform.value ? auth.isAdmin : auth.isTeamAdmin),
);
const canViewTeam = computed(
  () =>
    overview.value?.can_view_team === true &&
    (platform.value ? auth.isAdmin : auth.isTeamAdmin),
);
const tabs = computed(() =>
  canViewTeam.value
    ? [
        "overview",
        "members",
        "keys",
        "grants",
        "usage",
        "ledger",
        "invitations",
      ]
    : ["keys", "usage", "ledger"],
);
const statusOptions = computed(() =>
  ["active", "disabled"].map((value) => ({
    value,
    label: t(`teams.${value}`),
  })),
);
const memberOptions = computed(() =>
  members.value.map((m) => ({ value: m.id, label: m.email || String(m.id) })),
);
const activeMemberOptions = computed(() =>
  members.value
    .filter((m) => m.status === "active")
    .map((m) => ({ value: m.id, label: m.email || String(m.id) })),
);
const userOptions = computed(() =>
  users.value.map((u) => {
    const reason = adminUnavailableReason(u);
    return {
      value: u.id,
      label: `${u.email || u.id}${reason ? `（${reason}）` : ""}`,
      disabled: !!reason,
    };
  }),
);
function adminUnavailableReason(user: TeamRow): string {
  if (user.role === "enterprise_admin") return t("enterprise.roleUnavailable");
  if (user.role === "admin") return t("teams.platformAdminUnavailable");
  if (
    user.status !== "active" &&
    !(dialog.value === "appoint" && !form.promote && user.role === "team_admin")
  )
    return t("teams.inactiveAdminUnavailable");
  if (dialog.value === "createTeam" && user.role === "team_admin")
    return t("teams.assignedAdminUnavailable");
  return "";
}
const adminSearchWarning = computed(() => {
  const exact = users.value.find(
    (user) =>
      user.email?.toLowerCase() === userSearch.value.trim().toLowerCase(),
  );
  return exact ? adminUnavailableReason(exact) : "";
});
const adminSelectionInvalid = computed(() => {
  if (!["createTeam", "appoint"].includes(dialog.value)) return false;
  const selected = users.value.find((user) => user.id === form.user_id);
  return usersLoading.value || !selected || !!adminUnavailableReason(selected);
});
const groupOptions = computed(() =>
  (dialogTeam.value?.groups || overview.value?.groups || [])
    .filter((g) => g.status === "active")
    .map((g) => ({
      value: g.id,
      label: g.name,
      subscription: g.subscription_type === "subscription",
    })),
);
const grantOptions = computed(() =>
  grants.value
    .filter((g) => (g.remaining || 0) > 0)
    .map((g) => ({
      value: g.id,
      label: `${g.group_name} · ${g.validity_days} ${t("teams.days")} · ${g.remaining} ${t("teams.remaining")}`,
    })),
);
const keyOptions = computed(() =>
  keys.value.map((k) => ({ value: k.id, label: `${k.name} (${k.id})` })),
);
const allocationRemaining = computed(() =>
  form.kind === "balance"
    ? money(Number(overview.value?.balance || 0) - Number(form.amount || 0))
    : String(
        (grants.value.find((g) => g.id === form.grant_id)?.remaining || 0) - 1,
      ),
);
const allocationInvalid = computed(
  () =>
    dialog.value === "allocate" &&
    (!form.member_id ||
      (form.kind === "balance"
        ? Number(form.amount) <= 0 ||
          Number(form.amount) > Number(overview.value?.balance || 0)
        : !form.grant_id ||
          !grants.value.find((g) => g.id === form.grant_id)?.remaining)),
);
const limitFields = [
  { key: "quota", label: "quota" },
  { key: "rate_limit_5h", label: "rate5h" },
  { key: "rate_limit_1d", label: "rate1d" },
  { key: "rate_limit_7d", label: "rate7d" },
] as const;
const columnSets: Record<string, { key: keyof TeamRow; label: string }[]> = {
  members: [
    { key: "email", label: "email" },
    { key: "username", label: "username" },
    { key: "role", label: "user" },
    { key: "status", label: "status" },
    { key: "allocated_balance", label: "allocated" },
  ],
  keys: [
    { key: "name", label: "name" },
    { key: "email", label: "member" },
    { key: "masked_key", label: "keys" },
    { key: "group_name", label: "group" },
    { key: "status", label: "status" },
    { key: "quota_used", label: "used" },
    { key: "quota", label: "quota" },
    { key: "expires_at", label: "expires" },
  ],
  grants: [
    { key: "group_name", label: "group" },
    { key: "validity_days", label: "days" },
    { key: "quantity", label: "quantity" },
    { key: "remaining", label: "remaining" },
  ],
  invitations: [
    { key: "email", label: "email" },
    { key: "status", label: "status" },
    { key: "expires_at", label: "expires" },
  ],
  usage: [
    { key: "email", label: "member" },
    { key: "api_key_id", label: "keys" },
    { key: "model", label: "model" },
    { key: "input_tokens", label: "inputTokens" },
    { key: "output_tokens", label: "outputTokens" },
    { key: "cache_tokens", label: "cacheTokens" },
    { key: "actual_cost", label: "cost" },
    { key: "created_at", label: "when" },
  ],
  ledger: [
    { key: "kind", label: "event" },
    { key: "outcome", label: "result" },
    { key: "actor_email", label: "actor" },
    { key: "member_email", label: "recipient" },
    { key: "amount", label: "amount" },
    { key: "balance_after", label: "balanceChange" },
    { key: "details", label: "details" },
    { key: "created_at", label: "when" },
  ],
};
const columns = computed(() => columnSets[section.value] || []);
function money(v: unknown) {
  return `$${Number(v || 0).toLocaleString(undefined, { maximumFractionDigits: 8 })}`;
}
function label(v?: string) {
  return v && te(`teams.${v}`) ? t(`teams.${v}`) : v || "—";
}
function cell(row: TeamRow, key: keyof TeamRow): string {
  const value = row[key];
  if (key === "status" && section.value === "invitations")
    return t(
      `teams.${row.accepted_at ? "accepted" : row.revoked_at ? "revoked" : new Date(row.expires_at || "").getTime() < Date.now() ? "expired" : "pending"}`,
    );
  if (key === "role") return t(`admin.users.roles.${String(value)}`);
  if (key === "status" || key === "outcome") return label(String(value));
  if (key === "kind") return label(String(value).replace(/\./g, "_"));
  if (key === "details") return eventDetails(row);
  if (key === "balance_after" && value != null)
    return `${money(row.balance_before)} → ${money(value)}`;
  if (
    [
      "amount",
      "balance_after",
      "allocated_balance",
      "actual_cost",
      "quota_used",
    ].includes(key)
  )
    return value == null ? "—" : money(value);
  if (key === "created_at" || key === "expires_at")
    return value ? new Date(String(value)).toLocaleString() : "—";
  return value == null || value === "" ? "—" : String(value);
}
function eventDetails(row: TeamRow) {
  const d = row.details || {},
    kind = row.kind || "";
  if (kind === "administrator.change")
    return `${d.email || ""} · ${t(`admin.users.roles.${d.before}`)} → ${t(`admin.users.roles.${d.after}`)}`;
  if (kind.startsWith("member."))
    return d.before
      ? `${label(String(d.before))} → ${label(String(d.after))}`
      : String(d.email || "—");
  if (kind.startsWith("subscription."))
    return t("teams.subscriptionInfo", {
      group:
        overview.value?.groups.find((g) => g.id === d.group_id)?.name ||
        d.group_id ||
        "—",
      days: d.validity_days || "—",
    });
  if (kind.startsWith("invitation.")) return String(d.email || "—");
  if (kind === "team.save" || kind === "key.save") {
    const before = d.before as Record<string, unknown> | undefined,
      after = (d.after || d.settings) as Record<string, unknown> | undefined;
    if (!after) return "—";
    const fields: Record<string, string> =
      kind === "team.save"
        ? { name: "name", status: "status", group_ids: "groups" }
        : {
            name: "name",
            status: "status",
            group_id: "group",
            quota: "quota",
            rate_limit_5h: "rate5h",
            rate_limit_1d: "rate1d",
            rate_limit_7d: "rate7d",
            expires_at: "expires",
          };
    const value = (field: string, v: unknown): string => {
      if (v == null) return "—";
      if (field === "status") return label(String(v));
      if (field === "group_id")
        return (
          overview.value?.groups.find((g) => g.id === v)?.name || String(v)
        );
      if (field === "group_ids" && Array.isArray(v))
        return v
          .map(
            (id) => overview.value?.groups.find((g) => g.id === id)?.name || id,
          )
          .join(", ");
      if (field === "expires_at") return new Date(String(v)).toLocaleString();
      return String(v);
    };
    return (
      Object.entries(fields)
        .filter(
          ([field]) =>
            !before ||
            JSON.stringify(before[field]) !== JSON.stringify(after[field]),
        )
        .map(
          ([field, title]) =>
            `${t(`teams.${title}`)}: ${before ? value(field, before[field]) + " → " : ""}${value(field, after[field])}`,
        )
        .join(" · ") || "—"
    );
  }
  return "—";
}
function message(e: unknown) {
  return (e as { message?: string })?.message || t("teams.failed");
}
async function allRows(kind: string): Promise<TeamRow[]> {
  const result: TeamRow[] = [];
  let p = 1;
  const id = teamID.value,
    ownOnly = mine.value;
  for (;;) {
    const data = await teamsAPI.rows(id, kind, {
      page: p,
      page_size: 200,
      mine: ownOnly,
    });
    result.push(...data.items);
    if (result.length >= data.total || !data.items.length) return result;
    p++;
  }
}
async function initialize() {
  closeActionMenu();
  actionEpoch++;
  actionLoadingID.value = 0;
  const stamp = ++epoch;
  loading.value = true;
  error.value = "";
  overview.value = null;
  records.value = [];
  members.value = [];
  grants.value = [];
  keys.value = [];
  try {
    await auth.refreshUser();
    if (stamp !== epoch) return;
    if (route.name === "TeamInvitation") return;
    if (route.name === "MyTeam" && auth.isEnterpriseAdmin) {
      await router.replace("/my-enterprise");
      return;
    }
    if (platform.value) {
      teamID.value = Number(route.query.team || 0);
      if (!teamID.value) {
        const data = await teamsAPI.list(page.value, enterpriseFilter.value);
        const options: Enterprise[] = [];
        for (let p = 1; ; p++) {
          const res = await enterprisesAPI.list({ page: p, page_size: 200 });
          options.push(...res.items);
          if (options.length >= res.total || !res.items.length) break;
        }
        enterpriseOptions.value = options;
        if (stamp !== epoch) return;
        teams.value = data.items;
        total.value = data.total;
        return;
      }
    } else {
      const current = await teamsAPI.current();
      if (stamp !== epoch) return;
      teamID.value = current?.id || 0;
      if (!teamID.value) return;
    }
    const data = await teamsAPI.overview(teamID.value, mine.value);
    if (stamp !== epoch) return;
    overview.value = data;
    if (canViewTeam.value) {
      const collections = await Promise.all([
        allRows("members"),
        allRows("grants"),
        allRows("keys"),
      ]);
      if (stamp !== epoch) return;
      [members.value, grants.value, keys.value] = collections;
    } else {
      const ownKeys = await allRows("keys");
      if (stamp !== epoch) return;
      keys.value = ownKeys;
    }
    if (!tabs.value.includes(section.value)) section.value = tabs.value[0];
    await loadRows();
  } catch (e) {
    if (stamp === epoch) error.value = message(e);
  } finally {
    if (stamp === epoch) loading.value = false;
  }
}
function params(): Record<string, unknown> {
  return {
    mine: mine.value,
    page: page.value,
    page_size: 25,
    member_id: filters.member_id,
    key_id: filters.key_id,
    model: filters.model,
    from: filters.from
      ? new Date(`${filters.from}T00:00:00`).toISOString()
      : undefined,
    to: filters.to
      ? new Date(
          new Date(`${filters.to}T00:00:00`).getTime() + 86400000,
        ).toISOString()
      : undefined,
  };
}
async function loadRows() {
  if (!teamID.value || section.value === "overview") return;
  const stamp = epoch;
  loading.value = true;
  try {
    const data = await teamsAPI.rows(teamID.value, section.value, params());
    if (stamp !== epoch) return;
    records.value = data.items;
    total.value = data.total;
  } catch (e) {
    if (stamp === epoch) error.value = message(e);
  } finally {
    if (stamp === epoch) loading.value = false;
  }
}
async function changeSection(tab: string) {
  section.value = tab;
  page.value = 1;
  await loadRows();
}
function selectTeam(id: number) {
  page.value = 1;
  void router.replace({ name: "AdminTeams", query: id ? { team: id } : {} });
}
function closeActionMenu() {
  actionMenuTeam.value = null;
}
async function toggleActionMenu(team: TeamRow, event: MouseEvent) {
  if (actionMenuTeam.value?.id === team.id) {
    closeActionMenu();
    return;
  }
  actionMenuTrigger = event.currentTarget as HTMLElement;
  const anchor = actionMenuTrigger.getBoundingClientRect();
  actionMenuAnchor = anchor;
  actionMenuTeam.value = team;
  await nextTick();
  if (!actionMenuRef.value || actionMenuTeam.value?.id !== team.id) return;
  const { width, height } = actionMenuRef.value.getBoundingClientRect();
  const padding = 8;
  const top =
    anchor.bottom + height + padding + 4 <= window.innerHeight
      ? anchor.bottom + 4
      : anchor.top - height - 4;
  menuPosition.value = {
    top: Math.max(
      padding,
      Math.min(top, window.innerHeight - height - padding),
    ),
    left: Math.max(
      padding,
      Math.min(anchor.right - width, window.innerWidth - width - padding),
    ),
  };
  await nextTick();
  actionMenuRef.value
    ?.querySelector<HTMLButtonElement>("button")
    ?.focus({ preventScroll: true });
}
function handleMenuKeydown(event: KeyboardEvent) {
  if (event.key === "Escape" || event.key === "Tab") {
    if (event.key === "Escape") event.preventDefault();
    actionMenuTrigger?.focus({ preventScroll: true });
    closeActionMenu();
    return;
  }
  const buttons = Array.from(
    actionMenuRef.value?.querySelectorAll<HTMLButtonElement>("button") || [],
  );
  if (
    !buttons.length ||
    !["ArrowDown", "ArrowUp", "Home", "End"].includes(event.key)
  )
    return;
  event.preventDefault();
  const current = buttons.indexOf(document.activeElement as HTMLButtonElement);
  const index =
    event.key === "Home"
      ? 0
      : event.key === "End"
        ? buttons.length - 1
        : (current + (event.key === "ArrowDown" ? 1 : -1) + buttons.length) %
          buttons.length;
  buttons[index]?.focus();
}
onClickOutside(actionMenuRef, closeActionMenu, {
  ignore: [".team-action-trigger"],
});
useEventListener(window, "resize", closeActionMenu);
useEventListener(
  window,
  "scroll",
  (event) => {
    if (!actionMenuTeam.value || !actionMenuTrigger || !actionMenuAnchor)
      return;
    if (
      event.target instanceof Node &&
      actionMenuRef.value?.contains(event.target)
    )
      return;
    const anchor = actionMenuTrigger.getBoundingClientRect();
    if (
      anchor.top !== actionMenuAnchor.top ||
      anchor.left !== actionMenuAnchor.left
    )
      closeActionMenu();
  },
  { capture: true, passive: true },
);
async function openTeamAction(action: string, team: TeamRow) {
  if (actionLoadingID.value) return;
  closeActionMenu();
  const stamp = ++actionEpoch;
  actionLoadingID.value = team.id;
  try {
    const target = await teamsAPI.overview(team.id);
    if (stamp !== actionEpoch) return;
    await open(action, undefined, target);
  } catch (e) {
    if (stamp === actionEpoch) app.showError(message(e));
  } finally {
    if (stamp === actionEpoch) actionLoadingID.value = 0;
  }
}
async function searchUsers() {
  const stamp = ++searchEpoch;
  form.user_id = 0;
  users.value = [];
  usersLoading.value = true;
  try {
    const data = await apiClient.get<{ items: TeamRow[] }>("/admin/users", {
      params: { search: userSearch.value, page_size: 100 },
    });
    if (stamp === searchEpoch) users.value = data.data.items;
  } catch (e) {
    if (stamp === searchEpoch) formError.value = message(e);
  } finally {
    if (stamp === searchEpoch) usersLoading.value = false;
  }
}
async function open(action: string, row?: TeamRow, target = overview.value) {
  const stamp = ++dialogEpoch;
  closeActionMenu();
  dialogTeam.value = action === "createTeam" ? null : target;
  formReady.value = false;
  platformGroups.value = [];
  formError.value = "";
  userSearch.value = "";
  users.value = [];
  usersLoading.value = false;
  searchEpoch++;
  operationID = crypto.randomUUID();
  editingID.value = row?.id || 0;
  Object.assign(form, {
    name: action === "editTeam" ? target?.name || "" : row?.name || "",
    status:
      row?.status === "disabled"
        ? "disabled"
        : action === "editTeam"
          ? target?.status || "active"
          : "active",
    user_id: 0,
    group_ids: target?.groups.map((g) => g.id) || [],
    promote: "yes",
    amount: "",
    group_id: row?.group_id || 0,
    days: 30,
    quantity: 1,
    email: "",
    username: "",
    password: "",
    member_id: action === "allocate" ? row?.id || 0 : row?.member_id || 0,
    kind: "balance",
    grant_id: 0,
    quota: row?.quota || 0,
    rate_limit_5h: row?.rate_limit_5h || 0,
    rate_limit_1d: row?.rate_limit_1d || 0,
    rate_limit_7d: row?.rate_limit_7d || 0,
    expires: row?.expires_at
      ? new Date(
          new Date(row.expires_at).getTime() -
            new Date().getTimezoneOffset() * 60000,
        )
          .toISOString()
          .slice(0, 16)
      : "",
  });
  dialog.value = action;
  try {
    if (["createTeam", "editTeam"].includes(action)) {
      const { data } = await apiClient.get<TeamGroup[]>("/admin/groups/all");
      if (stamp !== dialogEpoch) return;
      platformGroups.value = data;
    }
    if (["createTeam", "appoint"].includes(action)) await searchUsers();
    if (stamp === dialogEpoch) formReady.value = true;
  } catch (e) {
    if (stamp === dialogEpoch) formError.value = message(e);
  }
}
function closeDialog() {
  if (!saving.value) {
    dialogEpoch++;
    searchEpoch++;
    dialog.value = "";
    dialogTeam.value = null;
    form.password = "";
  }
}
async function request(
  method: "post" | "put" | "patch" | "delete",
  path: string,
  data?: unknown,
) {
  return stepUp.run(() => teamsAPI.save(method, path, data));
}
async function submit() {
  if (saving.value || !formReady.value) return;
  saving.value = true;
  formError.value = "";
  try {
    let result: Record<string, unknown> = {};
    const targetID = dialogTeam.value?.id || teamID.value;
    const base = `/teams/${targetID}`,
      admin = `/admin/teams/${targetID}`;
    switch (dialog.value) {
      case "createTeam":
      case "editTeam":
        if (dialog.value === "createTeam" && adminSelectionInvalid.value)
          throw new Error(t("teams.selectAdminRequired"));
        result = await request(
          dialog.value === "createTeam" ? "post" : "put",
          dialog.value === "createTeam" ? "/admin/teams" : admin,
          {
            name: form.name,
            status: form.status,
            admin_id: form.user_id,
            group_ids: form.group_ids,
          },
        );
        break;
      case "appoint":
        if (adminSelectionInvalid.value)
          throw new Error(t("teams.selectAdminRequired"));
        result = await request(
          "put",
          `${admin}/administrators/${form.user_id}`,
          { promote: form.promote === "yes" },
        );
        break;
      case "fund":
        result = await request("post", `${admin}/funding`, {
          amount: String(form.amount),
          operation_id: operationID,
        });
        break;
      case "grant":
        result = await request("post", `${admin}/grants`, {
          group_id: form.group_id,
          validity_days: form.days,
          quantity: form.quantity,
          operation_id: operationID,
        });
        break;
      case "createMember":
        result = await request("post", `${base}/members`, {
          email: form.email,
          username: form.username,
          password: form.password,
        });
        break;
      case "invite":
        result = await request("post", `${base}/invitations`, {
          email: form.email,
        });
        revealTitle.value = t("teams.inviteLink");
        reveal.value = `${location.origin}/team-invite?token=${encodeURIComponent(String(result.token))}`;
        break;
      case "allocate":
        result = await request("post", `${base}/allocations`, {
          member_id: form.member_id,
          kind: form.kind,
          amount: String(form.amount),
          grant_id: form.grant_id,
          operation_id: operationID,
        });
        break;
      case "createKey":
      case "editKey":
        result = await request(
          dialog.value === "createKey" ? "post" : "put",
          `${base}/keys${dialog.value === "editKey" ? `/${editingID.value}` : ""}`,
          {
            member_id: form.member_id,
            name: form.name,
            group_id: form.group_id,
            status: form.status,
            quota: form.quota,
            rate_limit_5h: form.rate_limit_5h,
            rate_limit_1d: form.rate_limit_1d,
            rate_limit_7d: form.rate_limit_7d,
            expires_at: form.expires
              ? new Date(form.expires).toISOString()
              : null,
          },
        );
        if (result.key) {
          revealTitle.value = t("teams.secret");
          reveal.value = String(result.key);
        }
        break;
    }
    dialog.value = "";
    dialogTeam.value = null;
    form.password = "";
    app.showSuccess(t("teams.success"));
    await initialize();
  } catch (e) {
    if (!isStepUpCancelled(e)) formError.value = message(e);
  } finally {
    saving.value = false;
  }
}
function confirmAction(text: string, action: () => Promise<unknown>) {
  confirmation.value = { text, action };
}
async function runConfirmation() {
  if (!confirmation.value) return;
  saving.value = true;
  try {
    await confirmation.value.action();
    confirmation.value = null;
    app.showSuccess(t("teams.success"));
    await initialize();
  } catch (e) {
    if (!isStepUpCancelled(e)) app.showError(message(e));
  } finally {
    saving.value = false;
  }
}
function memberStatus(row: TeamRow, status: string) {
  confirmAction(t("teams.confirmHint"), () =>
    request("patch", `/teams/${teamID.value}/members/${row.id}`, { status }),
  );
}
async function copy(value: string) {
  try {
    await navigator.clipboard.writeText(value);
    app.showSuccess(t("teams.copied"));
  } catch (e) {
    app.showError(message(e));
  }
}
async function showSecret(row: TeamRow) {
  try {
    revealTitle.value = t("teams.secret");
    reveal.value = await teamsAPI.secret(teamID.value, row.id);
  } catch (e) {
    app.showError(message(e));
  }
}
async function acceptInvite() {
  saving.value = true;
  try {
    await request("post", "/teams/invitations/accept", {
      token: String(route.query.token || ""),
    });
    await auth.refreshUser();
    await router.replace("/my-team");
  } catch (e) {
    error.value = message(e);
  } finally {
    saving.value = false;
  }
}
async function exportUsage() {
  saving.value = true;
  try {
    const blob = await teamsAPI.export(teamID.value, params());
    const url = URL.createObjectURL(blob),
      link = document.createElement("a");
    link.href = url;
    link.download = "team-usage.csv";
    link.click();
    URL.revokeObjectURL(url);
  } catch (e) {
    app.showError(message(e));
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
    Object.assign(filters, {
      from: "",
      to: "",
      member_id: 0,
      key_id: 0,
      model: "",
    });
    void initialize();
  },
);
watch(
  [() => auth.user?.role, () => auth.user?.team?.enterprise_status],
  (_next, previous) => {
    if (!previous[0] || route.name !== "MyTeam") return;
    closeDialog();
    confirmation.value = null;
    page.value = 1;
    section.value = "overview";
    Object.assign(filters, {
      from: "",
      to: "",
      member_id: 0,
      key_id: 0,
      model: "",
    });
    void initialize();
  },
);
onUnmounted(() => {
  epoch++;
  actionEpoch++;
  dialogEpoch++;
  searchEpoch++;
});
onMounted(initialize);
</script>
