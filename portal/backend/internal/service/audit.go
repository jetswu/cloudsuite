package service

// audit.go implements the Sprint 1.5a append-only audit trail. AuditLogger
// writes one row per admin action into audit_logs; the table rejects UPDATE
// and DELETE via triggers, so entries are immutable.
//
// The logger is non-blocking by contract: an audit miss must never fail the
// business request it observes. Log uses a context detached from the request
// (handlers log right after writing the response; chi cancels the request
// context at that point, which would kill the INSERT mid-flight) and returns
// warnings instead of hard errors.

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

// AuditEntry describes one auditable action.
type AuditEntry struct {
	TenantID    *string
	ActorID     *int
	ActorEmail  string
	ActorType   string // "user" | "system" | "admin"
	Action      string // user.create, group.delete, domain.verify, ...
	TargetType  string // user, group, domain, provisioning
	TargetID    string
	ServiceCode string // stalwart, nextcloud, odoo; "" when n/a
	IPAddress   string
	UserAgent   string
	Metadata    map[string]any
}

// AuditLogger persists audit entries. Safe for concurrent use.
type AuditLogger struct {
	pool *pgxpool.Pool
}

// NewAuditLogger wires the logger to the portal DB pool. A nil pool (dev
// environments without a portal DB) makes Log a no-op.
func NewAuditLogger(pool *pgxpool.Pool) *AuditLogger {
	return &AuditLogger{pool: pool}
}

// Log inserts one audit row. Errors are logged as warnings and returned, but
// callers must treat audit failures as non-fatal.
func (a *AuditLogger) Log(ctx context.Context, e AuditEntry) error {
	if a == nil || a.pool == nil {
		return nil // audit not wired; nothing to write
	}
	if e.ActorType == "" {
		e.ActorType = "user"
	}
	var meta []byte
	if len(e.Metadata) > 0 {
		meta, _ = json.Marshal(e.Metadata)
	}
	if meta == nil {
		meta = []byte("{}")
	}

	// Detach from the request context with a bounded timeout: the write must
	// outlive the response and must not pile up when the DB is slow.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()

	_, err := a.pool.Exec(ctx, `
		INSERT INTO audit_logs
			(tenant_id, actor_id, actor_email, actor_type, action,
			 target_type, target_id, service_code, ip_address, user_agent, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NULLIF($9, '')::inet, $10, $11)`,
		e.TenantID, e.ActorID, e.ActorEmail, e.ActorType, e.Action,
		e.TargetType, e.TargetID, e.ServiceCode, e.IPAddress, e.UserAgent, meta,
	)
	if err != nil {
		log.Warn().Err(err).Str("action", e.Action).Msg("audit log insert failed")
	}
	return err
}
