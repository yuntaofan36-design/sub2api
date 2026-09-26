package team

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// These DTOs deliberately exclude keys, balances of individuals, request rows,
// upstream models/costs and platform-only dashboard fields.
type DashboardMetrics struct {
	Requests            int64   `json:"requests"`
	InputTokens         int64   `json:"input_tokens"`
	OutputTokens        int64   `json:"output_tokens"`
	CacheCreationTokens int64   `json:"cache_creation_tokens"`
	CacheReadTokens     int64   `json:"cache_read_tokens"`
	Tokens              int64   `json:"tokens"`
	Cost                float64 `json:"cost"`
	AverageDuration     float64 `json:"average_duration_ms"`
}
type DashboardResources struct {
	TeamCount              int64   `json:"team_count"`
	MemberCount            int64   `json:"member_count"`
	TeamKeyCount           int64   `json:"team_key_count"`
	Balance                float64 `json:"balance"`
	RemainingSubscriptions int64   `json:"remaining_subscriptions"`
	AllocatedBalance       float64 `json:"allocated_balance"`
}
type DashboardPoint struct {
	DashboardMetrics
	Date string `json:"date"`
}
type DashboardModel struct {
	DashboardMetrics
	Model string `json:"model"`
}
type DashboardTeam struct {
	DashboardMetrics
	TeamID int64  `json:"team_id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}
type DashboardMember struct {
	DashboardMetrics
	TeamID   int64  `json:"team_id"`
	TeamName string `json:"team_name"`
	UserID   int64  `json:"user_id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Status   string `json:"status"`
}
type DashboardMemberPage struct {
	Items    []DashboardMember `json:"items"`
	Total    int64             `json:"total"`
	Page     int               `json:"page"`
	PageSize int               `json:"page_size"`
}
type DashboardScope struct {
	Kind             string `json:"kind"`
	ID               int64  `json:"id"`
	Name             string `json:"name"`
	Status           string `json:"status"`
	EnterpriseStatus string `json:"enterprise_status,omitempty"`
	CanViewMembers   bool   `json:"can_view_members"`
	EnterpriseID     int64  `json:"-"`
	TeamID           int64  `json:"-"`
}
type OrganizationDashboard struct {
	Scope     DashboardScope     `json:"scope"`
	Resources DashboardResources `json:"resources"`
	Usage     DashboardMetrics   `json:"usage"`
	Trend     []DashboardPoint   `json:"trend"`
	Models    []DashboardModel   `json:"models"`
	Teams     []DashboardTeam    `json:"teams"`
	Members   []DashboardMember  `json:"members"`
}
type DashboardMemberDetail struct {
	Member DashboardMember  `json:"member"`
	Trend  []DashboardPoint `json:"trend"`
	Models []DashboardModel `json:"models"`
}
type DashboardFilter struct {
	Filter
	Granularity string
	Timezone    string
}

