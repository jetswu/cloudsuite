package provisioning

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/jetswu/cloudsuite/portal/backend/internal/repository/stalwart"
)

// Actions and the default provision target set (Sprint 1.4a scope: provision
// only; de-provisioning ships in 1.4b).
const (
	ActionProvision = "provision"

	StatusQueued = "queued"
	StatusRunning = "running"
	StatusSuccess = "success"
	StatusFailed  = "failed"
)

// Services in provisioning order. Odoo is included as a deferred/verify job
// (its account is auto-created by Authentik OIDC on first login).
var Services = []string{"stalwart", "nextcloud", "odoo"}

// emailRe is the same simple local@domain.tld shape check the admin handler
// uses; the service re-validates so enqueue cannot be bypassed.
var emailRe = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

// ValidateEmailFormat rejects anything that is not a plain address. The error
// text is user-facing (admin UI).
func ValidateEmailFormat(email string) error {
	if !emailRe.MatchString(strings.TrimSpace(email)) {
		return errors.New("invalid email format")
	}
	return nil
}

// Service orchestrates provisioning: validation, persistence, enqueue.
type Service struct {
	queue   *Queue
	store   *Store
	stalwart *stalwart.Client // live domain registry (Stalwart) for validation
}

// NewService wires the pipeline. stalwart may be nil only in tests; without it
// domain validation is refused (fail closed).
func NewService(q *Queue, st *Store, stal *stalwart.Client) *Service {
	return &Service{queue: q, store: st, stalwart: stal}
}

// ProvisionUser validates the address against the registered mail domains and
// enqueues one provision job per service. It returns the job ids in service
// order. Partial enqueue failures are self-healing: the rows stay 'queued'
// and the worker's sweep re-enqueues them.
func (s *Service) ProvisionUser(ctx context.Context, userID int, email string) ([]string, error) {
	email = strings.TrimSpace(email)
	if err := ValidateEmailFormat(email); err != nil {
		return nil, err
	}
	if err := s.validateDomainRegistered(ctx, email); err != nil {
		return nil, err
	}
	return s.enqueue(ctx, userID, email, ActionProvision, Services)
}

// Precheck validates email format and domain registration without creating
// anything — used by the create-user handler to fail fast with a 400 before
// the Authentik user exists.
func (s *Service) Precheck(ctx context.Context, email string) error {
	email = strings.TrimSpace(email)
	if err := ValidateEmailFormat(email); err != nil {
		return err
	}
	return s.validateDomainRegistered(ctx, email)
}

func (s *Service) enqueue(ctx context.Context, userID int, email, action string, services []string) ([]string, error) {
	ids, err := s.store.CreateJobs(ctx, userID, email, action, services)
	if err != nil {
		return nil, err
	}
	for i, id := range ids {
		p := JobPayload{JobID: id, UserID: userID, UserEmail: email, Action: action, Service: services[i]}
		if err := s.queue.Enqueue(ctx, p); err != nil {
			return ids, fmt.Errorf("enqueue %s (job %s stays queued, worker will re-enqueue): %w", services[i], id, err)
		}
	}
	return ids, nil
}

// validateDomainRegistered requires the email domain to exist in Stalwart.
func (s *Service) validateDomainRegistered(ctx context.Context, email string) error {
	if s.stalwart == nil {
		return errors.New("domain validation unavailable (stalwart not configured)")
	}
	parts := strings.SplitN(email, "@", 2)
	if len(parts) != 2 || parts[1] == "" {
		return errors.New("invalid email format")
	}
	domains, err := s.stalwart.ListDomains(ctx)
	if err != nil {
		return fmt.Errorf("list registered domains: %w", err)
	}
	for _, d := range domains {
		if strings.EqualFold(d.Name, parts[1]) {
			return nil
		}
	}
	return fmt.Errorf("domain %s is not registered", strings.ToLower(parts[1]))
}

// backoff returns the delay before attempt n is retried (n starts at 1):
// 30s, 2m, 8m — capped by maxAttempts at the caller.
func backoff(attempt int) time.Duration {
	d := 30 * time.Second
	for i := 1; i < attempt; i++ {
		d *= 4
	}
	if d > 8*time.Minute {
		d = 8 * time.Minute
	}
	return d
}
