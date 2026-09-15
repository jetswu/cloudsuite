// Package postgres: DNSRecordRepository implementation (pgx).
package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jetswu/cloudsuite/portal/backend/internal/domain"
)

// DNSRecordRepo is the pgx implementation of repository.DNSRecordRepository.
type DNSRecordRepo struct {
	pool *pgxpool.Pool
}

// NewDNSRecordRepo wires a DNSRecordRepo to the given pool.
func NewDNSRecordRepo(pool *pgxpool.Pool) *DNSRecordRepo {
	return &DNSRecordRepo{pool: pool}
}

const dnsRecordColumns = `id, domain_id, record_type, name, value, priority, purpose, is_required, status, last_checked_at, last_error, created_at, updated_at`

func scanDNSRecord(row interface{ Scan(...any) error }) (*domain.DNSRecord, error) {
	var r domain.DNSRecord
	err := row.Scan(
		&r.ID, &r.DomainID, &r.RecordType, &r.Name, &r.Value, &r.Priority,
		&r.Purpose, &r.IsRequired, &r.Status, &r.LastCheckedAt, &r.LastError,
		&r.CreatedAt, &r.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scan dns record: %w", err)
	}
	return &r, nil
}

// CreateBatch inserts multiple DNS records in a single transaction so a
// partially-written set never appears to readers.
func (r *DNSRecordRepo) CreateBatch(ctx context.Context, records []domain.DNSRecord) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin dns batch: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after commit

	for _, rec := range records {
		_, err := tx.Exec(ctx, `
			INSERT INTO dns_records
				(id, domain_id, record_type, name, value, priority, purpose, is_required)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			rec.ID, rec.DomainID, rec.RecordType, rec.Name, rec.Value,
			rec.Priority, rec.Purpose, rec.IsRequired,
		)
		if err != nil {
			return fmt.Errorf("insert dns record %s: %w", rec.Name, err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit dns batch: %w", err)
	}
	return nil
}

// ListByDomain returns all DNS records for a domain ordered by purpose.
func (r *DNSRecordRepo) ListByDomain(ctx context.Context, domainID string) ([]domain.DNSRecord, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+dnsRecordColumns+`
		FROM dns_records
		WHERE domain_id = $1
		ORDER BY
			CASE purpose
				WHEN 'mx' THEN 1
				WHEN 'spf' THEN 2
				WHEN 'dkim' THEN 3
				WHEN 'dmarc' THEN 4
				ELSE 5
			END,
			name`,
		domainID,
	)
	if err != nil {
		return nil, fmt.Errorf("list dns records: %w", err)
	}
	defer rows.Close()

	out := []domain.DNSRecord{}
	for rows.Next() {
		rec, err := scanDNSRecord(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *rec)
	}
	if rows.Err() != nil {
		return nil, fmt.Errorf("iterate dns records: %w", rows.Err())
	}
	return out, nil
}

// UpdateStatus writes the verification result of a single record, along with
// an optional error detail (nil clears it) and the check timestamp.
func (r *DNSRecordRepo) UpdateStatus(ctx context.Context, id string, status domain.DNSRecordStatus, errMsg *string) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE dns_records
		SET status = $2, last_error = $3, last_checked_at = now(), updated_at = now()
		WHERE id = $1`,
		id, status, errMsg,
	)
	if err != nil {
		return fmt.Errorf("update dns record status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