func dashboardDecode[T any](v any, err error) (out T, e error) {
	if err != nil {
		return out, err
	}
	b, e := json.Marshal(v)
	if e != nil {
		return out, e
	}
	e = json.Unmarshal(b, &out)
	return
}
func dashboardFilter(c *gin.Context) (DashboardFilter, error) {
	f := DashboardFilter{Granularity: c.DefaultQuery("granularity", "day"), Timezone: c.DefaultQuery("timezone", "UTC")}
	if f.Granularity != "day" && f.Granularity != "hour" {
		return f, invalid("无效时间粒度")
	}
	if f.Timezone == "Local" {
		return f, invalid("无效时区")
	}
	loc, e := time.LoadLocation(f.Timezone)
	if e != nil {
		return f, invalid("无效时区")
	}
	now := time.Now().In(loc)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	f.From = today.AddDate(0, 0, -6)
	f.To = today.AddDate(0, 0, 1)
	for _, v := range []struct {
		key    string
		target *time.Time
		end    bool
	}{{"start_date", &f.From, false}, {"end_date", &f.To, true}} {
		if value := c.Query(v.key); value != "" {
			parsed, err := time.ParseInLocation("2006-01-02", value, loc)
			if err != nil {
				return f, invalid("日期格式须为 YYYY-MM-DD")
			}
			if v.end {
				parsed = parsed.AddDate(0, 0, 1)
			}
			*v.target = parsed
		}
	}
	if !f.From.Before(f.To) || f.To.Sub(f.From) > 366*24*time.Hour {
		return f, invalid("请选择不超过一年的有效日期范围")
	}
	if f.Granularity == "hour" && f.To.Sub(f.From) > 31*24*time.Hour {
		return f, invalid("小时粒度最多查询 31 天")
	}
	for _, v := range []struct {
		key    string
		target *int64
	}{{"team_id", &f.TeamID}} {
		if raw := c.Query(v.key); raw != "" {
			n, e := strconv.ParseInt(raw, 10, 64)
			if e != nil || n <= 0 {
				return f, invalid("无效团队编号")
			}
			*v.target = n
		}
	}
	for _, key := range []string{"user_id", "member_id", "api_key_id", "enterprise_id"} {
		if c.Query(key) != "" {
			return f, invalid("此接口不接受该范围参数")
		}
	}
	f.Page = 1
	f.Size = 20
	for _, v := range []struct {
		key    string
		target *int
	}{{"page", &f.Page}, {"page_size", &f.Size}} {
		if raw := c.Query(v.key); raw != "" {
			n, e := strconv.Atoi(raw)
			if e != nil || n < 1 || (v.key == "page_size" && n > 100) || n > 100000 {
				return f, invalid("无效分页参数")
			}
			*v.target = n
		}
	}
	return f, nil
}
func (s *Service) dashboardScope(ctx context.Context, actor int64, enterprise bool, teamID int64) (DashboardScope, error) {
	var out DashboardScope
	if !s.Enabled(ctx) {
		return out, forbidden()
	}
	if enterprise {
		entity, e := s.enterpriseScope(ctx, s.db, actor, 0, false)
		if e != nil {
			return out, e
		}
		out = DashboardScope{Kind: "enterprise", ID: number(entity, "id"), Name: str(entity, "name"), Status: str(entity, "status"), EnterpriseID: number(entity, "id"), CanViewMembers: entity["allow_member_usage_view"] == true}
		if teamID > 0 {
			if _, e = one(ctx, s.db, "SELECT id FROM teams WHERE id=$1 AND enterprise_id=$2", teamID, out.ID); e != nil {
				return out, notFound()
			}
			out.TeamID = teamID
		}
		return out, nil
	}
	row, e := one(ctx, s.db, `SELECT m.team_id FROM users u JOIN team_memberships m ON m.user_id=u.id AND m.left_at IS NULL AND m.status='active' WHERE u.id=$1 AND u.role=$2 AND u.status='active' AND u.deleted_at IS NULL`, actor, service.RoleTeamAdmin)
	if e != nil {
		return out, forbidden()
	}
	tid := number(row, "team_id")
	if teamID > 0 && teamID != tid {
		return out, forbidden()
	}
	entity, e := s.scope(ctx, s.db, actor, tid, false)
	if e != nil || entity["can_view_team"] != true {
		return out, forbidden()
	}
	return DashboardScope{Kind: "team", ID: tid, TeamID: tid, Name: str(entity, "name"), Status: str(entity, "status"), EnterpriseStatus: str(entity, "enterprise_status"), CanViewMembers: true}, nil
}

// One attributed usage relation feeds every organization metric. No joins to
// current membership: former members and repeated joins cannot duplicate rows.
const dashboardCTE = `WITH scoped_teams AS (SELECT id,name,status,balance FROM teams WHERE ($1::bigint=0 OR enterprise_id=$1) AND ($2::bigint=0 OR id=$2)), scoped_usage AS (
 SELECT l.* FROM usage_logs l JOIN scoped_teams t ON t.id=l.organization_team_id WHERE l.created_at>=$3 AND l.created_at<$4 AND ($5::bigint=0 OR l.user_id=$5)) `
const dashboardMetricSQL = `count(l.id) AS requests,COALESCE(sum(l.input_tokens),0) AS input_tokens,COALESCE(sum(l.output_tokens),0) AS output_tokens,
 COALESCE(sum(l.cache_creation_tokens),0) AS cache_creation_tokens,COALESCE(sum(l.cache_read_tokens),0) AS cache_read_tokens,
 COALESCE(sum(l.input_tokens+l.output_tokens+l.cache_creation_tokens+l.cache_read_tokens),0) AS tokens,COALESCE(sum(l.actual_cost),0) AS cost,COALESCE(avg(l.duration_ms),0) AS average_duration_ms`
