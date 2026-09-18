package postgres

// audit.go is the read model for the Sprint 1.5a audit trail
// (GET /api/admin/audit). Writes go through service.AuditLogger; the table
// itself is append-only (UPDATE/DELETE triggers).

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// AuditLog is one audit_logs row.
type AuditLog struct {
	ID          int64          `json:"id"`
	ActorID     *int           `json:"actor_id"`
	ActorEmail  string         `json:"actor_email"`
	ActorType   string         `json:"actor_type"`
	Action      string         `json:"action"`
	TargetType  *string        `json:"target_type"`
	TargetID    *string        `json:"target_id"`
	ServiceCode *string        `json:"service_code"`
	IPAddress   *string        `json:"ip_address"`
	UserAgent   *string        `json:"user_agent"`
	Metadata    map[string]any `json:"metadata"`
	CreatedAt   time.Time      `json:"created_at"`
}

// AuditFilter carries the list/export filters. Zero values are ignored.
// Service == "none" filters rows without a service (group/user actions).
type AuditFilter struct {
	ActorID int
	Action  string
	Service string
	From    time.Time
	To      time.Time
	Search  string
	Limit   int
	Offset  int
}

// AuditRepo reads audit_logs.
type AuditRepo struct {
	pool *pgxpool.Pool
}

// NewAuditRepo wires the repo to the portal DB pool.
func NewAuditRepo(pool *pgxpool.Pool) *AuditRepo {
	return &AuditRepo{pool: pool}
}

// where builds the WHERE clause and appends positional args in order.
func (f *AuditFilter) where(args *[]any) string {
	conds := []string{"1=1"}
	if f.ActorID > 0 {
		*args = append(*args, f.ActorID)
		conds = append(conds, fmt.Sprintf("actor_id = $%d", len(*args)))
	}
	if f.Action != "" {
		*args = append(*args, f.Action)
		conds = append(conds, fmt.Sprintf("action = $%d", len(*args)))
	}
	switch f.Service {
	case "":
	case "none":
		conds = append(conds, "service_code IS NULL")
	default:
		*args = append(*args, f.Service)
		conds = append(conds, fmt.Sprintf("service_code = $%d", len(*args)))
	}
	if !f.From.IsZero() {
		*args = append(*args, f.From)
		conds = append(conds, fmt.Sprintf("created_at >= $%d", len(*args)))
	}
	if !f.To.IsZero() {
		*args = append(*args, f.To)
		conds = append(conds, fmt.Sprintf("created_at <= $%d", len(*args)))
	}
	if f.Search != "" {
		*args = append(*args, "%"+f.Search+"%")
		n := len(*args)
		conds = append(conds, fmt.Sprintf(
			"(actor_email ILIKE $%d OR target_id ILIKE $%d OR action ILIKE $%d OR metadata::text ILIKE $%d)",
			n, n, n, n))
	}
	return strings.Join(conds, " AND ")
}

// List returns one page of audit rows (newest first) plus the total count
// for the given filter.
func (r *AuditRepo) List(ctx context.Context, f AuditFilter) ([]AuditLog, int, error) {
	if r == nil || r.pool == nil {
		return nil, 0, errors.New("audit repo not wired")
	}
	args := []any{}
	where := f.where(&args)

	var total int
	if err := r.pool.QueryRow(ctx,
		"SELECT count(*) FROM audit_logs WHERE "+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("audit count: %w", err)
	}

	args = append(args, f.Limit, f.Offset)
	limitArg, offsetArg := len(args)-1, len(args)
	rows, err := r.pool.Query(ctx, `
		SELECT id, actor_id, COALESCE(actor_email, ''), actor_type, action,
		       target_type, target_id, service_code, ip_address::text, user_agent,
		       metadata, created_at
		FROM audit_logs
		WHERE `+where+`
		ORDER BY created_at DESC, id DESC
		LIMIT $`+fmt.Sprint(limitArg)+` OFFSET $`+fmt.Sprint(offsetArg), args...)
	if err != nil {
		return nil, 0, fmt.Errorf("audit list: %w", err)
	}
	defer rows.Close()

	items := []AuditLog{}
	for rows.Next() {
		var a AuditLog
		var meta []byte
		if err := rows.Scan(&a.ID, &a.ActorID, &a.ActorEmail, &a.ActorType, &a.Action,
			&a.TargetType, &a.TargetID, &a.ServiceCode, &a.IPAddress, &a.UserAgent,
			&meta, &a.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("audit scan: %w", err)
		}
		a.Metadata = map[string]any{}
		if len(meta) > 0 {
			if err := json.Unmarshal(meta, &a.Metadata); err != nil {
				a.Metadata = map[string]any{"raw": string(meta)}
			}
		}
		items = append(items, a)
	}
	if rows.Err() != nil {
		return nil, 0, rows.Err()
	}
	return items, total, nil
}
