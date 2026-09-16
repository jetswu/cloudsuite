package provisioning

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/jetswu/cloudsuite/portal/backend/internal/repository/authentik"
)

// user_provisioning status values (migration 00004 CHECK constraints).
const (
	ProvPending      = "pending"
	ProvProvisioning = "provisioning"
	ProvActive       = "active"
	ProvFailed       = "failed"
)

// Worker consumes provisioning jobs from Redis and executes them through the
// per-service connectors. User identity (uid/email/name) is resolved from the
// Authentik API at execution time, so neither the handler nor the recovery
// sweep needs to carry it. A periodic sweep re-enqueues jobs whose Redis
// enqueue was lost and failed jobs whose backoff expired — the pipeline
// self-heals without operator action.
type Worker struct {
	queue      *Queue
	store      *Store
	auth       *authentik.Client
	connectors map[string]Connector

	popTimeout time.Duration
	sweepEvery time.Duration
	staleAfter time.Duration
}

// NewWorker wires the worker. popTimeout is the Redis BRPOP block (5s), the
// sweep runs every minute and re-enqueues queued jobs older than staleAfter.
func NewWorker(q *Queue, st *Store, auth *authentik.Client, connectors []Connector) *Worker {
	byName := make(map[string]Connector, len(connectors))
	for _, c := range connectors {
		byName[c.Service()] = c
	}
	return &Worker{
		queue:      q,
		store:      st,
		auth:       auth,
		connectors: byName,
		popTimeout: 5 * time.Second,
		sweepEvery: time.Minute,
		staleAfter: 2 * time.Minute,
	}
}

// Run blocks until ctx is cancelled (graceful shutdown). It never returns an
// error: transient failures are logged and retried in the next loop pass.
func (w *Worker) Run(ctx context.Context) {
	log.Info().Msg("provisioning worker started")
	sweep := time.NewTicker(w.sweepEvery)
	defer sweep.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("provisioning worker stopped")
			return
		case <-sweep.C:
			w.sweepOnce(ctx)
		default:
		}

		p, err := w.queue.Pop(ctx, w.popTimeout)
		if ctx.Err() != nil {
			log.Info().Msg("provisioning worker stopped")
			return
		}
		if err != nil {
			log.Error().Err(err).Msg("provisioning: queue pop failed")
			time.Sleep(2 * time.Second)
			continue
		}
		if p == nil {
			continue // BRPOP timeout, no work
		}
		w.process(ctx, p)
	}
}

// process executes one job: resolve identity, run the connector, record the
// outcome. Failed jobs are retried with exponential backoff (max_attempts).
func (w *Worker) process(ctx context.Context, p *JobPayload) {
	if p.Action != ActionProvision {
		msg := "action " + p.Action + " not supported in Sprint 1.4a"
		_ = w.store.MarkJob(ctx, p.JobID, StatusFailed, 0, nil, &msg)
		log.Warn().Str("job", p.JobID).Msg(msg)
		return
	}
	conn, ok := w.connectors[p.Service]
	if !ok {
		msg := "no connector registered for service"
		_ = w.store.MarkJob(ctx, p.JobID, StatusFailed, 0, nil, &msg)
		log.Error().Str("job", p.JobID).Str("service", p.Service).Msg(msg)
		return
	}

	// Identity resolution: fresh payloads may lack uid (handler never had it)
	// and swept payloads always do; both paths end with a full identity.
	if p.UID == "" || p.UserEmail == "" || p.Name == "" {
		u, err := w.auth.GetUser(ctx, p.UserID)
		if err != nil {
			lg := log.With().Str("job", p.JobID).Int("user_id", p.UserID).Logger()
			lg.Error().Err(err).Msg("provisioning: resolve user failed, requeueing")
			_ = w.store.MarkJob(ctx, p.JobID, StatusQueued, 0, ptrTime(time.Now().Add(w.sweepEvery)), strPtr(err.Error()))
			return
		}
		if p.UID == "" {
			p.UID = u.UID
		}
		if p.UserEmail == "" {
			p.UserEmail = u.Email
		}
		if p.Name == "" {
			p.Name = u.Name
		}
	}

	lg := log.With().Str("job", p.JobID).Str("service", p.Service).
		Str("email", p.UserEmail).Int("user_id", p.UserID).Logger()

	attempts, maxAttempts, err := w.store.GetJob(ctx, p.JobID)
	if err != nil {
		lg.Error().Err(err).Msg("provisioning: load job failed")
		return
	}
	attempts++

	_ = w.store.MarkJob(ctx, p.JobID, StatusRunning, attempts, nil, nil)
	_ = w.store.SetServiceStatus(ctx, p.UserID, p.Service, ProvProvisioning, "")

	extID, perr := conn.Provision(ctx, *p)
	if perr != nil {
		_ = w.store.SetServiceStatus(ctx, p.UserID, p.Service, ProvFailed, perr.Error())
		if attempts >= maxAttempts {
			_ = w.store.MarkJob(ctx, p.JobID, StatusFailed, attempts, nil, strPtr(perr.Error()))
			lg.Error().Err(perr).Int("attempts", attempts).Msg("provisioning: job failed permanently")
			return
		}
		next := time.Now().Add(backoff(attempts))
		_ = w.store.MarkJob(ctx, p.JobID, StatusFailed, attempts, &next, strPtr(perr.Error()))
		lg.Warn().Err(perr).Int("attempts", attempts).Time("retry_at", next).Msg("provisioning: job failed, will retry")
		return
	}

	if extID != "" {
		_ = w.store.SetExternalID(ctx, p.UserID, p.Service, extID)
	}
	_ = w.store.SetServiceStatus(ctx, p.UserID, p.Service, ProvActive, "")
	_ = w.store.MarkJob(ctx, p.JobID, StatusSuccess, attempts, nil, nil)
	lg.Info().Str("external_id", extID).Msg("provisioning: job succeeded")
}

// sweepOnce re-enqueues: queued jobs stale for > staleAfter (lost enqueue)
// and failed jobs whose backoff expired (attempts < max, checked in SQL).
func (w *Worker) sweepOnce(ctx context.Context) {
	jobs, err := w.store.PendingJobs(ctx, w.staleAfter, 50)
	if err != nil {
		log.Error().Err(err).Msg("provisioning: sweep query failed")
		return
	}
	for _, j := range jobs {
		p := JobPayload{JobID: j.ID, UserID: j.UserID, Action: j.Action, Service: j.Service}
		if err := w.queue.Enqueue(ctx, p); err != nil {
			log.Error().Err(err).Str("job", j.ID).Msg("provisioning: sweep re-enqueue failed")
			continue
		}
		log.Info().Str("job", j.ID).Str("service", j.Service).Msg("provisioning: sweep re-enqueued job")
	}
}

func strPtr(s string) *string { return &s }

func ptrTime(t time.Time) *time.Time { return &t }