const dashboardMemberSQL = `SELECT m.team_id,t.name AS team_name,m.user_id,u.username,u.email,m.status,` + dashboardMetricSQL + `
 FROM (SELECT DISTINCT ON (team_id,user_id) team_id,user_id,status FROM team_memberships WHERE team_id IN (SELECT id FROM scoped_teams) ORDER BY team_id,user_id,joined_at DESC,id DESC) m
 JOIN scoped_teams t ON t.id=m.team_id JOIN users u ON u.id=m.user_id
 LEFT JOIN scoped_usage l ON l.organization_team_id=m.team_id AND l.user_id=m.user_id
 WHERE ($5::bigint=0 OR m.user_id=$5)
 GROUP BY m.team_id,t.name,m.user_id,u.username,u.email,m.status ORDER BY cost DESC,m.team_id,m.user_id`

func dashboardArgs(scope DashboardScope, f DashboardFilter, uid int64) []any {
	return []any{scope.EnterpriseID, scope.TeamID, f.From, f.To, uid}
}
func (s *Service) dashboardCharts(ctx context.Context, scope DashboardScope, f DashboardFilter, uid int64) ([]DashboardPoint, []DashboardModel, error) {
	args := dashboardArgs(scope, f, uid)
	trend, e := dashboardDecode[[]DashboardPoint](rows(ctx, s.db, dashboardCTE+`SELECT to_char(date_trunc($6, l.created_at AT TIME ZONE $7),CASE WHEN $6='hour' THEN 'YYYY-MM-DD HH24:00' ELSE 'YYYY-MM-DD' END) AS date,`+dashboardMetricSQL+` FROM scoped_usage l GROUP BY 1 ORDER BY 1`, append(args, f.Granularity, f.Timezone)...))
	if e != nil {
		return nil, nil, e
	}
	models, e := dashboardDecode[[]DashboardModel](rows(ctx, s.db, dashboardCTE+`SELECT COALESCE(NULLIF(l.requested_model,''),l.model) AS model,`+dashboardMetricSQL+` FROM scoped_usage l GROUP BY 1 ORDER BY cost DESC,model`, args...))
	return trend, models, e
}
func (s *Service) Dashboard(ctx context.Context, actor int64, enterprise bool, f DashboardFilter) (*OrganizationDashboard, error) {
	scope, e := s.dashboardScope(ctx, actor, enterprise, f.TeamID)
	if e != nil {
		return nil, e
	}
	out := &OrganizationDashboard{Scope: scope, Teams: []DashboardTeam{}, Members: []DashboardMember{}}
	args := dashboardArgs(scope, f, 0)
	out.Resources, e = dashboardDecode[DashboardResources](one(ctx, s.db, dashboardCTE+`SELECT
 (SELECT count(*) FROM scoped_teams) AS team_count,
 (SELECT count(*) FROM team_memberships WHERE team_id IN (SELECT id FROM scoped_teams) AND left_at IS NULL) AS member_count,
 (SELECT count(*) FROM api_keys WHERE team_id IN (SELECT id FROM scoped_teams) AND deleted_at IS NULL) AS team_key_count,
 (SELECT COALESCE(sum(balance),0) FROM scoped_teams) AS balance,
 (SELECT COALESCE(sum(remaining),0) FROM team_subscription_grants WHERE team_id IN (SELECT id FROM scoped_teams)) AS remaining_subscriptions,
 (SELECT COALESCE(sum(amount),0) FROM team_events WHERE team_id IN (SELECT id FROM scoped_teams) AND kind='balance.allocate') AS allocated_balance`, args...))
	if e != nil {
		return nil, e
	}
	out.Usage, e = dashboardDecode[DashboardMetrics](one(ctx, s.db, dashboardCTE+`SELECT `+dashboardMetricSQL+` FROM scoped_usage l`, args...))
	if e != nil {
		return nil, e
	}
	out.Trend, out.Models, e = s.dashboardCharts(ctx, scope, f, 0)
	if e != nil {
		return nil, e
	}
	if enterprise {
		out.Teams, e = dashboardDecode[[]DashboardTeam](rows(ctx, s.db, dashboardCTE+`SELECT t.id AS team_id,t.name,t.status,`+dashboardMetricSQL+` FROM scoped_teams t LEFT JOIN scoped_usage l ON l.organization_team_id=t.id GROUP BY t.id,t.name,t.status ORDER BY cost DESC,t.id LIMIT 12`, args...))
		if e != nil {
			return nil, e
		}
	}
	if scope.CanViewMembers {
		out.Members, e = dashboardDecode[[]DashboardMember](rows(ctx, s.db, dashboardCTE+dashboardMemberSQL+` LIMIT 12`, args...))
		if e != nil {
			return nil, e
		}
	}
	return out, nil
}
func (s *Service) DashboardMembers(ctx context.Context, actor int64, enterprise bool, f DashboardFilter) (*DashboardMemberPage, error) {
	scope, e := s.dashboardScope(ctx, actor, enterprise, f.TeamID)
	if e != nil {
		return nil, e
	}
	if !scope.CanViewMembers {
		return nil, forbidden()
	}
	return dashboardDecode[*DashboardMemberPage](page(ctx, s.db, dashboardCTE+dashboardMemberSQL, dashboardArgs(scope, f, 0), f.Filter))
}
func (s *Service) DashboardMember(ctx context.Context, actor int64, enterprise bool, f DashboardFilter, uid int64) (*DashboardMemberDetail, error) {
	scope, e := s.dashboardScope(ctx, actor, enterprise, f.TeamID)
	if e != nil {
		return nil, e
	}
	if !scope.CanViewMembers {
		return nil, forbidden()
	}
	if uid <= 0 || scope.TeamID == 0 {
		return nil, invalid("请选择成员所属团队")
	}
	member, e := dashboardDecode[DashboardMember](one(ctx, s.db, dashboardCTE+dashboardMemberSQL, dashboardArgs(scope, f, uid)...))
	if e != nil {
		return nil, e
	}
	trend, models, e := s.dashboardCharts(ctx, scope, f, uid)
	if e != nil {
		return nil, e
	}
	return &DashboardMemberDetail{member, trend, models}, nil
}
func (s *Service) registerDashboard(group *gin.RouterGroup, path string, enterprise bool) {
	group.GET(path, func(c *gin.Context) {
		f, e := dashboardFilter(c)
		if e != nil {
			respond(c, nil, e)
			return
		}
		out, e := s.Dashboard(c.Request.Context(), actor(c), enterprise, f)
		respond(c, out, e)
	})
	group.GET(path+"/members", func(c *gin.Context) {
		f, e := dashboardFilter(c)
		if e != nil {
			respond(c, nil, e)
			return
		}
		out, e := s.DashboardMembers(c.Request.Context(), actor(c), enterprise, f)
		respond(c, out, e)
	})
	group.GET(path+"/members/:userID", func(c *gin.Context) {
		f, e := dashboardFilter(c)
		if e != nil {
			respond(c, nil, e)
			return
		}
		out, e := s.DashboardMember(c.Request.Context(), actor(c), enterprise, f, param(c, "userID"))
		respond(c, out, e)
	})
}

// Include the scoped CTE for shared management-page aggregates without changing
// ordinary users' team-key-only history endpoints.
func (s *Service) organizationUsageTotal(ctx context.Context, teamID int64) (map[string]any, error) {
	return one(ctx, s.db, fmt.Sprintf(`SELECT %s FROM usage_logs l WHERE l.organization_team_id=$1`, dashboardMetricSQL), teamID)
}

// Snapshot membership without applying organization status to personal usage.
// The partial unique index on current membership makes this a single-row lookup.
func (s *Service) UsageMembership(ctx context.Context, userID int64) (*int64, error) {
	var id int64
	err := s.db.QueryRowContext(ctx, "SELECT COALESCE((SELECT id FROM team_memberships WHERE user_id=$1 AND left_at IS NULL),0)", userID).Scan(&id)
	return &id, err
}
