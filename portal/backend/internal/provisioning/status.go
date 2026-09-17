package provisioning

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// user_provisioning deprovision-lifecycle status values (migration 00005).
const (
	ProvPendingDelete = "pending_delete"
	ProvDeleted       = "deleted"
)

// DeprovisioningStates reports whether a per-service status marks a user the
// de-provision pipeline has claimed. Provision jobs arriving after this point
// are skipped — a deleted user must never be re-created in a service (the
// orphan re-creation incident in the 1.4a-fix cleanup motivated this guard).
func DeprovisioningStates() map[string]bool {
	return map[string]bool{ProvPendingDelete: true, ProvDeleted: true}
}

// ProvisioningState is the read model the admin UI renders per user. Empty
// statuses mean the user was never provisioned (pre-1.4a account).
type ProvisioningState struct {
	UserID   int    `json:"user_id"`
	Email    string `json:"email"`
	Stalwart string `json:"stalwart"`
	Nextcloud string `json:"nextcloud"`
	Odoo     string `json:"odoo"`
	LastError string `json:"last_error,omitempty"`
}

// ProvisioningStates returns the user_provisioning rows keyed by user id.
// Users without a provisioning row are simply absent from the map.
func (s *Store) ProvisioningStates(ctx context.Context) (map[int]ProvisioningState, error) {
	rows, err := s.pool.Query(ctx, `SELECT user_id, user_email, stalwart_status, nextcloud_status, odoo_status,
		COALESCE(last_error, '') FROM user_provisioning`)
	if err != nil {
		return nil, fmt.Errorf("list user_provisioning: %w", err)
	}
	defer rows.Close()

	out := make(map[int]ProvisioningState)
	for rows.Next() {
		var p ProvisioningState
		if err := rows.Scan(&p.UserID, &p.Email, &p.Stalwart, &p.Nextcloud, &p.Odoo, &p.LastError); err != nil {
			return nil, fmt.Errorf("scan user_provisioning: %w", err)
		}
		out[p.UserID] = p
	}
	return out, rows.Err()
}

// UserIdentity returns the email and the Nextcloud uid recorded for a user.
// The uid (Authentik sha256 hex) is the Nextcloud userid and lives in
// user_provisioning.nextcloud_external_id — needed by deprovision jobs
// because the Authentik user is typically already deleted by the time the
// worker executes them.
func (s *Store) UserIdentity(ctx context.Context, userID int) (email, ncUID string, err error) {
	err = s.pool.QueryRow(ctx, `SELECT user_email, COALESCE(nextcloud_external_id, '')
		FROM user_provisioning WHERE user_id = $1`, userID).Scan(&email, &ncUID)
	if err != nil {
		return "", "", fmt.Errorf("user_provisioning lookup user %d: %w", userID, err)
	}
	return email, ncUID, nil
}

// ServiceStatus returns the current <service>_status for one user. An absent
// user_provisioning row reads as "" (never provisioned).
func (s *Store) ServiceStatus(ctx context.Context, userID int, service string) (string, error) {
	cols, ok := serviceCols[service]
	if !ok {
		return "", fmt.Errorf("unknown service %q", service)
	}
	var status string
	query := fmt.Sprintf(`SELECT COALESCE(%s, '') FROM user_provisioning WHERE user_id = $1`, cols[0])
	err := s.pool.QueryRow(ctx, query, userID).Scan(&status)
	if err != nil {
		if err == pgx.ErrNoRows {
			return "", nil
		}
		return "", fmt.Errorf("read %s status user %d: %w", service, userID, err)
	}
	return status, nil
}

// ServiceStates returns {service: status} for all three services of a user.
func (s *Store) ServiceStates(ctx context.Context, userID int) (map[string]string, error) {
	var stal, nc, odoo string
	err := s.pool.QueryRow(ctx, `SELECT stalwart_status, nextcloud_status, odoo_status
		FROM user_provisioning WHERE user_id = $1`, userID).Scan(&stal, &nc, &odoo)
	if err != nil {
		if err == pgx.ErrNoRows {
			return map[string]string{}, nil
		}
		return nil, fmt.Errorf("read statuses user %d: %w", userID, err)
	}
	return map[string]string{"stalwart": stal, "nextcloud": nc, "odoo": odoo}, nil
}

// MarkPendingDelete claims a user for de-provisioning: all three service
// statuses flip to pending_delete and the claim clears last_error.
func (s *Store) MarkPendingDelete(ctx context.Context, userID int, email string) error {
	_, err := s.pool.Exec(ctx, `UPDATE user_provisioning SET
		user_email = $2,
		stalwart_status = CASE WHEN stalwart_status = 'deleted' THEN stalwart_status ELSE 'pending_delete' END,
		nextcloud_status = CASE WHEN nextcloud_status = 'deleted' THEN nextcloud_status ELSE 'pending_delete' END,
		odoo_status = CASE WHEN odoo_status = 'deleted' THEN odoo_status ELSE 'pending_delete' END,
		updated_at = now()
		WHERE user_id = $1`, userID, email)
	if err != nil {
		return fmt.Errorf("mark pending_delete user %d: %w", userID, err)
	}
	return nil
}

// ProvisioningState returns the read model for one user (pgx.ErrNoRows when
// the user has no user_provisioning row).
func (s *Store) ProvisioningState(ctx context.Context, userID int) (ProvisioningState, error) {
	var p ProvisioningState
	err := s.pool.QueryRow(ctx, `SELECT user_id, user_email, stalwart_status, nextcloud_status, odoo_status,
		COALESCE(last_error, '') FROM user_provisioning WHERE user_id = $1`, userID).
		Scan(&p.UserID, &p.Email, &p.Stalwart, &p.Nextcloud, &p.Odoo, &p.LastError)
	if err != nil {
		return ProvisioningState{}, err
	}
	return p, nil
}

// LatestUserJobs returns the most recent job row per service for a user,
// newest created_at wins. Powers the retry endpoint: it sees whether the
// last attempt failed (or is queued/running) before enqueuing again.
func (s *Store) LatestUserJobs(ctx context.Context, userID int) ([]JobRow, error) {
	rows, err := s.pool.Query(ctx, `SELECT DISTINCT ON (service) service, id, user_id, action, status
		FROM provisioning_jobs WHERE user_id = $1
		ORDER BY service, created_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("latest jobs user %d: %w", userID, err)
	}
	defer rows.Close()

	out := make([]JobRow, 0, 3)
	for rows.Next() {
		var j JobRow
		if err := rows.Scan(&j.Service, &j.ID, &j.UserID, &j.Action, &j.Status); err != nil {
			return nil, fmt.Errorf("scan latest job: %w", err)
		}
		out = append(out, j)
	}
	return out, rows.Err()
}
