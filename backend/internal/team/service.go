// Package team implements team-scoped management without granting platform administration.
package team

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/mail"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
	"github.com/shopspring/decimal"
)

type queryer interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}
type Error struct {
	Status  int
	Message string
}

func (e *Error) Error() string { return e.Message }
func invalid(s string) error   { return &Error{400, s} }
func forbidden() error         { return &Error{403, "没有此团队的操作权限"} }
func notFound() error          { return &Error{404, "记录不存在或不属于当前团队"} }
func missing(err error) bool {
	var e *Error
	return errors.As(err, &e) && e.Status == 404
}

type Service struct {
	db            *sql.DB
	ent           *dbent.Client
	keys          *service.APIKeyService
	users         service.UserRepository
	subscriptions *service.SubscriptionService
	billing       service.BillingCache
	cfg           *config.Config
	settings      *service.SettingService
}

func New(db *sql.DB, ent *dbent.Client, keys *service.APIKeyService, users service.UserRepository, subscriptions *service.SubscriptionService, billing service.BillingCache, cfg *config.Config, settings *service.SettingService) *Service {
	return &Service{db, ent, keys, users, subscriptions, billing, cfg, settings}
}
func (s *Service) Enabled(ctx context.Context) bool {
	return s.cfg.RunMode != config.RunModeSimple && !s.settings.IsBackendModeEnabled(ctx)
}
func rows(ctx context.Context, q queryer, statement string, args ...any) ([]map[string]any, error) {
	query := "SELECT row_to_json(team_row) FROM (" + statement + ") team_row"
	if strings.HasPrefix(statement, "INSERT") || strings.HasPrefix(statement, "UPDATE") {
		query = "WITH team_row AS (" + statement + ") SELECT row_to_json(team_row) FROM team_row"
	}
	r, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	result := make([]map[string]any, 0)
	for r.Next() {
		var raw []byte
		if err = r.Scan(&raw); err != nil {
			return nil, err
		}
		var item map[string]any
		if err = json.Unmarshal(raw, &item); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, r.Err()
}
func one(ctx context.Context, q queryer, statement string, args ...any) (map[string]any, error) {
	r, e := rows(ctx, q, statement, args...)
	if e != nil {
		return nil, e
	}
	if len(r) == 0 {
		return nil, notFound()
	}
	return r[0], nil
}
func number(m map[string]any, k string) int64 { v, _ := m[k].(float64); return int64(v) }
func str(m map[string]any, k string) string   { v, _ := m[k].(string); return v }
func token() (string, error) {
	b := make([]byte, 32)
	_, e := rand.Read(b)
	return hex.EncodeToString(b), e
}
func hash(v string) string { h := sha256.Sum256([]byte(v)); return hex.EncodeToString(h[:]) }
func email(v string) (string, error) {
	v = strings.ToLower(strings.TrimSpace(v))
	a, e := mail.ParseAddress(v)
	if e != nil || a.Address != v || len(v) > 255 {
		return "", invalid("请输入有效邮箱")
	}
	return v, nil
}
func amount(v string, positive bool) (string, error) {
	d, e := decimal.NewFromString(v)
	if e != nil || d.IsZero() || (positive && d.IsNegative()) || d.Exponent() < -8 || d.Abs().GreaterThanOrEqual(decimal.New(1, 12)) {
		return "", invalid("金额必须有效，最多 8 位小数且不能为零")
	}
	return d.StringFixed(8), nil
}
func changed(r sql.Result, e error) error {
	if e != nil {
		return e
	}
	n, e := r.RowsAffected()
	if e != nil {
		return e
	}
	if n == 0 {
		return notFound()
	}
	return nil
}
func (s *Service) scope(ctx context.Context, q queryer, actor, teamID int64, manage bool) (map[string]any, error) {
	if !s.Enabled(ctx) {
		return nil, &Error{403, "当前运行模式不开放团队功能"}
	}
	u, e := one(ctx, q, "SELECT id,role FROM users WHERE id=$1 AND status='active' AND deleted_at IS NULL", actor)
	if e != nil {
		return nil, e
	}
	t, e := one(ctx, q, `SELECT t.id,t.name,t.status,t.balance,t.enterprise_id,e.status AS enterprise_status FROM teams t LEFT JOIN enterprises e ON e.id=t.enterprise_id WHERE t.id=$1`, teamID)
	if e != nil {
		return nil, e
	}
	if str(u, "role") == service.RoleAdmin {
		t["can_manage"] = true
		t["platform_admin"] = true
		t["can_view_team"] = true
		return t, nil
	}
	m, e := one(ctx, q, "SELECT id,status FROM team_memberships WHERE team_id=$1 AND user_id=$2 AND left_at IS NULL", teamID, actor)
	if e != nil {
		return nil, forbidden()
	}
	canView := str(u, "role") == service.RoleTeamAdmin && str(m, "status") == "active"
	canManage := canView && str(t, "status") == "active" && str(t, "enterprise_status") != "disabled"
	t["can_view_team"] = canView
	if manage && !canManage {
		return nil, forbidden()
	}
	t["can_manage"] = canManage
	t["platform_admin"] = false
	t["membership_id"] = m["id"]
	t["member_status"] = m["status"]
	return t, nil
}
func (s *Service) transaction(ctx context.Context, actor, teamID int64, manage bool, fn func(context.Context, queryer, map[string]any) (any, error)) (any, error) {
	tx, e := s.ent.Tx(ctx)
	if e != nil {
		return nil, e
	}
	defer tx.Rollback()
	tc := dbent.NewTxContext(ctx, tx)
	q := tx.Client()
	if _, e = q.ExecContext(tc, "SELECT pg_advisory_xact_lock(824001)"); e != nil {
		return nil, e
	}
	var scope map[string]any
	if teamID > 0 {
		scope, e = s.scope(tc, q, actor, teamID, manage)
		if e != nil {
			return nil, e
		}
		if str(scope, "enterprise_status") == "disabled" && scope["platform_admin"] != true {
			return nil, invalid("企业已停用，不能执行团队管理操作")
		}
		if _, e = q.ExecContext(tc, "SELECT id FROM teams WHERE id=$1 FOR UPDATE", teamID); e != nil {
			return nil, e
		}
	}
	out, e := fn(tc, q, scope)
	if e != nil {
		return nil, e
	}
	if e = tx.Commit(); e != nil {
		return nil, e
	}
	return out, nil
}
func record(ctx context.Context, q queryer, teamID, actor, memberID int64, kind string, details any) error {
	data, e := json.Marshal(details)
	if e != nil {
		return e
	}
	var member any
	if memberID > 0 {
		member = memberID
	}
	_, e = q.ExecContext(ctx, "INSERT INTO team_events(team_id,actor_id,member_id,kind,details) VALUES($1,$2,$3,$4,$5)", teamID, actor, member, kind, string(data))
	return e
}
func (s *Service) invalidate(ctx context.Context, userID int64) {
	s.keys.InvalidateAuthCacheByUserID(ctx, userID)
	if s.billing != nil {
		_ = s.billing.InvalidateUserBalance(ctx, userID)
	}
}
func (s *Service) invalidateTeam(ctx context.Context, teamID int64) {
	members, err := rows(ctx, s.db, "SELECT user_id FROM team_memberships WHERE team_id=$1 AND left_at IS NULL", teamID)
	if err != nil {
		// Gateway authorization always reloads team state even if invalidation fails.
		slog.Error("team cache invalidation failed", "team_id", teamID, "error", err)
		return
	}
	for _, member := range members {
		s.invalidate(ctx, number(member, "user_id"))
	}
}
func (s *Service) Current(ctx context.Context, userID int64) (any, error) {
	if !s.Enabled(ctx) {
		return nil, nil
	}
	r, e := rows(ctx, s.db, `SELECT t.id,t.name,t.status,m.id AS membership_id,m.status AS member_status,u.role,t.enterprise_id,e.status AS enterprise_status
 FROM team_memberships m JOIN teams t ON t.id=m.team_id JOIN users u ON u.id=m.user_id LEFT JOIN enterprises e ON e.id=t.enterprise_id
 WHERE m.user_id=$1 AND m.left_at IS NULL`, userID)
	if e != nil {
		return nil, e
	}
	if len(r) == 0 {
		return nil, nil
	}
	return r[0], nil
}
func (s *Service) ListTeams(ctx context.Context, f Filter) (any, error) {
	args := []any{f.EnterpriseID}
	search := searchClause(f.Search, &args, "t.name", "e.name", `(SELECT string_agg(concat_ws(' ',u.email,u.username),' ') FROM team_memberships m JOIN users u ON u.id=m.user_id WHERE m.team_id=t.id AND m.left_at IS NULL AND u.role='team_admin' AND u.deleted_at IS NULL)`)
	return page(ctx, s.db, `SELECT t.id,t.name,t.status,t.balance,t.created_at,t.updated_at,t.enterprise_id,e.name AS enterprise_name,e.status AS enterprise_status,
 (SELECT count(*) FROM team_memberships m WHERE m.team_id=t.id AND m.left_at IS NULL) AS member_count,
 COALESCE((SELECT json_agg(json_build_object('id',u.id,'email',u.email,'username',u.username,
 'status',u.status,'member_status',m.status) ORDER BY u.id)
 FROM team_memberships m JOIN users u ON u.id=m.user_id
 WHERE m.team_id=t.id AND m.left_at IS NULL AND u.role='team_admin' AND u.deleted_at IS NULL), '[]'::json) AS administrators
 FROM teams t LEFT JOIN enterprises e ON e.id=t.enterprise_id WHERE ($1::bigint=0 OR t.enterprise_id=$1 OR ($1=-1 AND t.enterprise_id IS NULL))`+search+` ORDER BY t.id DESC`, args, f)
}

type TeamInput struct {
	Name     string  `json:"name"`
	Status   string  `json:"status"`
	AdminID  int64   `json:"admin_id"`
	GroupIDs []int64 `json:"group_ids"`
}

func (s *Service) SaveTeam(ctx context.Context, actor, id int64, in TeamInput) (any, error) {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" || len([]rune(in.Name)) > 100 {
		return nil, invalid("团队名称须为 1–100 个字符")
	}
	if in.Status == "" {
		in.Status = "active"
	}
	if in.Status != "active" && in.Status != "disabled" {
		return nil, invalid("无效团队状态")
	}
	out, err := s.transaction(ctx, actor, id, true, func(tc context.Context, q queryer, scope map[string]any) (any, error) {
		var before map[string]any
		if id == 0 {
			if _, err := one(tc, q, "SELECT id FROM users WHERE id=$1 AND role='admin' AND status='active' AND deleted_at IS NULL", actor); err != nil {
				return nil, forbidden()
			}
			t, e := one(tc, q, `INSERT INTO teams(name,status) VALUES($1,$2) RETURNING id`, in.Name, in.Status)
			if e != nil {
				return nil, e
			}
			id = number(t, "id")
			if e = s.appoint(tc, q, id, in.AdminID, true); e != nil {
				return nil, e
			}
		} else {
			if scope["platform_admin"] != true {
				return nil, forbidden()
			}
			before = map[string]any{"name": scope["name"], "status": scope["status"]}
			groups, err := rows(tc, q, "SELECT group_id FROM team_allowed_groups WHERE team_id=$1 ORDER BY group_id", id)
			if err != nil {
				return nil, err
			}
			ids := make([]int64, 0, len(groups))
			for _, g := range groups {
				ids = append(ids, number(g, "group_id"))
			}
			before["group_ids"] = ids
			if _, e := q.ExecContext(tc, "UPDATE teams SET name=$2,status=$3,updated_at=now() WHERE id=$1", id, in.Name, in.Status); e != nil {
				return nil, e
			}
		}
		if _, e := q.ExecContext(tc, "DELETE FROM team_allowed_groups WHERE team_id=$1", id); e != nil {
			return nil, e
		}
		for _, gid := range in.GroupIDs {
			r, e := q.ExecContext(tc, `INSERT INTO team_allowed_groups(team_id,group_id) SELECT $1,id FROM groups WHERE id=$2 AND deleted_at IS NULL AND status='active' ON CONFLICT DO NOTHING`, id, gid)
			if e != nil {
				return nil, e
			}
			if n, _ := r.RowsAffected(); n == 0 {
				return nil, invalid("分组无效或重复")
			}
		}
		if e := record(tc, q, id, actor, 0, "team.save", map[string]any{"before": before, "after": in}); e != nil {
			return nil, e
		}
		return map[string]any{"id": id}, nil
	})
	if err == nil {
		s.invalidateTeam(ctx, id)
	}
	return out, err
}
func (s *Service) appoint(ctx context.Context, q queryer, teamID, userID int64, promote bool) error {
	u, e := one(ctx, q, "SELECT id,role,status FROM users WHERE id=$1 AND deleted_at IS NULL FOR UPDATE", userID)
	if e != nil {
		return e
	}
	if promote && str(u, "status") != "active" {
		return invalid("只能任命有效账号为团队管理员")
	}
	if str(u, "role") != service.RoleUser && str(u, "role") != service.RoleTeamAdmin {
		return invalid("平台管理员和企业负责人不能加入团队")
	}
	m, e := rows(ctx, q, "SELECT id,team_id,status FROM team_memberships WHERE user_id=$1 AND left_at IS NULL", userID)
	if e != nil {
		return e
	}
	if len(m) > 0 && (number(m[0], "team_id") != teamID || str(m[0], "status") != "active") {
		return invalid("用户已属于其他团队或成员关系被暂停")
	}
	if len(m) == 0 {
		if !promote {
			return notFound()
		}
		if _, e = q.ExecContext(ctx, "INSERT INTO team_memberships(team_id,user_id) VALUES($1,$2)", teamID, userID); e != nil {
			return e
		}
	}
	role := service.RoleUser
	if promote {
		role = service.RoleTeamAdmin
	}
	_, e = q.ExecContext(ctx, "UPDATE users SET role=$2,updated_at=now() WHERE id=$1", userID, role)
	return e
}
func (s *Service) Appoint(ctx context.Context, actor, teamID, userID int64, promote bool) (any, error) {
	r, e := s.transaction(ctx, actor, teamID, true, func(tc context.Context, q queryer, scope map[string]any) (any, error) {
		if scope["platform_admin"] != true {
			return nil, forbidden()
		}
		before, e := one(tc, q, "SELECT role,email FROM users WHERE id=$1", userID)
		if e != nil {
			return nil, e
		}
		if e := s.appoint(tc, q, teamID, userID, promote); e != nil {
			return nil, e
		}
		role := service.RoleUser
		if promote {
			role = service.RoleTeamAdmin
		}
		return true, record(tc, q, teamID, actor, 0, "administrator.change", map[string]any{"user_id": userID, "email": before["email"], "before": before["role"], "after": role})
	})
	if e == nil {
		s.invalidate(ctx, userID)
	}
	return r, e
}
func (s *Service) CheckKey(ctx context.Context, k *service.APIKey) error {
	if k.TeamID == nil {
		return nil
	}
	// Team keys are few relative to personal gateway traffic. Refresh their
	// mutable policy before checking live membership so stale snapshots cannot
	// preserve an old group, quota, or IP policy after an administrator edits it.
	fresh, err := s.keys.GetByID(ctx, k.ID)
	if err != nil {
		return err
	}
	*k = *fresh
	if !s.Enabled(ctx) {
		return service.ErrInsufficientPerms
	}
	r, e := one(ctx, s.db, `SELECT k.group_id,k.status,k.quota,k.quota_used,k.expires_at,k.rate_limit_5h,k.rate_limit_1d,k.rate_limit_7d,u.status AS user_status,u.balance
 FROM api_keys k JOIN team_memberships m ON m.id=k.team_membership_id AND m.team_id=k.team_id AND m.user_id=k.user_id
 JOIN teams t ON t.id=k.team_id LEFT JOIN enterprises e ON e.id=t.enterprise_id JOIN users u ON u.id=k.user_id
 JOIN team_allowed_groups ag ON ag.team_id=k.team_id AND ag.group_id=k.group_id
 JOIN groups g ON g.id=k.group_id
 WHERE k.id=$1 AND k.deleted_at IS NULL AND m.status='active' AND m.left_at IS NULL AND t.status='active'
 AND (t.enterprise_id IS NULL OR e.status='active')
 AND u.status='active' AND u.deleted_at IS NULL AND g.status='active' AND g.deleted_at IS NULL
 AND ( (g.subscription_type='subscription' AND EXISTS (SELECT 1 FROM user_subscriptions us WHERE us.user_id=u.id AND us.group_id=g.id AND us.status='active' AND us.expires_at>now() AND us.deleted_at IS NULL))
 OR (g.subscription_type<>'subscription' AND ((NOT g.is_exclusive AND NOT u.restrict_public_groups) OR EXISTS(SELECT 1 FROM user_allowed_groups ug WHERE ug.user_id=u.id AND ug.group_id=g.id))))`, k.ID)
	if e != nil {
		if _, ok := e.(*Error); ok {
			return service.ErrInsufficientPerms
		}
		return e
	}
	if str(r, "status") != "active" && str(r, "status") != "quota_exhausted" && str(r, "status") != "expired" {
		return service.ErrInsufficientPerms
	}
	if k.GroupID == nil || *k.GroupID != number(r, "group_id") {
		return service.ErrInsufficientPerms
	}
	k.Status = str(r, "status")
	k.Quota, _ = r["quota"].(float64)
	k.QuotaUsed, _ = r["quota_used"].(float64)
	k.RateLimit5h, _ = r["rate_limit_5h"].(float64)
	k.RateLimit1d, _ = r["rate_limit_1d"].(float64)
	k.RateLimit7d, _ = r["rate_limit_7d"].(float64)
	if v := str(r, "expires_at"); v != "" {
		t, e := time.Parse(time.RFC3339Nano, v)
		if e != nil {
			return e
		}
		k.ExpiresAt = &t
	} else {
		k.ExpiresAt = nil
	}
	return nil
}
func friendly(err error) error {
	var p *pq.Error
	if errors.As(err, &p) {
		switch p.Code {
		case "23505":
			return invalid("记录已存在，用户可能已加入团队，或请求已提交")
		case "23514":
			message := p.Message
			switch message {
			case "An active team must retain an active administrator":
				message = "正常团队必须保留至少一名有效管理员"
			case "Team administrators must have an active team membership":
				message = "团队管理员必须关联有效团队成员关系"
			case "Platform administrators cannot be team members":
				message = "平台管理员不能兼任团队成员"
			}
			return invalid(message)
		case "23503":
			return invalid("关联的用户、分组或成员不存在")
		case "40P01", "40001":
			return &Error{409, "操作发生并发冲突，请重试"}
		}
	}
	return err
}
