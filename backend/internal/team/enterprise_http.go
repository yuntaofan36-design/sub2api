package team

import (
	"context"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func (s *Service) GovernTeamDetail(ctx context.Context, actor, tid int64) (any, error) {
	ent, t, e := s.governedTeam(ctx, s.db, actor, 0, tid, false)
	if e != nil {
		return nil, e
	}
	t["can_govern"] = ent["can_govern"]
	t["enterprise_status"] = ent["status"]
	stats, e := s.EnterpriseStatistics(ctx, actor, 0, Filter{TeamID: tid})
	if e != nil {
		return nil, e
	}
	t["stats"] = stats
	groups, e := rows(ctx, s.db, `SELECT g.id,g.name,g.status,g.subscription_type FROM team_allowed_groups a JOIN groups g ON g.id=a.group_id WHERE a.team_id=$1 AND g.deleted_at IS NULL ORDER BY g.id`, tid)
	if e != nil {
		return nil, e
	}
	t["groups"] = groups
	return t, nil
}
func (s *Service) registerEnterprises(v1 *gin.RouterGroup, jwt middleware.JWTAuthMiddleware, audit middleware.AuditLogMiddleware, stepUp middleware.StepUpAuthMiddleware, limiter *middleware.PanelRateLimiter, enabled gin.HandlerFunc) {
	platform := v1.Group("/admin/enterprises", gin.HandlerFunc(jwt), middleware.AdminOnly(), middleware.AdminComplianceGuard(s.settings), enabled, limiter.Global(), gin.HandlerFunc(audit))
	platform.GET("", func(c *gin.Context) {
		f, e := filter(c)
		if e != nil {
			respond(c, nil, e)
			return
		}
		out, e := s.ListEnterprises(c.Request.Context(), f)
		respond(c, out, e)
	})
	platform.GET("/leader-candidates", func(c *gin.Context) {
		f, e := filter(c)
		if e != nil {
			respond(c, nil, e)
			return
		}
		out, e := page(c.Request.Context(), s.db, `SELECT u.id,u.email,u.username FROM users u WHERE u.role='user' AND u.status='active' AND u.deleted_at IS NULL AND NOT EXISTS(SELECT 1 FROM team_memberships m WHERE m.user_id=u.id AND m.left_at IS NULL) AND ($1='' OR u.email ILIKE '%'||$1||'%' OR u.username ILIKE '%'||$1||'%') ORDER BY u.id`, []any{strings.TrimSpace(c.Query("search"))}, f)
		respond(c, out, e)
	})
	platform.POST("", gin.HandlerFunc(stepUp), func(c *gin.Context) {
		var in EnterpriseInput
		if !bind(c, &in) {
			return
		}
		out, e := s.SaveEnterprise(c.Request.Context(), actor(c), 0, in)
		respond(c, out, e)
	})
	platform.GET("/:enterpriseID", func(c *gin.Context) {
		out, e := s.Enterprise(c.Request.Context(), actor(c), param(c, "enterpriseID"))
		respond(c, out, e)
	})
	platform.PUT("/:enterpriseID", gin.HandlerFunc(stepUp), func(c *gin.Context) {
		var in EnterpriseInput
		if !bind(c, &in) {
			return
		}
		out, e := s.SaveEnterprise(c.Request.Context(), actor(c), param(c, "enterpriseID"), in)
		respond(c, out, e)
	})
	platform.PUT("/:enterpriseID/leaders/:userID", gin.HandlerFunc(stepUp), func(c *gin.Context) {
		var in struct {
			Promote bool `json:"promote"`
		}
		if !bind(c, &in) {
			return
		}
		out, e := s.AppointLeader(c.Request.Context(), actor(c), param(c, "enterpriseID"), param(c, "userID"), in.Promote)
		respond(c, out, e)
	})
	platform.PUT("/:enterpriseID/teams/:teamID", gin.HandlerFunc(stepUp), func(c *gin.Context) {
		out, e := s.AttachTeam(c.Request.Context(), actor(c), param(c, "enterpriseID"), param(c, "teamID"))
		respond(c, out, e)
	})
	for _, kind := range []string{"teams", "administrators", "stats", "events"} {
		platform.GET("/:enterpriseID/"+kind, func(c *gin.Context) { s.enterpriseRead(c, param(c, "enterpriseID"), kind) })
	}
	owners := v1.Group("/enterprise", gin.HandlerFunc(jwt), enabled, limiter.Global(), gin.HandlerFunc(audit), func(c *gin.Context) {
		u, e := one(c.Request.Context(), s.db, `SELECT u.role,l.enterprise_id FROM users u JOIN enterprise_leaders l ON l.user_id=u.id AND l.left_at IS NULL WHERE u.id=$1 AND u.status='active' AND u.deleted_at IS NULL`, actor(c))
		if e != nil || str(u, "role") != service.RoleEnterpriseAdmin {
			respond(c, nil, forbidden())
			c.Abort()
			return
		}
		middleware.SetAuditExtra(c, map[string]any{"enterprise_id": number(u, "enterprise_id"), "team_id": param(c, "teamID"), "target_user_id": param(c, "userID")})
		c.Next()
	})
	s.registerDashboard(owners, "/dashboard", true)
	owners.GET("/current", func(c *gin.Context) { out, e := s.Enterprise(c.Request.Context(), actor(c), 0); respond(c, out, e) })
	for _, kind := range []string{"teams", "administrators", "stats", "events"} {
		owners.GET("/"+kind, func(c *gin.Context) { s.enterpriseRead(c, 0, kind) })
	}
	owners.GET("/teams/:teamID", func(c *gin.Context) {
		out, e := s.GovernTeamDetail(c.Request.Context(), actor(c), param(c, "teamID"))
		respond(c, out, e)
	})
	owners.GET("/teams/:teamID/administrator-candidates", func(c *gin.Context) {
		f, e := filter(c)
		if e != nil {
			respond(c, nil, e)
			return
		}
		out, e := s.AdministratorCandidates(c.Request.Context(), actor(c), 0, param(c, "teamID"), f)
		respond(c, out, e)
	})
	owners.POST("/teams", gin.HandlerFunc(stepUp), func(c *gin.Context) {
		var in GovernTeamInput
		if !bind(c, &in) {
			return
		}
		out, e := s.GovernSaveTeam(c.Request.Context(), actor(c), 0, in)
		respond(c, out, e)
	})
	owners.PATCH("/teams/:teamID", gin.HandlerFunc(stepUp), func(c *gin.Context) {
		var in GovernTeamInput
		if !bind(c, &in) {
			return
		}
		out, e := s.GovernSaveTeam(c.Request.Context(), actor(c), param(c, "teamID"), in)
		respond(c, out, e)
	})
	owners.POST("/teams/:teamID/administrators", gin.HandlerFunc(stepUp), func(c *gin.Context) {
		var in MemberInput
		if !bind(c, &in) {
			return
		}
		out, e := s.GovernCreateAdministrator(c.Request.Context(), actor(c), param(c, "teamID"), in)
		respond(c, out, e)
	})
	owners.PUT("/teams/:teamID/administrators/:userID", gin.HandlerFunc(stepUp), func(c *gin.Context) {
		var in struct {
			Promote bool `json:"promote"`
		}
		if !bind(c, &in) {
			return
		}
		out, e := s.GovernAppoint(c.Request.Context(), actor(c), param(c, "teamID"), param(c, "userID"), in.Promote)
		respond(c, out, e)
	})
}
func (s *Service) enterpriseRead(c *gin.Context, id int64, kind string) {
	f, e := filter(c)
	if e != nil {
		respond(c, nil, e)
		return
	}
	var out any
	switch kind {
	case "teams":
		out, e = s.EnterpriseTeams(c.Request.Context(), actor(c), id, f)
	case "administrators":
		out, e = s.EnterpriseAdministrators(c.Request.Context(), actor(c), id, f)
	case "stats":
		out, e = s.EnterpriseStatistics(c.Request.Context(), actor(c), id, f)
	case "events":
		out, e = s.EnterpriseEvents(c.Request.Context(), actor(c), id, f)
	}
	respond(c, out, e)
}
