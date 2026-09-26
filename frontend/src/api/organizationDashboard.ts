import { apiClient } from './client'
export type OrganizationKind = 'enterprise' | 'team'
export interface OrganizationMetrics {
  requests: number
  input_tokens: number
  output_tokens: number
  cache_creation_tokens: number
  cache_read_tokens: number
  tokens: number
  cost: number
  average_duration_ms: number
}
export interface OrganizationPoint extends OrganizationMetrics {
  date: string
}
export interface OrganizationModel extends OrganizationMetrics {
  model: string
}
export interface OrganizationTeam extends OrganizationMetrics {
  team_id: number
  name: string
  status: string
}
export interface OrganizationMember extends OrganizationMetrics {
  team_id: number
  team_name: string
  user_id: number
  username: string
  email: string
  status: string
}
export interface OrganizationDashboardData {
  scope: {
    kind: OrganizationKind
    id: number
    name: string
    status: string
    enterprise_status?: string
    can_view_members: boolean
  }
  resources: {
    team_count: number
    member_count: number
    team_key_count: number
    balance: number
    remaining_subscriptions: number
    allocated_balance: number
  }
  usage: OrganizationMetrics
  trend: OrganizationPoint[]
  models: OrganizationModel[]
  teams: OrganizationTeam[]
  members: OrganizationMember[]
}
export interface OrganizationMemberPage {
  items: OrganizationMember[]
  total: number
  page: number
  page_size: number
}
export interface OrganizationMemberDetail {
  member: OrganizationMember
  trend: OrganizationPoint[]
  models: OrganizationModel[]
}
export interface OrganizationParams {
  start_date: string
  end_date: string
  granularity: 'day' | 'hour'
  team_id?: number
  page?: number
  page_size?: number
}
function base(kind: OrganizationKind) {
  return kind === 'enterprise' ? '/enterprise/dashboard' : '/teams/current/dashboard'
}
export const organizationDashboardAPI = {
  dashboard: async (kind: OrganizationKind, params: OrganizationParams, signal?: AbortSignal) =>
    (await apiClient.get<OrganizationDashboardData>(base(kind), { params, signal })).data,
  members: async (kind: OrganizationKind, params: OrganizationParams, signal?: AbortSignal) =>
    (await apiClient.get<OrganizationMemberPage>(`${base(kind)}/members`, { params, signal })).data,
  member: async (
    kind: OrganizationKind,
    userID: number,
    params: OrganizationParams,
    signal?: AbortSignal
  ) =>
    (
      await apiClient.get<OrganizationMemberDetail>(`${base(kind)}/members/${userID}`, {
        params,
        signal
      })
    ).data
}
