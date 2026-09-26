package team

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type GrantInput struct {
	GroupID      int64 `json:"group_id"`
	ValidityDays int   `json:"validity_days"`
	Quantity     int   `json:"quantity"`
}
type AllocationInput struct {
	Kind        string `json:"kind"`
	MemberID    int64  `json:"member_id"`
	Amount      string `json:"amount"`
	GrantID     int64  `json:"grant_id"`
	OperationID string `json:"operation_id"`
}

func operation(v string) error {
	if len(v) < 8 || len(v) > 100 {
		return invalid("请提供 8–100 字符的唯一操作标识")
	}
	return nil
}
func replay(ctx context.Context, q queryer, teamID int64, op, fingerprint string) (any, bool, error) {
	if e := operation(op); e != nil {
		return nil, false, e
	}
	r, e := rows(ctx, q, "SELECT fingerprint,result FROM team_events WHERE team_id=$1 AND operation_id=$2", teamID, op)
	if e != nil {
		return nil, false, e
	}
	if len(r) == 0 {
		return nil, false, nil
	}
	if str(r[0], "fingerprint") != fingerprint {
		return nil, false, &Error{409, "操作标识已用于其他请求"}
	}
	return r[0]["result"], true, nil
}
func eventOperation(ctx context.Context, q queryer, teamID, actor, memberID int64, kind, amount, before, after, op, fingerprint string, details, result any) error {
	data, e := json.Marshal(details)
	if e != nil {
		return e
	}
	out, e := json.Marshal(result)
	if e != nil {
		return e
	}
	var mid any
	if memberID > 0 {
		mid = memberID
	}
	var b, a any
	if before != "" {
		b = before
		a = after
	}
	_, e = q.ExecContext(ctx, `INSERT INTO team_events(team_id,actor_id,member_id,kind,amount,balance_before,balance_after,operation_id,fingerprint,details,result) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, teamID, actor, mid, kind, amount, b, a, op, fingerprint, string(data), string(out))
	return e
}
func (s *Service) Fund(ctx context.Context, actor, teamID int64, value, op string) (any, error) {
	value, e := amount(value, false)
	if e != nil {
		return nil, e
	}
	fp := hash(fmt.Sprintf("fund:%d:%s", actor, value))
	return s.transaction(ctx, actor, teamID, true, func(tc context.Context, q queryer, scope map[string]any) (any, error) {
		if scope["platform_admin"] != true {
			return nil, forbidden()
		}
		if out, ok, e := replay(tc, q, teamID, op, fp); ok || e != nil {
			return out, e
		}
		r, e := one(tc, q, `UPDATE teams SET balance=balance+$2::numeric,updated_at=now() WHERE id=$1 AND balance+$2::numeric>=0 RETURNING (balance-$2::numeric)::text AS before,balance::text AS after`, teamID, value)
		if e != nil {
			if missing(e) {
				return nil, invalid("可分配余额不足，不能撤回已发放额度")
			}
			return nil, e
		}
		out := map[string]any{"balance": r["after"]}
		return out, eventOperation(tc, q, teamID, actor, 0, "funding", value, str(r, "before"), str(r, "after"), op, fp, map[string]any{}, out)
	})
}
func (s *Service) Grant(ctx context.Context, actor, teamID int64, in GrantInput, op string) (any, error) {
	if in.ValidityDays < 1 || in.ValidityDays > service.MaxValidityDays || in.Quantity < 1 || in.Quantity > 100000 {
		return nil, invalid("套餐天数或份数无效")
	}
	data, _ := json.Marshal(in)
	fp := hash(fmt.Sprintf("grant:%d:%s", actor, data))
	return s.transaction(ctx, actor, teamID, true, func(tc context.Context, q queryer, scope map[string]any) (any, error) {
		if scope["platform_admin"] != true {
			return nil, forbidden()
		}
		if out, ok, e := replay(tc, q, teamID, op, fp); ok || e != nil {
			return out, e
		}
		r, e := one(tc, q, `INSERT INTO team_subscription_grants(team_id,group_id,validity_days,quantity,remaining) SELECT $1,g.id,$3,$4,$4 FROM groups g JOIN team_allowed_groups ag ON ag.group_id=g.id AND ag.team_id=$1 WHERE g.id=$2 AND g.subscription_type='subscription' AND g.status='active' AND g.deleted_at IS NULL RETURNING id,remaining`, teamID, in.GroupID, in.ValidityDays, in.Quantity)
		if e != nil {
			if missing(e) {
				return nil, invalid("请选择已授权且有效的订阅分组")
			}
			return nil, e
		}
		return r, eventOperation(tc, q, teamID, actor, 0, "subscription.authorize", "0", "", "", op, fp, in, r)
	})
}
func (s *Service) Allocate(ctx context.Context, actor, teamID int64, in AllocationInput) (any, error) {
	if in.Kind != "balance" && in.Kind != "subscription" {
		return nil, invalid("无效资源类型")
	}
	if in.Kind == "balance" {
		v, e := amount(in.Amount, true)
		if e != nil {
			return nil, e
		}
		in.Amount = v
	}
	data, _ := json.Marshal(in)
	fp := hash(fmt.Sprintf("allocate:%d:%s", actor, data))
	var uid, gid int64
	out, e := s.transaction(ctx, actor, teamID, true, func(tc context.Context, q queryer, scope map[string]any) (any, error) {
		if str(scope, "status") != "active" {
			return nil, invalid("团队已停用")
		}
		if out, ok, e := replay(tc, q, teamID, in.OperationID, fp); ok || e != nil {
			return out, e
		}
		m, e := one(tc, q, `SELECT m.user_id FROM team_memberships m JOIN users u ON u.id=m.user_id WHERE m.id=$1 AND m.team_id=$2 AND m.status='active' AND m.left_at IS NULL AND u.status='active' AND u.deleted_at IS NULL FOR UPDATE OF m,u`, in.MemberID, teamID)
		if e != nil {
			return nil, e
		}
		uid = number(m, "user_id")
		if in.Kind == "balance" {
			r, e := one(tc, q, `UPDATE teams SET balance=balance-$2::numeric,updated_at=now() WHERE id=$1 AND balance>=$2::numeric RETURNING (balance+$2::numeric)::text AS before,balance::text AS after`, teamID, in.Amount)
			if e != nil {
				if missing(e) {
					return nil, invalid("团队可分配余额不足")
				}
				return nil, e
			}
			if _, e = q.ExecContext(tc, "UPDATE users SET balance=balance+$2::numeric,updated_at=now() WHERE id=$1", uid, in.Amount); e != nil {
				return nil, e
			}
			result := map[string]any{"allocated": in.Amount, "remaining": r["after"]}
			return result, eventOperation(tc, q, teamID, actor, in.MemberID, "balance.allocate", in.Amount, str(r, "before"), str(r, "after"), in.OperationID, fp, map[string]any{}, result)
		}
		g, e := one(tc, q, `UPDATE team_subscription_grants gr SET remaining=remaining-1 WHERE gr.id=$1 AND gr.team_id=$2 AND remaining>0 AND EXISTS(SELECT 1 FROM team_allowed_groups ag JOIN groups g ON g.id=ag.group_id WHERE ag.team_id=gr.team_id AND ag.group_id=gr.group_id AND g.status='active' AND g.deleted_at IS NULL AND g.subscription_type='subscription') RETURNING id,group_id,validity_days,remaining`, in.GrantID, teamID)
		if e != nil {
			if missing(e) {
				return nil, invalid("订阅配额不足或分组已失效")
			}
			return nil, e
		}
		gid = number(g, "group_id")
		sub, _, e := s.subscriptions.AssignOrExtendSubscriptionInTransaction(tc, &service.AssignSubscriptionInput{UserID: uid, GroupID: gid, ValidityDays: int(number(g, "validity_days")), AssignedBy: actor, Notes: fmt.Sprintf("Team %d allocation", teamID)})
		if e != nil {
			return nil, e
		}
		result := map[string]any{"subscription_id": sub.ID, "remaining": g["remaining"], "expires_at": sub.ExpiresAt}
		return result, eventOperation(tc, q, teamID, actor, in.MemberID, "subscription.allocate", "0", "", "", in.OperationID, fp, map[string]any{"grant_id": in.GrantID, "group_id": gid, "validity_days": g["validity_days"], "before": number(g, "remaining") + 1, "after": g["remaining"]}, result)
	})
	if e == nil && uid > 0 {
		s.invalidate(ctx, uid)
		if gid > 0 {
			s.subscriptions.InvalidateSubCache(uid, gid)
		}
	}
	return out, e
}
