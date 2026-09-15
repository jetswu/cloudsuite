// Package repository defines the persistence contracts the handlers depend
// on. Concrete implementations (pgx) live under internal/repository/postgres.
package repository

import (
	"context"

	"github.com/jetswu/cloudsuite/portal/backend/internal/domain"
)

// DomainRepository persists portal mail domains.
type DomainRepository interface {
	Create(ctx context.Context, d domain.Domain) error
	GetByID(ctx context.Context, id string) (*domain.Domain, error)
	GetByName(ctx context.Context, name string) (*domain.Domain, error)
	List(ctx context.Context) ([]domain.Domain, error)
	Update(ctx context.Context, d domain.Domain) error
	Delete(ctx context.Context, id string) error
}

// DNSRecordRepository persists the DNS records that must be published for a
// domain before it can receive mail.
type DNSRecordRepository interface {
	CreateBatch(ctx context.Context, records []domain.DNSRecord) error
	ListByDomain(ctx context.Context, domainID string) ([]domain.DNSRecord, error)
	UpdateStatus(ctx context.Context, id string, status domain.DNSRecordStatus, errMsg *string) error
}
