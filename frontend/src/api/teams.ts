import { apiClient } from "./client";

export interface TeamSummary {
  enterprise_id?: number | null;
  enterprise_status?: string | null;
  id: number;
  name: string;
  status: string;
  membership_id?: number;
  member_status?: string;
  role?: "admin" | "enterprise_admin" | "team_admin" | "user";
}
export interface TeamGroup {
  id: number;
  name: string;
  subscription_type: string;
  status: string;
}
export interface TeamOverview extends TeamSummary {
  can_manage: boolean;
  can_view_team?: boolean;
  platform_admin: boolean;
  balance?: number;
  member_count?: number;
  remaining_subscriptions?: number;
  allocated_balance?: number;
  groups: TeamGroup[];
  usage: { requests: number; tokens: number; cost: number };
}
export interface TeamAdministrator {
  id: number;
  email: string;
  username: string;
  status: string;
  member_status: string;
}
export interface TeamRow {
  enterprise_id?: number | null;
  enterprise_name?: string | null;
  enterprise_status?: string | null;
  id: number;
  administrators?: TeamAdministrator[];
  name?: string;
  email?: string;
  username?: string;
  user_id?: number;
  role?: string;
  status?: string;
  member_status?: string;
  member_id?: number;
  member_count?: number;
  balance?: number;
  allocated_balance?: number;
  group_id?: number;
  group_name?: string;
  quantity?: number;
  remaining?: number;
  validity_days?: number;
  expires_at?: string | null;
  revoked_at?: string | null;
  accepted_at?: string | null;
  masked_key?: string;
  quota?: number;
  quota_used?: number;
  rate_limit_5h?: number;
  rate_limit_1d?: number;
  rate_limit_7d?: number;
  kind?: string;
  outcome?: string;
  amount?: number;
  member_email?: string;
  actor_email?: string;
  balance_before?: number;
  balance_after?: number;
  details?: Record<string, unknown>;
  model?: string;
  api_key_id?: number;
  input_tokens?: number;
  cache_tokens?: number;
  output_tokens?: number;
  actual_cost?: number;
  created_at?: string;
}
export interface TeamPage {
  items: TeamRow[];
  total: number;
  page: number;
  page_size: number;
}
export const teamsAPI = {
  current: async () =>
    (await apiClient.get<TeamSummary | null>("/teams/current")).data,
  list: async (page = 1, enterpriseID = 0) =>
    (
      await apiClient.get<TeamPage>("/admin/teams", {
        params: { page, page_size: 25, enterprise_id: enterpriseID },
      })
    ).data,
  overview: async (id: number, mine = false) =>
    (await apiClient.get<TeamOverview>(`/teams/${id}`, { params: { mine } }))
      .data,
  rows: async (
    id: number,
    kind: string,
    params: Record<string, unknown> = {},
  ) => (await apiClient.get<TeamPage>(`/teams/${id}/${kind}`, { params })).data,
  save: async (
    method: "post" | "put" | "patch" | "delete",
    path: string,
    data?: unknown,
  ) =>
    (
      await apiClient.request<Record<string, unknown>>({
        method,
        url: path,
        data,
      })
    ).data,
  secret: async (id: number, keyID: number) =>
    (await apiClient.get<{ key: string }>(`/teams/${id}/keys/${keyID}/secret`))
      .data.key,
  export: async (id: number, params: Record<string, unknown>) =>
    (
      await apiClient.get<Blob>(`/teams/${id}/usage/export`, {
        params,
        responseType: "blob",
      })
    ).data,
};
