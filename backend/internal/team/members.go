package team

import (
	"context"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type MemberInput struct {
	Email    string `json:"email"`
	Username string `json:"username"`
	Password string `json:"password"`
}

func (s *Service) CreateMember(ctx context.Context, actor, teamID int64, in MemberInput) (any, error) {
	address, e := email(in.Email)
	if e != nil {
		return nil, e
	}
	if len(in.Password) < 8 || len(in.Password) > 72 {
		return nil, invalid("密码长度须为 8–72 字节")
	}
	if len([]rune(in.Username)) > 100 {
		return nil, invalid("用户名过长")
	}
	u := &service.User{Email: address, Username: strings.TrimSpace(in.Username), Role: service.RoleUser, Status: service.StatusActive, Concurrency: 5, SignupSource: "email"}
	if e = u.SetPassword(in.Password); e != nil {
		return nil, e
	}
	return s.transaction(ctx, actor, teamID, true, func(tc context.Context, q queryer, _ map[string]any) (any, error) {
		if e := s.users.CreateWithEmailAliasGuard(tc, u); e != nil {
			return nil, e
		}
		m, e := one(tc, q, "INSERT INTO team_memberships(team_id,user_id) VALUES($1,$2) RETURNING id,user_id,status", teamID, u.ID)
		if e != nil {
			return nil, e
		}
		return m, record(tc, q, teamID, actor, number(m, "id"), "member.create", map[string]any{"email": address})
	})
}
func (s *Service) ChangeMember(ctx context.Context, actor, teamID, memberID int64, status string, self bool) (any, error) {
	if status != "active" && status != "suspended" && status != "left" {
		return nil, invalid("无效成员状态")
	}
	var uid int64
	out, e := s.transaction(ctx, actor, teamID, !self, func(tc context.Context, q queryer, _ map[string]any) (any, error) {
		m, e := one(tc, q, `SELECT m.id,m.user_id,m.status,u.role FROM team_memberships m JOIN users u ON u.id=m.user_id WHERE m.id=$1 AND m.team_id=$2 AND m.left_at IS NULL FOR UPDATE OF m,u`, memberID, teamID)
		if e != nil {
			return nil, e
		}
		uid = number(m, "user_id")
		if self && (uid != actor || status != "left") {
			return nil, forbidden()
		}
		if str(m, "role") == service.RoleTeamAdmin {
			return nil, invalid("请先由平台管理员或所属企业负责人撤销团队管理员身份")
		}
		if _, e = q.ExecContext(tc, `UPDATE team_memberships SET status=$2::varchar,left_at=CASE WHEN $2::varchar='left' THEN now() ELSE NULL END WHERE id=$1`, memberID, status); e != nil {
			return nil, e
		}
		if status == "left" {
			if _, e = q.ExecContext(tc, "UPDATE api_keys SET status='disabled',updated_at=now() WHERE team_membership_id=$1", memberID); e != nil {
				return nil, e
			}
		}
		return true, record(tc, q, teamID, actor, memberID, "member."+status, map[string]any{"before": m["status"], "after": status})
	})
	if e == nil {
		s.invalidate(ctx, uid)
	}
	return out, e
}
func (s *Service) Invite(ctx context.Context, actor, teamID int64, address string) (any, error) {
	address, e := email(address)
	if e != nil {
		return nil, e
	}
	secret, e := token()
	if e != nil {
		return nil, e
	}
	return s.transaction(ctx, actor, teamID, true, func(tc context.Context, q queryer, _ map[string]any) (any, error) {
		i, e := one(tc, q, `INSERT INTO team_invitations(team_id,email,token_hash,created_by,expires_at) VALUES($1,$2,$3,$4,$5) RETURNING id,email,expires_at`, teamID, address, hash(secret), actor, time.Now().Add(7*24*time.Hour))
		if e != nil {
			return nil, e
		}
		if e = record(tc, q, teamID, actor, 0, "invitation.create", map[string]any{"invitation_id": i["id"], "email": address}); e != nil {
			return nil, e
		}
		i["token"] = secret
		return i, nil
	})
}
func (s *Service) RevokeInvite(ctx context.Context, actor, teamID, inviteID int64) (any, error) {
	return s.transaction(ctx, actor, teamID, true, func(tc context.Context, q queryer, _ map[string]any) (any, error) {
		r, e := q.ExecContext(tc, "UPDATE team_invitations SET revoked_at=now() WHERE id=$1 AND team_id=$2 AND accepted_at IS NULL AND revoked_at IS NULL", inviteID, teamID)
		if e = changed(r, e); e != nil {
			return nil, e
		}
		return true, record(tc, q, teamID, actor, 0, "invitation.revoke", map[string]any{"invitation_id": inviteID})
	})
}
func (s *Service) AcceptInvite(ctx context.Context, actor int64, secret string) (any, error) {
	if len(secret) != 64 {
		return nil, invalid("邀请链接无效")
	}
	return s.transaction(ctx, actor, 0, false, func(tc context.Context, q queryer, _ map[string]any) (any, error) {
		if !s.Enabled(tc) {
			return nil, forbidden()
		}
		i, e := one(tc, q, `SELECT i.id,i.team_id,i.email FROM team_invitations i JOIN teams t ON t.id=i.team_id WHERE token_hash=$1 AND accepted_at IS NULL AND revoked_at IS NULL AND expires_at>now() AND t.status='active' AND (t.enterprise_id IS NULL OR EXISTS(SELECT 1 FROM enterprises e WHERE e.id=t.enterprise_id AND e.status='active')) FOR UPDATE OF i,t`, hash(secret))
		if e != nil {
			return nil, invalid("邀请已失效、已使用或已撤销")
		}
		u, e := one(tc, q, "SELECT email,role FROM users WHERE id=$1 AND status='active' AND deleted_at IS NULL FOR UPDATE", actor)
		if e != nil {
			return nil, e
		}
		if str(u, "role") != service.RoleUser || !strings.EqualFold(str(u, "email"), str(i, "email")) {
			return nil, invalid("请使用受邀邮箱对应的普通用户账号接受邀请")
		}
		tid := number(i, "team_id")
		m, e := one(tc, q, "INSERT INTO team_memberships(team_id,user_id) VALUES($1,$2) RETURNING id", tid, actor)
		if e != nil {
			return nil, e
		}
		if _, e = q.ExecContext(tc, "UPDATE team_invitations SET accepted_at=now() WHERE id=$1", number(i, "id")); e != nil {
			return nil, e
		}
		return map[string]any{"team_id": tid}, record(tc, q, tid, actor, number(m, "id"), "invitation.accept", map[string]any{"invitation_id": i["id"]})
	})
}
