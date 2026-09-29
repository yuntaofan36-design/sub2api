package team

import (
	"encoding/csv"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func Register(v1 *gin.RouterGroup, s *Service, jwt middleware.JWTAuthMiddleware, audit middleware.AuditLogMiddleware, stepUp middleware.StepUpAuthMiddleware, limiter *middleware.PanelRateLimiter) {
	enabled := func(c *gin.Context) {
		for _, p := range c.Params {
			id, err := strconv.ParseInt(p.Value, 10, 64)
			if err != nil || id <= 0 {
				response.BadRequest(c, "无效记录编号")
				c.Abort()
				return
			}
		}
		if !s.Enabled(c.Request.Context()) {
			response.Forbidden(c, "当前运行模式不开放团队功能")
			c.Abort()
			return
		}
		c.Next()
	}
	s.registerEnterprises(v1, jwt, audit, stepUp, limiter, enabled)
	platform := v1.Group("/admin/teams", gin.HandlerFunc(jwt), middleware.AdminOnly(), middleware.AdminComplianceGuard(s.settings), enabled, limiter.Global(), gin.HandlerFunc(audit))
	platform.GET("", func(c *gin.Context) {
		f, e := filter(c)
		if e != nil {
			respond(c, nil, e)
			return
		}
		out, e := s.ListTeams(c.Request.Context(), f)
		respond(c, out, e)
	})
	platform.POST("", gin.HandlerFunc(stepUp), func(c *gin.Context) {
		var in TeamInput
		if !bind(c, &in) {
			return
		}
		out, e := s.SaveTeam(c.Request.Context(), actor(c), 0, in)
		respond(c, out, e)
	})
	platform.PUT("/:teamID", gin.HandlerFunc(stepUp), func(c *gin.Context) {
		var in TeamInput
		if !bind(c, &in) {
			return
		}
		out, e := s.SaveTeam(c.Request.Context(), actor(c), param(c, "teamID"), in)
		respond(c, out, e)
	})
	platform.PUT("/:teamID/administrators/:userID", gin.HandlerFunc(stepUp), func(c *gin.Context) {
		var in struct {
			Promote bool `json:"promote"`
		}
		if !bind(c, &in) {
			return
		}
		out, e := s.Appoint(c.Request.Context(), actor(c), param(c, "teamID"), param(c, "userID"), in.Promote)
		respond(c, out, e)
	})
	platform.POST("/:teamID/funding", gin.HandlerFunc(stepUp), func(c *gin.Context) {
		var in struct {
			Amount      string `json:"amount"`
			OperationID string `json:"operation_id"`
		}
		if !bind(c, &in) {
			return
		}
		out, e := s.Fund(c.Request.Context(), actor(c), param(c, "teamID"), in.Amount, in.OperationID)
		respond(c, out, e)
	})
	platform.POST("/:teamID/grants", gin.HandlerFunc(stepUp), func(c *gin.Context) {
		var in struct {
			GrantInput
			OperationID string `json:"operation_id"`
		}
		if !bind(c, &in) {
			return
		}
		out, e := s.Grant(c.Request.Context(), actor(c), param(c, "teamID"), in.GrantInput, in.OperationID)
		respond(c, out, e)
	})
	teams := v1.Group("/teams", gin.HandlerFunc(jwt), enabled, limiter.Global(), gin.HandlerFunc(audit))
	s.registerDashboard(teams, "/current/dashboard", false)
	teams.GET("/current", func(c *gin.Context) { out, e := s.Current(c.Request.Context(), actor(c)); respond(c, out, e) })
	teams.POST("/invitations/accept", func(c *gin.Context) {
		var in struct {
			Token string `json:"token"`
		}
		if !bind(c, &in) {
			return
		}
		out, e := s.AcceptInvite(c.Request.Context(), actor(c), in.Token)
		respond(c, out, e)
	})
	teams.GET("/:teamID", func(c *gin.Context) {
		out, e := s.Overview(c.Request.Context(), actor(c), param(c, "teamID"), c.Query("mine") == "true")
		respond(c, out, e)
	})
	for _, kind := range []string{"members", "keys", "grants", "invitations", "ledger", "usage"} {
		teams.GET("/:teamID/"+kind, func(c *gin.Context) {
			f, e := filter(c)
			if e != nil {
				respond(c, nil, e)
				return
			}
			out, e := s.List(c.Request.Context(), actor(c), param(c, "teamID"), kind, f)
			respond(c, out, e)
		})
	}
	teams.GET("/:teamID/usage/export", limiter.Heavy(), func(c *gin.Context) {
		f, e := filter(c)
		if e != nil {
			respond(c, nil, e)
			return
		}
		f.Size = 200
		f.Page = 1
		out, e := s.List(c.Request.Context(), actor(c), param(c, "teamID"), "usage", f)
		if e != nil {
			respond(c, nil, e)
			return
		}
		p := out.(*Page)
		if p.Total > 10000 {
			respond(c, nil, invalid("一次最多导出 10000 条，请缩小日期范围"))
			return
		}
		// Gather before sending headers, so a failed page never becomes a successful partial CSV.
		all := p.Items
		for int64(len(all)) < p.Total {
			f.Page++
			out, e = s.List(c.Request.Context(), actor(c), param(c, "teamID"), "usage", f)
			if e != nil {
				respond(c, nil, e)
				return
			}
			items := out.(*Page).Items
			if len(items) == 0 {
				break
			}
			all = append(all, items...)
		}
		c.Header("Content-Type", "text/csv; charset=utf-8")
		c.Header("Content-Disposition", "attachment; filename=team-usage.csv")
		w := csv.NewWriter(c.Writer)
		columns := []string{"id", "member_id", "api_key_id", "email", "model", "input_tokens", "output_tokens", "cache_tokens", "actual_cost", "created_at"}
		_ = w.Write(columns)
		for _, row := range all {
			line := make([]string, len(columns))
			for i, key := range columns {
				v := fmt.Sprint(row[key])
				if strings.ContainsAny(strings.TrimSpace(v)[:min(1, len(strings.TrimSpace(v)))], "=+-@") {
					v = "'" + v
				}
				line[i] = v
			}
			_ = w.Write(line)
		}
		w.Flush()
	})
	teams.POST("/:teamID/members", func(c *gin.Context) {
		var in MemberInput
		if !bind(c, &in) {
			return
		}
		out, e := s.CreateMember(c.Request.Context(), actor(c), param(c, "teamID"), in)
		respond(c, out, e)
	})
	teams.PATCH("/:teamID/members/:memberID", func(c *gin.Context) {
		var in struct {
			Status string `json:"status"`
		}
		if !bind(c, &in) {
			return
		}
		out, e := s.ChangeMember(c.Request.Context(), actor(c), param(c, "teamID"), param(c, "memberID"), in.Status, false)
		respond(c, out, e)
	})
	teams.POST("/:teamID/leave", func(c *gin.Context) {
		current, e := s.Current(c.Request.Context(), actor(c))
		if e != nil {
			respond(c, nil, e)
			return
		}
		m, ok := current.(map[string]any)
		if !ok || number(m, "id") != param(c, "teamID") {
			respond(c, nil, forbidden())
			return
		}
		out, e := s.ChangeMember(c.Request.Context(), actor(c), param(c, "teamID"), number(m, "membership_id"), "left", true)
		respond(c, out, e)
	})
	teams.POST("/:teamID/invitations", func(c *gin.Context) {
		var in struct {
			Email string `json:"email"`
		}
		if !bind(c, &in) {
			return
		}
		out, e := s.Invite(c.Request.Context(), actor(c), param(c, "teamID"), in.Email)
		respond(c, out, e)
	})
	teams.DELETE("/:teamID/invitations/:inviteID", func(c *gin.Context) {
		out, e := s.RevokeInvite(c.Request.Context(), actor(c), param(c, "teamID"), param(c, "inviteID"))
		respond(c, out, e)
	})
	teams.POST("/:teamID/allocations", gin.HandlerFunc(stepUp), func(c *gin.Context) {
		var in AllocationInput
		if !bind(c, &in) {
			return
		}
		out, e := s.Allocate(c.Request.Context(), actor(c), param(c, "teamID"), in)
		respond(c, out, e)
	})
	teams.POST("/:teamID/keys", func(c *gin.Context) {
		var in KeyInput
		if !bind(c, &in) {
			return
		}
		out, e := s.SaveKey(c.Request.Context(), actor(c), param(c, "teamID"), 0, in)
		respond(c, out, e)
	})
	teams.PUT("/:teamID/keys/:keyID", func(c *gin.Context) {
		var in KeyInput
		if !bind(c, &in) {
			return
		}
		out, e := s.SaveKey(c.Request.Context(), actor(c), param(c, "teamID"), param(c, "keyID"), in)
		respond(c, out, e)
	})
	teams.GET("/:teamID/keys/:keyID/secret", func(c *gin.Context) {
		out, e := s.Secret(c.Request.Context(), actor(c), param(c, "teamID"), param(c, "keyID"))
		respond(c, out, e)
	})
}
func actor(c *gin.Context) int64           { s, _ := middleware.GetAuthSubjectFromContext(c); return s.UserID }
func param(c *gin.Context, k string) int64 { v, _ := strconv.ParseInt(c.Param(k), 10, 64); return v }
func bind(c *gin.Context, out any) bool {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 64*1024)
	if e := c.ShouldBindJSON(out); e != nil {
		response.BadRequest(c, "请求格式不正确")
		return false
	}
	return true
}
func respond(c *gin.Context, out any, err error) {
	if err == nil {
		response.Success(c, out)
		return
	}
	err = friendly(err)
	var te *Error
	if errors.As(err, &te) {
		response.Error(c, te.Status, te.Message)
		return
	}
	if errors.Is(err, service.ErrEmailExists) {
		if strings.HasPrefix(c.FullPath(), "/api/v1/enterprise/") {
			response.BadRequest(c, "邮箱已存在，不能接管已有账号，请联系平台管理员引入")
		} else {
			response.BadRequest(c, "邮箱已存在，请邀请已有账号加入")
		}
		return
	}
	slog.Error("team operation failed", "error", err)
	response.Error(c, 500, "团队操作失败，请稍后重试")
}
func filter(c *gin.Context) (Filter, error) {
	f := Filter{Mine: c.Query("mine") == "true"}
	f.EnterpriseID, _ = strconv.ParseInt(c.Query("enterprise_id"), 10, 64)
	f.TeamID, _ = strconv.ParseInt(c.Query("team_id"), 10, 64)
	f.Page, _ = strconv.Atoi(c.Query("page"))
	f.Size, _ = strconv.Atoi(c.Query("page_size"))
	f.MemberID, _ = strconv.ParseInt(c.Query("member_id"), 10, 64)
	f.KeyID, _ = strconv.ParseInt(c.Query("key_id"), 10, 64)
	f.Model = c.Query("model")
	f.Search = strings.TrimSpace(c.Query("search"))
	for k, p := range map[string]*time.Time{"from": &f.From, "to": &f.To} {
		if v := c.Query(k); v != "" {
			t, e := time.Parse(time.RFC3339, v)
			if e != nil {
				return f, invalid("日期格式不正确")
			}
			*p = t
		}
	}
	f = f.normalized()
	if !f.From.Before(f.To) {
		return f, invalid("结束时间必须晚于开始时间")
	}
	return f, nil
}
