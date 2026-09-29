package team

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type EnterpriseInput struct {
	AllowMemberUsageView *bool  `json:"allow_member_usage_view"`
	Name                 string `json:"name"`
	Status               string `json:"status"`
	LeaderID             int64  `json:"leader_id"`
}
type GovernTeamInput struct {
	Name          string      `json:"name"`
	Status        string      `json:"status"`
	Administrator MemberInput `json:"administrator"`
}

func governanceName(name, status string) (string, string, error) {
	name = strings.TrimSpace(name)
	if name == "" || len([]rune(name)) > 100 {
		return "", "", invalid("名称须为 1–100 个字符")
	}
	if status == "" {
		status = "active"
	}
	if status != "active" && status != "disabled" {
		return "", "", invalid("无效状态")
	}
	return name, status, nil
}
func (s *Service) enterpriseScope(ctx context.Context, q queryer, actor, id int64, write bool) (map[string]any, error) {
	if !s.Enabled(ctx) {
		return nil, forbidden()
	}
	u, e := one(ctx, q, "SELECT role FROM users WHERE id=$1 AND status='active' AND deleted_at IS NULL", actor)
	if e != nil {
		return nil, e
	}
	var entity map[string]any
	if str(u, "role") == service.RoleAdmin && id > 0 {
		entity, e = one(ctx, q, "SELECT id,name,status,allow_member_usage_view FROM enterprises WHERE id=$1", id)
	} else if str(u, "role") == service.RoleEnterpriseAdmin {
		entity, e = one(ctx, q, `SELECT e.id,e.name,e.status,e.allow_member_usage_view FROM enterprises e JOIN enterprise_leaders l ON l.enterprise_id=e.id WHERE l.user_id=$1 AND l.left_at IS NULL AND ($2::bigint=0 OR e.id=$2)`, actor, id)
	} else {
		return nil, forbidden()
	}
	if e != nil {
		if missing(e) {
			return nil, forbidden()
		}
		return nil, e
	}
	entity["can_govern"] = str(entity, "status") == "active"
	if write && entity["can_govern"] != true {
		return nil, invalid("企业已停用，不能执行管理操作")
	}
	return entity, nil
}
func recordEnterprise(ctx context.Context, q queryer, id, actor int64, kind string, details any) error {
	b, e := json.Marshal(details)
	if e != nil {
		return e
	}
	_, e = q.ExecContext(ctx, "INSERT INTO enterprise_events(enterprise_id,actor_id,kind,details) VALUES($1,$2,$3,$4)", id, actor, kind, string(b))
	return e
}

const enterpriseLeadersSQL = `COALESCE((SELECT json_agg(json_build_object('id',u.id,'email',u.email,'username',u.username,'status',u.status) ORDER BY u.id) FROM enterprise_leaders l JOIN users u ON u.id=l.user_id WHERE l.enterprise_id=e.id AND l.left_at IS NULL AND u.deleted_at IS NULL),'[]'::json)`

