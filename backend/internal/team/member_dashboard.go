package team

import (
	"context"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

type MemberUsageDashboard struct {
	Usage  DashboardMetrics `json:"usage"`
	Trend  []DashboardPoint `json:"trend"`
	Models []DashboardModel `json:"models"`
}

// Membership authorizes reading; recorded Key ownership determines historical usage.
func (s *Service) MemberDashboard(ctx context.Context, actor, teamID int64, f DashboardFilter) (*MemberUsageDashboard, error) {
	scope, err := s.scope(ctx, s.db, actor, teamID, false)
	if err != nil {
		return nil, err
	}
	if number(scope, "membership_id") == 0 || f.TeamID != 0 {
		return nil, forbidden()
	}
	if f.KeyID > 0 {
		if _, err = one(ctx, s.db, `SELECT id FROM api_keys WHERE id=$1 AND team_id=$2 AND user_id=$3 AND deleted_at IS NULL`, f.KeyID, teamID, actor); err != nil {
			return nil, notFound()
		}
	}
	cte := `WITH scoped_usage AS (SELECT l.* FROM usage_logs l WHERE l.team_id=$1 AND l.user_id=$2 AND l.created_at>=$3 AND l.created_at<$4 AND ($5::bigint=0 OR l.api_key_id=$5) AND ($6::text='' OR COALESCE(NULLIF(l.requested_model,''),l.model)=$6)) `
	args := []any{teamID, actor, f.From, f.To, f.KeyID, f.Model}
	out := &MemberUsageDashboard{}
	out.Usage, err = dashboardDecode[DashboardMetrics](one(ctx, s.db, cte+`SELECT `+dashboardMetricSQL+` FROM scoped_usage l`, args...))
	if err != nil {
		return nil, err
	}
	out.Trend, err = dashboardDecode[[]DashboardPoint](rows(ctx, s.db, cte+`SELECT to_char(date_trunc($7,l.created_at AT TIME ZONE $8),CASE WHEN $7='hour' THEN 'YYYY-MM-DD HH24:00' ELSE 'YYYY-MM-DD' END) AS date,`+dashboardMetricSQL+` FROM scoped_usage l GROUP BY 1 ORDER BY 1`, append(args, f.Granularity, f.Timezone)...))
	if err != nil {
		return nil, err
	}
	out.Models, err = dashboardDecode[[]DashboardModel](rows(ctx, s.db, cte+`SELECT COALESCE(NULLIF(l.requested_model,''),l.model) AS model,`+dashboardMetricSQL+` FROM scoped_usage l GROUP BY 1 ORDER BY cost DESC,model`, args...))
	return out, err
}

func memberDashboardFilter(c *gin.Context) (DashboardFilter, error) {
	f, err := dashboardFilter(c)
	if err != nil {
		return f, err
	}
	if raw := c.Query("key_id"); raw != "" && raw != "0" {
		f.KeyID, err = strconv.ParseInt(raw, 10, 64)
		if err != nil || f.KeyID <= 0 {
			return f, invalid("无效 Key 编号")
		}
	}
	f.Model = strings.TrimSpace(c.Query("model"))
	if len(f.Model) > 200 {
		return f, invalid("模型名称过长")
	}
	return f, nil
}
