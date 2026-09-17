package provisioning

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Store persists provisioning jobs and per-user provisioning state in the
// portal database (tables provisioning_jobs / user_provisioning, migration
// 00004). It is the durable side of the pipeline: the Redis list is only the
// hand-off between handler and worker, recovery relies on these rows.
type Store struct {
	pool *pgxpool.Pool
}

// NewStore returns a Store on top of the portal DB pool.
func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// CreateJobs inserts one provisioning job per service and seeds the per-user
// state row, in a single transaction. Idempotent on re-create: the state row
// keeps its progress and is only refreshed with the new email.
func (s *Store) CreateJobs(ctx context.Context, userID int, email, action string, services []string) ([]string, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `INSERT INTO user_provisioning (user_id, user_email) VALUES ($1, $2)
		ON CONFLICT (user_id) DO UPDATE SET user_email = EXCLUDED.user_email, updated_at = now()`,
		userID, email)
	if err != nil {
		return nil, fmt.Errorf("upsert user_provisioning: %w", err)
	}

	ids := make([]string, 0, len(services))
	for _, svc := range services {
		var id string
		if err := tx.QueryRow(ctx, `INSERT INTO provisioning_jobs (user_id, user_email, action, service)
			VALUES ($1, $2, $3, $4) RETURNING id`, userID, email, action, svc).Scan(&id); err != nil {
			return nil, fmt.Errorf("insert job %s: %w", svc, err)
		}
		ids = append(ids, id)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return ids, nil
}

// serviceCols maps a service name to its user_provisioning columns. The map is
// the whitelist that keeps dynamic column names out of user input.
var serviceCols = map[string][2]string{
	"stalwart":  {"stalwart_status", "stalwart_external_id"},
	"nextcloud": {"nextcloud_status", "nextcloud_external_id"},
	"odoo":      {"odoo_status", "odoo_external_id"},
}

// SetServiceStatus updates the <service>_status (and last_error on failure or
// when a message is given) of the user's provisioning row.
func (s *Store) SetServiceStatus(ctx context.Context, userID int, service, status, lastErr string) error {
	cols, ok := serviceCols[service]
	if !ok {
		return fmt.Errorf("unknown service %q", service)
	}
	query := fmt.Sprintf(`UPDATE user_provisioning SET %s = $2, last_error = NULL, updated_at = now() WHERE user_id = $1`, cols[0])
	args := []any{userID, status}
	if lastErr != "" {
		query = fmt.Sprintf(`UPDATE user_provisioning SET %s = $2, last_error = $3, updated_at = now() WHERE user_id = $1`, cols[0])
		args = append(args, lastErr)
	}
	if _, err := s.pool.Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("update %s status: %w", service, err)
	}
	return nil
}

// SetExternalID records the account id returned by the target service.
func (s *Store) SetExternalID(ctx context.Context, userID int, service, externalID string) error {
	cols, ok := serviceCols[service]
	if !ok {
		return fmt.Errorf("unknown service %q", service)
	}
	query := fmt.Sprintf(`UPDATE user_provisioning SET %s = $2, updated_at = now() WHERE user_id = $1`, cols[1])
	if _, err := s.pool.Exec(ctx, query, userID, externalID); err != nil {
		return fmt.Errorf("update %s external id: %w", service, err)
	}
	return nil
}

// GetJob returns the retry bookkeeping of a job.
func (s *Store) GetJob(ctx context.Context, jobID string) (attempts, maxAttempts int, err error) {
	err = s.pool.QueryRow(ctx, `SELECT attempts, max_attempts FROM provisioning_jobs WHERE id = $1`, jobID).
		Scan(&attempts, &maxAttempts)
	if err != nil {
		return 0, 0, fmt.Errorf("get job %s: %w", jobID, err)
	}
	return attempts, maxAttempts, nil
}

// MarkJob writes the job outcome. status is one of queued/running/success/
// failed; nextRetry is stored for requeue sweeps and must be nil otherwise.
func (s *Store) MarkJob(ctx context.Context, jobID, status string, attempts int, nextRetry *time.Time, lastErr *string) error {
	_, err := s.pool.Exec(ctx, `UPDATE provisioning_jobs
		SET status = $2, attempts = $3, next_retry_at = $4, last_error = $5, updated_at = now()
		WHERE id = $1`, jobID, status, attempts, nextRetry, lastErr)
	if err != nil {
		return fmt.Errorf("mark job %s: %w", jobID, err)
	}
	return nil
}

// JobRow is a job as requeued by the recovery sweeps. The worker refetches
// user identity (uid/email/name) from Authentik before executing it.
type JobRow struct {
	ID      string
	UserID  int
	Action  string
	Service string
	Status  string
}

// PendingJobs returns queued jobs whose enqueue may have been lost (Redis down
// at create time) plus failed jobs whose backoff expired, up to limit. The
// age/backoff comparisons go through make_interval so pgx sends the duration
// as seconds (a timestamptz cannot be compared against an int).
func (s *Store) PendingJobs(ctx context.Context, olderThan time.Duration, limit int) ([]JobRow, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, user_id, action, service FROM provisioning_jobs
		WHERE (status = 'queued' AND created_at < now() - make_interval(secs => $1) AND next_retry_at IS NULL)
		   OR (status = 'queued' AND next_retry_at <= now())
		   OR (status = 'failed' AND attempts < max_attempts AND next_retry_at <= now())
		ORDER BY created_at LIMIT $2`, olderThan.Seconds(), limit)
	if err != nil {
		return nil, fmt.Errorf("pending jobs: %w", err)
	}
	defer rows.Close()

	var out []JobRow
	for rows.Next() {
		var j JobRow
		if err := rows.Scan(&j.ID, &j.UserID, &j.Action, &j.Service); err != nil {
			return nil, fmt.Errorf("scan pending job: %w", err)
		}
		out = append(out, j)
	}
	return out, rows.Err()
}