func (s *Service) ListEnterprises(ctx context.Context, f Filter) (any, error) {
	args := []any{}
	search := searchClause(f.Search, &args, "e.name", `(SELECT string_agg(concat_ws(' ',u.email,u.username),' ') FROM enterprise_leaders l JOIN users u ON u.id=l.user_id WHERE l.enterprise_id=e.id AND l.left_at IS NULL AND u.deleted_at IS NULL)`)
	return page(ctx, s.db, `SELECT e.id,e.name,e.status,e.allow_member_usage_view,e.created_at,`+enterpriseLeadersSQL+` AS leaders,(SELECT count(*) FROM teams t WHERE t.enterprise_id=e.id) AS team_count FROM enterprises e WHERE true`+search+` ORDER BY e.id DESC`, args, f)
}
func (s *Service) Enterprise(ctx context.Context, actor, id int64) (any, error) {
	entity, e := s.enterpriseScope(ctx, s.db, actor, id, false)
	if e != nil {
		return nil, e
	}
	leaders, e := rows(ctx, s.db, `SELECT u.id,u.email,u.username,u.status FROM enterprise_leaders l JOIN users u ON u.id=l.user_id WHERE l.enterprise_id=$1 AND l.left_at IS NULL AND u.deleted_at IS NULL ORDER BY u.id`, number(entity, "id"))
	if e != nil {
		return nil, e
	}
	entity["leaders"] = leaders
	return entity, nil
}
func (s *Service) appointLeader(ctx context.Context, q queryer, id, uid int64, promote bool) error {
	u, e := one(ctx, q, "SELECT id,role,status FROM users WHERE id=$1 AND deleted_at IS NULL FOR UPDATE", uid)
	if e != nil {
		return e
	}
	if promote {
		if str(u, "status") != "active" {
			return invalid("只能任命有效普通用户")
		}
		if str(u, "role") == service.RoleEnterpriseAdmin {
			_, e = one(ctx, q, "SELECT id FROM enterprise_leaders WHERE enterprise_id=$1 AND user_id=$2 AND left_at IS NULL", id, uid)
			if e != nil {
				return invalid("该用户已是其他企业负责人")
			}
			return nil
		}
		if str(u, "role") != service.RoleUser {
			return invalid("只能从未加入团队的普通用户中任命企业负责人")
		}
		memberships, e := rows(ctx, q, "SELECT id FROM team_memberships WHERE user_id=$1 AND left_at IS NULL", uid)
		if e != nil {
			return e
		}
		if len(memberships) > 0 {
			return invalid("请先退出现有团队再任命企业负责人")
		}
		if _, e = q.ExecContext(ctx, "INSERT INTO enterprise_leaders(enterprise_id,user_id) VALUES($1,$2)", id, uid); e != nil {
			return e
		}
		_, e = q.ExecContext(ctx, "UPDATE users SET role='enterprise_admin',updated_at=now() WHERE id=$1", uid)
		return e
	}
	if str(u, "role") != service.RoleEnterpriseAdmin {
		return invalid("该用户不是企业负责人")
	}
	result, e := q.ExecContext(ctx, "UPDATE enterprise_leaders SET left_at=now() WHERE enterprise_id=$1 AND user_id=$2 AND left_at IS NULL", id, uid)
	if e != nil {
		return e
	}
	if e = changed(result, nil); e != nil {
		return e
	}
	_, e = q.ExecContext(ctx, "UPDATE users SET role='user',updated_at=now() WHERE id=$1", uid)
	return e
}
func (s *Service) SaveEnterprise(ctx context.Context, actor, id int64, in EnterpriseInput) (any, error) {
	name, status, e := governanceName(in.Name, in.Status)
	if e != nil {
		return nil, e
	}
	out, e := s.transaction(ctx, actor, 0, false, func(tc context.Context, q queryer, _ map[string]any) (any, error) {
		if !s.Enabled(tc) {
			return nil, forbidden()
		}
		if _, err := one(tc, q, "SELECT id FROM users WHERE id=$1 AND role='admin' AND status='active' AND deleted_at IS NULL", actor); err != nil {
			return nil, forbidden()
		}
		var before any
		allowMembers := false
		if in.AllowMemberUsageView != nil {
			allowMembers = *in.AllowMemberUsageView
		}
		if id == 0 {
			row, err := one(tc, q, "INSERT INTO enterprises(name,status,allow_member_usage_view) VALUES($1,$2,$3) RETURNING id", name, status, allowMembers)
			if err != nil {
				return nil, err
			}
			id = number(row, "id")
			if err = s.appointLeader(tc, q, id, in.LeaderID, true); err != nil {
				return nil, err
			}
			if err = recordEnterprise(tc, q, id, actor, "leader.appoint", map[string]any{"user_id": in.LeaderID, "before": "user", "after": "enterprise_admin"}); err != nil {
				return nil, err
			}
		} else {
			row, err := s.enterpriseScope(tc, q, actor, id, false)
			if err != nil {
				return nil, err
			}
			before = map[string]any{"name": row["name"], "status": row["status"], "allow_member_usage_view": row["allow_member_usage_view"]}
			if in.AllowMemberUsageView == nil {
				allowMembers = row["allow_member_usage_view"] == true
			}
			if _, err = q.ExecContext(tc, "UPDATE enterprises SET name=$2,status=$3,allow_member_usage_view=$4,updated_at=now() WHERE id=$1", id, name, status, allowMembers); err != nil {
				return nil, err
			}
		}
		return map[string]any{"id": id}, recordEnterprise(tc, q, id, actor, "enterprise.save", map[string]any{"before": before, "after": map[string]any{"name": name, "status": status, "allow_member_usage_view": allowMembers}})
	})
	if e == nil {
		s.invalidateEnterprise(ctx, id)
		if in.LeaderID > 0 {
			s.invalidate(ctx, in.LeaderID)
		}
	}
	return out, e
}
func (s *Service) AppointLeader(ctx context.Context, actor, id, uid int64, promote bool) (any, error) {
	out, e := s.transaction(ctx, actor, 0, false, func(tc context.Context, q queryer, _ map[string]any) (any, error) {
		if _, e := one(tc, q, "SELECT id FROM users WHERE id=$1 AND role='admin' AND status='active' AND deleted_at IS NULL", actor); e != nil {
			return nil, forbidden()
		}
		if _, e := s.enterpriseScope(tc, q, actor, id, false); e != nil {
			return nil, e
		}
		u, e := one(tc, q, "SELECT email,role FROM users WHERE id=$1", uid)
		if e != nil {
			return nil, e
		}
		if e = s.appointLeader(tc, q, id, uid, promote); e != nil {
			return nil, e
		}
		role := "user"
		if promote {
			role = "enterprise_admin"
		}
		return true, recordEnterprise(tc, q, id, actor, "leader.change", map[string]any{"user_id": uid, "email": u["email"], "before": u["role"], "after": role})
	})
	if e == nil {
		s.invalidate(ctx, uid)
	}
	return out, e
}
func (s *Service) AttachTeam(ctx context.Context, actor, id, tid int64) (any, error) {
	out, e := s.transaction(ctx, actor, 0, false, func(tc context.Context, q queryer, _ map[string]any) (any, error) {
		if _, e := one(tc, q, "SELECT id FROM users WHERE id=$1 AND role='admin' AND status='active' AND deleted_at IS NULL", actor); e != nil {
			return nil, forbidden()
		}
		if _, e := s.enterpriseScope(tc, q, actor, id, false); e != nil {
			return nil, e
		}
		t, e := one(tc, q, "SELECT id,name,enterprise_id FROM teams WHERE id=$1 FOR UPDATE", tid)
		if e != nil {
			return nil, e
		}
		if number(t, "enterprise_id") == id {
			return true, nil
		}
		if t["enterprise_id"] != nil {
			return nil, invalid("团队已归属其他企业，不能转移")
		}
		if _, e = q.ExecContext(tc, "UPDATE teams SET enterprise_id=$2,updated_at=now() WHERE id=$1", tid, id); e != nil {
			return nil, e
		}
		return true, recordEnterprise(tc, q, id, actor, "team.attach", map[string]any{"team_id": tid, "name": t["name"], "before": nil, "after": id})
	})
	if e == nil {
		s.invalidateTeam(ctx, tid)
	}
	return out, e
}
func (s *Service) invalidateEnterprise(ctx context.Context, id int64) {
	teams, e := rows(ctx, s.db, "SELECT id FROM teams WHERE enterprise_id=$1", id)
	if e == nil {
		for _, t := range teams {
			s.invalidateTeam(ctx, number(t, "id"))
		}
	}
	leaders, e := rows(ctx, s.db, "SELECT user_id FROM enterprise_leaders WHERE enterprise_id=$1 AND left_at IS NULL", id)
	if e == nil {
		for _, l := range leaders {
			s.invalidate(ctx, number(l, "user_id"))
		}
	}
}
func (s *Service) governedTeam(ctx context.Context, q queryer, actor, id, tid int64, write bool) (map[string]any, map[string]any, error) {
	company, e := s.enterpriseScope(ctx, q, actor, id, write)
	if e != nil {
		return nil, nil, e
	}
	t, e := one(ctx, q, "SELECT id,name,status,balance,enterprise_id FROM teams WHERE id=$1 AND enterprise_id=$2", tid, number(company, "id"))
	if e != nil {
		return nil, nil, e
	}
	return company, t, nil
}
func (s *Service) EnterpriseTeams(ctx context.Context, actor, id int64, f Filter) (any, error) {
	e, err := s.enterpriseScope(ctx, s.db, actor, id, false)
	if err != nil {
		return nil, err
	}
	f.EnterpriseID = number(e, "id")
	return s.ListTeams(ctx, f)
}
func (s *Service) EnterpriseStatistics(ctx context.Context, actor, id int64, f Filter) (any, error) {
	e, err := s.enterpriseScope(ctx, s.db, actor, id, false)
	if err != nil {
		return nil, err
	}
	id = number(e, "id")
	if f.TeamID > 0 {
		if _, err = one(ctx, s.db, "SELECT id FROM teams WHERE id=$1 AND enterprise_id=$2", f.TeamID, id); err != nil {
			return nil, err
		}
	}
	f = f.normalized()
	return one(ctx, s.db, `SELECT
 (SELECT count(*) FROM teams t WHERE t.enterprise_id=$1 AND ($2::bigint=0 OR t.id=$2)) AS team_count,
 (SELECT count(*) FROM team_memberships m JOIN teams t ON t.id=m.team_id WHERE t.enterprise_id=$1 AND m.left_at IS NULL AND ($2::bigint=0 OR t.id=$2)) AS member_count,
 (SELECT COALESCE(sum(t.balance),0) FROM teams t WHERE t.enterprise_id=$1 AND ($2::bigint=0 OR t.id=$2)) AS balance,
 (SELECT COALESCE(sum(gr.remaining),0) FROM team_subscription_grants gr JOIN teams t ON t.id=gr.team_id WHERE t.enterprise_id=$1 AND ($2::bigint=0 OR t.id=$2)) AS remaining_subscriptions,
 count(l.id) AS requests,COALESCE(sum(l.input_tokens+l.output_tokens+l.cache_creation_tokens+l.cache_read_tokens),0) AS tokens,COALESCE(sum(l.actual_cost),0) AS cost
 FROM usage_logs l JOIN teams t ON t.id=l.organization_team_id WHERE t.enterprise_id=$1 AND ($2::bigint=0 OR t.id=$2) AND l.created_at>=$3 AND l.created_at<$4`, id, f.TeamID, f.From, f.To)
}
func (s *Service) EnterpriseAdministrators(ctx context.Context, actor, id int64, f Filter) (any, error) {
	e, err := s.enterpriseScope(ctx, s.db, actor, id, false)
	if err != nil {
		return nil, err
	}
	args := []any{number(e, "id")}
	search := searchClause(f.Search, &args, "u.email", "u.username", "t.name")
	return page(ctx, s.db, `SELECT u.id,u.email,u.username,u.status,m.status AS member_status,t.id AS team_id,t.name AS team_name FROM teams t JOIN team_memberships m ON m.team_id=t.id JOIN users u ON u.id=m.user_id WHERE t.enterprise_id=$1 AND m.left_at IS NULL AND u.role='team_admin' AND u.deleted_at IS NULL`+search+` ORDER BY t.id,u.id`, args, f)
}
func (s *Service) AdministratorCandidates(ctx context.Context, actor, id, tid int64, f Filter) (any, error) {
	if _, _, e := s.governedTeam(ctx, s.db, actor, id, tid, false); e != nil {
		return nil, e
	}
	return page(ctx, s.db, `SELECT u.id,u.email,u.username,u.role,u.status,m.status AS member_status FROM team_memberships m JOIN users u ON u.id=m.user_id WHERE m.team_id=$1 AND m.left_at IS NULL AND m.status='active' AND u.deleted_at IS NULL AND (u.role='team_admin' OR (u.role='user' AND u.status='active')) ORDER BY u.id`, []any{tid}, f)
}
func (s *Service) EnterpriseEvents(ctx context.Context, actor, id int64, f Filter) (any, error) {
	e, err := s.enterpriseScope(ctx, s.db, actor, id, false)
	if err != nil {
		return nil, err
	}
	id = number(e, "id")
	if f.TeamID > 0 {
		if _, err = one(ctx, s.db, "SELECT id FROM teams WHERE id=$1 AND enterprise_id=$2", f.TeamID, id); err != nil {
			return nil, err
		}
	}
	args := []any{id, f.TeamID}
	search := searchClause(f.Search, &args, "x.team_name", "x.kind", "u.email")
	return page(ctx, s.db, `SELECT x.*,'success' AS outcome,u.email AS actor_email FROM (
 SELECT 'enterprise:'||e.id AS id,NULL::bigint AS team_id,NULL::text AS team_name,e.actor_id,e.kind,e.details,e.created_at FROM enterprise_events e WHERE e.enterprise_id=$1 AND $2::bigint=0
 UNION ALL SELECT 'team:'||e.id,t.id,t.name,e.actor_id,e.kind,
 CASE WHEN e.kind IN ('funding','subscription.authorize') THEN jsonb_build_object('amount',e.amount,'details',e.details) ELSE e.details END,e.created_at
 FROM team_events e JOIN teams t ON t.id=e.team_id WHERE t.enterprise_id=$1 AND ($2::bigint=0 OR t.id=$2) AND e.kind IN ('team.save','administrator.change','administrator.create','funding','subscription.authorize')
 ) x JOIN users u ON u.id=x.actor_id WHERE true`+search+` ORDER BY x.created_at DESC,x.id DESC`, args, f)
}
func (s *Service) newTeamAdministrator(ctx context.Context, q queryer, tid, actor int64, in MemberInput) (int64, error) {
	address, e := email(in.Email)
	if e != nil {
		return 0, e
	}
	if len(in.Password) < 8 || len(in.Password) > 72 {
		return 0, invalid("密码长度须为 8–72 字节")
	}
	if len([]rune(in.Username)) > 100 {
		return 0, invalid("用户名过长")
	}
	u := &service.User{Email: address, Username: strings.TrimSpace(in.Username), Role: service.RoleTeamAdmin, Status: service.StatusActive, Concurrency: 5, SignupSource: "email"}
	if e = u.SetPassword(in.Password); e != nil {
		return 0, e
	}
	if e = s.users.CreateWithEmailAliasGuard(ctx, u); e != nil {
		return 0, e
	}
	m, e := one(ctx, q, "INSERT INTO team_memberships(team_id,user_id) VALUES($1,$2) RETURNING id", tid, u.ID)
	if e != nil {
		return 0, e
	}
	return u.ID, record(ctx, q, tid, actor, number(m, "id"), "administrator.create", map[string]any{"user_id": u.ID, "email": u.Email, "after": "team_admin"})
}
func (s *Service) GovernSaveTeam(ctx context.Context, actor, tid int64, in GovernTeamInput) (any, error) {
	name, status, e := governanceName(in.Name, in.Status)
	if e != nil {
		return nil, e
	}
	out, e := s.transaction(ctx, actor, 0, false, func(tc context.Context, q queryer, _ map[string]any) (any, error) {
		ent, e := s.enterpriseScope(tc, q, actor, 0, true)
		if e != nil {
			return nil, e
		}
		var before any
		if tid == 0 {
			t, e := one(tc, q, "INSERT INTO teams(name,status,enterprise_id) VALUES($1,$2,$3) RETURNING id", name, status, number(ent, "id"))
			if e != nil {
				return nil, e
			}
			tid = number(t, "id")
			if _, e = s.newTeamAdministrator(tc, q, tid, actor, in.Administrator); e != nil {
				return nil, e
			}
		} else {
			_, t, e := s.governedTeam(tc, q, actor, 0, tid, true)
			if e != nil {
				return nil, e
			}
			before = map[string]any{"name": t["name"], "status": t["status"]}
			if _, e = q.ExecContext(tc, "UPDATE teams SET name=$2,status=$3,updated_at=now() WHERE id=$1", tid, name, status); e != nil {
				return nil, e
			}
		}
		return map[string]any{"id": tid}, record(tc, q, tid, actor, 0, "team.save", map[string]any{"before": before, "after": map[string]any{"name": name, "status": status}})
	})
	if e == nil {
		s.invalidateTeam(ctx, tid)
	}
	return out, e
}
func (s *Service) GovernCreateAdministrator(ctx context.Context, actor, tid int64, in MemberInput) (any, error) {
	return s.transaction(ctx, actor, 0, false, func(tc context.Context, q queryer, _ map[string]any) (any, error) {
		if _, _, e := s.governedTeam(tc, q, actor, 0, tid, true); e != nil {
			return nil, e
		}
		uid, e := s.newTeamAdministrator(tc, q, tid, actor, in)
		return map[string]any{"id": uid}, e
	})
}
func (s *Service) GovernAppoint(ctx context.Context, actor, tid, uid int64, promote bool) (any, error) {
	out, e := s.transaction(ctx, actor, 0, false, func(tc context.Context, q queryer, _ map[string]any) (any, error) {
		if _, _, e := s.governedTeam(tc, q, actor, 0, tid, true); e != nil {
			return nil, e
		}
		before, e := one(tc, q, `SELECT u.role,u.email FROM users u JOIN team_memberships m ON m.user_id=u.id WHERE u.id=$1 AND m.team_id=$2 AND m.left_at IS NULL AND m.status='active' AND u.deleted_at IS NULL AND (($3 AND u.status='active' AND u.role IN ('user','team_admin')) OR (NOT $3 AND u.role='team_admin'))`, uid, tid, promote)
		if e != nil {
			return nil, e
		}
		if e = s.appoint(tc, q, tid, uid, promote); e != nil {
			return nil, e
		}
		role := "user"
		if promote {
			role = "team_admin"
		}
		return true, record(tc, q, tid, actor, 0, "administrator.change", map[string]any{"user_id": uid, "email": before["email"], "before": before["role"], "after": role})
	})
	if e == nil {
		s.invalidate(ctx, uid)
	}
	return out, e
}
