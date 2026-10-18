package team

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

type Page struct {
	Items    []map[string]any `json:"items"`
	Total    int64            `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"page_size"`
}
type Filter struct {
	EnterpriseID int64
	TeamID       int64
	Mine         bool
	Page         int
	Size         int
	MemberID     int64
	KeyID        int64
	Model        string
	Search       string
	From         time.Time
	To           time.Time
}

func (f Filter) normalized() Filter {
	if f.Page < 1 {
		f.Page = 1
	}
	f.Size = (pagination.PaginationParams{PageSize: f.Size}).Limit()
	f.Search = strings.TrimSpace(f.Search)
	if f.To.IsZero() {
		f.To = time.Now()
	}
	if f.From.IsZero() {
		f.From = f.To.AddDate(0, 0, -30)
	}
	return f
}

// Columns are fixed SQL expressions supplied by each scoped list query.
func searchClause(search string, args *[]any, columns ...string) string {
	search = strings.TrimSpace(search)
	if search == "" {
		return ""
	}
	pattern := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(search)
	*args = append(*args, "%"+pattern+"%")
	predicates := make([]string, 0, len(columns))
	for _, column := range columns {
		predicates = append(predicates, fmt.Sprintf("%s ILIKE $%d", column, len(*args)))
	}
	return " AND (" + strings.Join(predicates, " OR ") + ")"
}
func page(ctx context.Context, q queryer, query string, args []any, f Filter) (*Page, error) {
	f = f.normalized()
	total, e := one(ctx, q, "SELECT count(*) AS total FROM ("+query+") counted", args...)
	if e != nil {
		return nil, e
	}
	args = append(args, f.Size, (f.Page-1)*f.Size)
	r, e := rows(ctx, q, query+fmt.Sprintf(" LIMIT $%d OFFSET $%d", len(args)-1, len(args)), args...)
	return &Page{r, number(total, "total"), f.Page, f.Size}, e
}
func (s *Service) Overview(ctx context.Context, actor, teamID int64, mine bool) (any, error) {
	t, e := s.scope(ctx, s.db, actor, teamID, false)
	if e != nil {
		return nil, e
	}
	manage := t["can_view_team"] == true && !mine
	t["can_manage"] = t["can_manage"] == true && !mine
	t["can_view_team"] = manage
	stats, e := one(ctx, s.db, `SELECT count(*) AS requests,COALESCE(sum(input_tokens+output_tokens+cache_creation_tokens+cache_read_tokens),0) AS tokens,COALESCE(sum(actual_cost),0) AS cost FROM usage_logs WHERE team_id=$1 AND ($2 OR user_id=$3)`, teamID, manage, actor)
	if e != nil {
		return nil, e
	}
	if manage {
		stats, e = s.organizationUsageTotal(ctx, teamID)
		if e != nil {
			return nil, e
		}
	}
	t["usage"] = stats
	if !manage {
		delete(t, "balance")
		allocation, err := one(ctx, s.db, `SELECT COALESCE(sum(e.amount),0) AS amount FROM team_events e JOIN team_memberships m ON m.id=e.member_id WHERE e.team_id=$1 AND m.user_id=$2 AND e.kind='balance.allocate'`, teamID, actor)
		if err != nil {
			return nil, err
		}
		t["allocated_balance"] = allocation["amount"]
		administrators, err := rows(ctx, s.db, `SELECT u.id,u.email,u.username FROM team_memberships m JOIN users u ON u.id=m.user_id WHERE m.team_id=$1 AND m.left_at IS NULL AND m.status='active' AND u.role='team_admin' AND u.status='active' AND u.deleted_at IS NULL ORDER BY u.id`, teamID)
		if err != nil {
			return nil, err
		}
		t["administrators"] = administrators
	} else {
		c, e := one(ctx, s.db, "SELECT count(*) AS total FROM team_memberships WHERE team_id=$1 AND left_at IS NULL", teamID)
		if e != nil {
			return nil, e
		}
		t["member_count"] = c["total"]
		resources, e := one(ctx, s.db, `SELECT
 (SELECT COALESCE(sum(remaining),0) FROM team_subscription_grants WHERE team_id=$1) AS remaining_subscriptions,
 (SELECT COALESCE(sum(amount),0) FROM team_events WHERE team_id=$1 AND kind='balance.allocate') AS allocated_balance`, teamID)
		if e != nil {
			return nil, e
		}
		t["remaining_subscriptions"] = resources["remaining_subscriptions"]
		t["allocated_balance"] = resources["allocated_balance"]
	}
	groups, e := rows(ctx, s.db, `SELECT g.id,g.name,g.subscription_type,g.status FROM team_allowed_groups ag JOIN groups g ON g.id=ag.group_id WHERE ag.team_id=$1 AND g.deleted_at IS NULL ORDER BY g.id`, teamID)
	if e != nil {
		return nil, e
	}
	t["groups"] = groups
	return t, nil
}
func (s *Service) List(ctx context.Context, actor, teamID int64, kind string, f Filter) (any, error) {
	scope, e := s.scope(ctx, s.db, actor, teamID, false)
	if e != nil {
		return nil, e
	}
	manage := scope["can_view_team"] == true && !f.Mine
	f = f.normalized()
	args := []any{teamID}
	var query string
	switch kind {
	case "members":
		if !manage {
			return nil, forbidden()
		}
		query = `SELECT m.id,m.user_id,u.email,u.username,u.role,m.status,m.joined_at,m.left_at,
 COALESCE((SELECT sum(e.amount) FROM team_events e WHERE e.member_id=m.id AND e.kind='balance.allocate'),0) AS allocated_balance
 FROM team_memberships m JOIN users u ON u.id=m.user_id WHERE m.team_id=$1` + searchClause(f.Search, &args, "u.email", "u.username") + ` ORDER BY m.id DESC`
	case "invitations":
		if !manage {
			return nil, forbidden()
		}
		query = `SELECT id,email,expires_at,accepted_at,revoked_at,created_at FROM team_invitations WHERE team_id=$1` + searchClause(f.Search, &args, "email") + ` ORDER BY id DESC`
	case "grants":
		if !manage {
			return nil, forbidden()
		}
		query = `SELECT gr.id,gr.group_id,g.name AS group_name,gr.validity_days,gr.quantity,gr.remaining FROM team_subscription_grants gr JOIN groups g ON g.id=gr.group_id WHERE gr.team_id=$1` + searchClause(f.Search, &args, "g.name") + ` ORDER BY gr.id DESC`
	case "keys":
		args = append(args, manage, actor, f.MemberID)
		query = `SELECT k.id,k.user_id,k.team_membership_id AS member_id,u.email,k.name,k.group_id,g.name AS group_name,g.platform,g.allow_messages_dispatch,g.subscription_type,k.status,
 left(k.key,7)||'…'||right(k.key,4) AS masked_key,k.quota,k.quota_used,k.rate_limit_5h,k.rate_limit_1d,k.rate_limit_7d,k.expires_at,m.status AS member_status
 ,CASE WHEN k.window_5h_start+INTERVAL '5 hours'<=now() THEN 0 ELSE k.usage_5h END AS usage_5h
 ,CASE WHEN k.window_1d_start+INTERVAL '24 hours'<=now() THEN 0 ELSE k.usage_1d END AS usage_1d
 ,CASE WHEN k.window_7d_start+INTERVAL '7 days'<=now() THEN 0 ELSE k.usage_7d END AS usage_7d
 ,CASE WHEN k.window_5h_start+INTERVAL '5 hours'>now() THEN k.window_5h_start+INTERVAL '5 hours' END AS reset_5h_at
 ,CASE WHEN k.window_1d_start+INTERVAL '24 hours'>now() THEN k.window_1d_start+INTERVAL '24 hours' END AS reset_1d_at
 ,CASE WHEN k.window_7d_start+INTERVAL '7 days'>now() THEN k.window_7d_start+INTERVAL '7 days' END AS reset_7d_at
 ,COALESCE(g.status='active' AND g.deleted_at IS NULL AND EXISTS(SELECT 1 FROM team_allowed_groups ag WHERE ag.team_id=k.team_id AND ag.group_id=k.group_id) AND (
 (g.subscription_type='subscription' AND EXISTS(SELECT 1 FROM user_subscriptions us WHERE us.user_id=k.user_id AND us.group_id=k.group_id AND us.status='active' AND us.expires_at>now() AND us.deleted_at IS NULL)) OR
 (g.subscription_type<>'subscription' AND ((NOT g.is_exclusive AND NOT u.restrict_public_groups) OR EXISTS(SELECT 1 FROM user_allowed_groups ug WHERE ug.user_id=k.user_id AND ug.group_id=k.group_id)))),false) AS can_use_group
 FROM api_keys k JOIN team_memberships m ON m.id=k.team_membership_id JOIN users u ON u.id=k.user_id LEFT JOIN groups g ON g.id=k.group_id
 WHERE k.team_id=$1 AND k.deleted_at IS NULL AND ($2 OR k.user_id=$3) AND ($4::bigint=0 OR k.team_membership_id=$4)` + searchClause(f.Search, &args, "k.name", "u.email", "g.name") + ` ORDER BY k.id DESC`
	case "ledger":
		args = append(args, manage, actor)
		query = `SELECT e.id,e.member_id,e.kind,'success' AS outcome,e.amount,CASE WHEN $2 THEN e.balance_before END AS balance_before,CASE WHEN $2 THEN e.balance_after END AS balance_after,e.details,e.created_at,u.email AS member_email,a.email AS actor_email
 FROM team_events e LEFT JOIN team_memberships m ON m.id=e.member_id LEFT JOIN users u ON u.id=m.user_id JOIN users a ON a.id=e.actor_id
 WHERE e.team_id=$1 AND ($2 OR (m.user_id=$3 AND e.kind IN ('balance.allocate','subscription.allocate')))` + searchClause(f.Search, &args, "e.kind", "u.email", "a.email") + ` ORDER BY e.id DESC`
	case "usage":
		args = append(args, manage, actor, f.From, f.To, f.MemberID, f.KeyID, f.Model)
		query = `SELECT l.id,l.team_membership_id AS member_id,l.api_key_id,u.email,COALESCE(NULLIF(l.requested_model,''),l.model) AS model,l.input_tokens,l.output_tokens,(l.cache_creation_tokens+l.cache_read_tokens) AS cache_tokens,l.actual_cost,l.created_at
 FROM usage_logs l JOIN users u ON u.id=l.user_id WHERE l.team_id=$1 AND ($2 OR l.user_id=$3) AND l.created_at>=$4 AND l.created_at<$5
 AND ($6::bigint=0 OR l.team_membership_id=$6) AND ($7::bigint=0 OR l.api_key_id=$7) AND ($8::text='' OR COALESCE(NULLIF(l.requested_model,''),l.model)=$8)` + searchClause(f.Search, &args, "u.email", "COALESCE(NULLIF(l.requested_model,''),l.model)") + ` ORDER BY l.created_at DESC,l.id DESC`
	default:
		return nil, notFound()
	}
	return page(ctx, s.db, query, args, f)
}
