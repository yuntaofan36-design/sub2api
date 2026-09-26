package team

import (
	"context"
	"math"
	"strings"
	"time"
)

type KeyInput struct {
	MemberID    int64      `json:"member_id"`
	Name        string     `json:"name"`
	GroupID     int64      `json:"group_id"`
	Status      string     `json:"status"`
	Quota       float64    `json:"quota"`
	RateLimit5h float64    `json:"rate_limit_5h"`
	RateLimit1d float64    `json:"rate_limit_1d"`
	RateLimit7d float64    `json:"rate_limit_7d"`
	ExpiresAt   *time.Time `json:"expires_at"`
}

func (s *Service) SaveKey(ctx context.Context, actor, teamID, keyID int64, in KeyInput) (any, error) {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" || len([]rune(in.Name)) > 100 {
		return nil, invalid("Key 名称须为 1–100 个字符")
	}
	if in.Status == "" {
		in.Status = "active"
	}
	if in.Status != "active" && in.Status != "disabled" {
		return nil, invalid("无效 Key 状态")
	}
	for _, v := range []float64{in.Quota, in.RateLimit5h, in.RateLimit1d, in.RateLimit7d} {
		if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v >= 1e12 {
			return nil, invalid("使用限额必须为非负数")
		}
	}
	var uid int64
	out, e := s.transaction(ctx, actor, teamID, true, func(tc context.Context, q queryer, scope map[string]any) (any, error) {
		if str(scope, "status") != "active" {
			return nil, invalid("团队已停用")
		}
		m, e := one(tc, q, `SELECT m.user_id FROM team_memberships m JOIN users u ON u.id=m.user_id
 WHERE m.id=$1 AND m.team_id=$2 AND m.status='active' AND m.left_at IS NULL AND u.status='active' AND u.deleted_at IS NULL`, in.MemberID, teamID)
		if e != nil {
			return nil, e
		}
		uid = number(m, "user_id")
		if _, e = one(tc, q, `SELECT g.id FROM groups g JOIN team_allowed_groups ag ON ag.group_id=g.id AND ag.team_id=$1 JOIN users u ON u.id=$3
 WHERE g.id=$2 AND g.status='active' AND g.deleted_at IS NULL AND (
 (g.subscription_type='subscription' AND EXISTS(SELECT 1 FROM user_subscriptions us WHERE us.user_id=u.id AND us.group_id=g.id AND us.status='active' AND us.expires_at>now() AND us.deleted_at IS NULL)) OR
 (g.subscription_type<>'subscription' AND ((NOT g.is_exclusive AND NOT u.restrict_public_groups) OR EXISTS(SELECT 1 FROM user_allowed_groups ug WHERE ug.user_id=u.id AND ug.group_id=g.id))))`, teamID, in.GroupID, uid); e != nil {
			return nil, invalid("成员无权使用此分组，请先授予对应订阅或由平台授权")
		}
		var result map[string]any
		var before map[string]any
		if keyID == 0 {
			secret, e := token()
			if e != nil {
				return nil, e
			}
			secret = "sk-" + secret
			result, e = one(tc, q, `INSERT INTO api_keys(user_id,team_id,team_membership_id,key,name,group_id,status,quota,rate_limit_5h,rate_limit_1d,rate_limit_7d,expires_at)
 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12) RETURNING id,key`, uid, teamID, in.MemberID, secret, in.Name, in.GroupID, in.Status, in.Quota, in.RateLimit5h, in.RateLimit1d, in.RateLimit7d, in.ExpiresAt)
			if e != nil {
				return nil, e
			}
		} else {
			before, e = one(tc, q, `SELECT name,group_id,status,quota,rate_limit_5h,rate_limit_1d,rate_limit_7d,expires_at FROM api_keys WHERE id=$1 AND team_id=$2 AND team_membership_id=$3 AND deleted_at IS NULL`, keyID, teamID, in.MemberID)
			if e != nil {
				return nil, e
			}
			r, e := q.ExecContext(tc, `UPDATE api_keys SET name=$4,group_id=$5,status=$6,quota=$7,rate_limit_5h=$8,rate_limit_1d=$9,rate_limit_7d=$10,expires_at=$11,updated_at=now()
 WHERE id=$1 AND team_id=$2 AND team_membership_id=$3 AND deleted_at IS NULL`, keyID, teamID, in.MemberID, in.Name, in.GroupID, in.Status, in.Quota, in.RateLimit5h, in.RateLimit1d, in.RateLimit7d, in.ExpiresAt)
			if e = changed(r, e); e != nil {
				return nil, e
			}
			result = map[string]any{"id": keyID}
		}
		return result, record(tc, q, teamID, actor, in.MemberID, "key.save", map[string]any{"key_id": result["id"], "before": before, "after": in})
	})
	if e == nil {
		s.invalidate(ctx, uid)
	}
	return out, e
}
func (s *Service) Secret(ctx context.Context, actor, teamID, keyID int64) (any, error) {
	if _, e := s.scope(ctx, s.db, actor, teamID, false); e != nil {
		return nil, e
	}
	return one(ctx, s.db, `SELECT k.id,k.key FROM api_keys k JOIN team_memberships m ON m.id=k.team_membership_id JOIN teams t ON t.id=k.team_id
 WHERE k.id=$1 AND k.team_id=$2 AND k.user_id=$3 AND k.deleted_at IS NULL AND m.status='active' AND m.left_at IS NULL AND t.status='active'`, keyID, teamID, actor)
}
