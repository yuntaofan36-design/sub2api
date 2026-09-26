import { apiClient } from "./client";
import type { TeamGroup, TeamRow } from "./teams";

export interface EnterpriseSummary {
  id: number;
  name: string;
  status: string;
}
export interface EnterpriseAccount {
  id: number;
  email: string;
  username: string;
  status?: string;
  role?: string;
  member_status?: string;
  team_id?: number;
  team_name?: string;
}
export interface Enterprise extends EnterpriseSummary {
  allow_member_usage_view: boolean;
  leaders: EnterpriseAccount[];
  team_count?: number;
  can_govern?: boolean;
}
export interface EnterpriseStats {
  team_count: number;
  member_count: number;
  balance: number;
  remaining_subscriptions: number;
  requests: number;
  tokens: number;
  cost: number;
}
export interface EnterpriseEvent {
  outcome: string;
  id: string;
  team_id?: number;
  team_name?: string;
  actor_email: string;
  kind: string;
  details: Record<string, unknown>;
  created_at: string;
}
export interface EnterprisePage<T> {
  items: T[];
  total: number;
  page: number;
  page_size: number;
}
export interface GovernTeamDetail extends TeamRow {
  can_govern: boolean;
  groups: TeamGroup[];
  stats: EnterpriseStats;
}
export const enterprisesAPI = {
  list: async (params: Record<string, unknown> = {}) =>
    (
      await apiClient.get<EnterprisePage<Enterprise>>("/admin/enterprises", {
        params,
      })
    ).data,
  current: async (id?: number) =>
    (
      await apiClient.get<Enterprise>(
        id ? `/admin/enterprises/${id}` : "/enterprise/current",
      )
    ).data,
  rows: async <T>(
    id: number | undefined,
    kind: string,
    params: Record<string, unknown> = {},
  ) =>
    (
      await apiClient.get<EnterprisePage<T>>(
        `${id ? `/admin/enterprises/${id}` : "/enterprise"}/${kind}`,
        { params },
      )
    ).data,
  stats: async (id?: number, params: Record<string, unknown> = {}) =>
    (
      await apiClient.get<EnterpriseStats>(
        `${id ? `/admin/enterprises/${id}` : "/enterprise"}/stats`,
        { params },
      )
    ).data,
  candidates: async (search: string, page = 1) =>
    (
      await apiClient.get<EnterprisePage<EnterpriseAccount>>(
        "/admin/enterprises/leader-candidates",
        { params: { search, page, page_size: 100 } },
      )
    ).data,
  teamCandidates: async (id: number, page = 1) =>
    (
      await apiClient.get<EnterprisePage<EnterpriseAccount>>(
        `/enterprise/teams/${id}/administrator-candidates`,
        { params: { page, page_size: 200 } },
      )
    ).data,
  team: async (id: number) =>
    (await apiClient.get<GovernTeamDetail>(`/enterprise/teams/${id}`)).data,
  save: async (
    method: "post" | "put" | "patch",
    path: string,
    data?: unknown,
  ) => (await apiClient.request({ method, url: path, data })).data,
};
