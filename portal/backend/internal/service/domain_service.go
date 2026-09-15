// Package service: domain onboarding orchestration (Sprint 1.3).
//
// DomainService ties together the domain/dns repositories, the Stalwart
// client, and the DNS checker into the use cases the admin handlers invoke:
// create (validate + register in Stalwart + generate DNS records), verify,
// and delete (clean up both the portal DB and Stalwart).
package service

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/jetswu/cloudsuite/portal/backend/internal/domain"
	"github.com/jetswu/cloudsuite/portal/backend/internal/repository"
	"github.com/jetswu/cloudsuite/portal/backend/internal/repository/stalwart"
)

// domainNameRe is a deliberately simple domain shape check. It accepts
// multi-label domains without enforcing a specific TLD, mirroring the
// frontend validation.
var domainNameRe = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)+$`)

// StalwartClient is the subset of the Stalwart client the domain service
// needs. It is an interface so tests can substitute a fake.
type StalwartClient interface {
	RegisterDomain(ctx context.Context, name string) (stalwart.Domain, error)
	DeleteDomain(ctx context.Context, id string) error
	ListDomains(ctx context.Context) ([]stalwart.Domain, error)
}

// DomainService orchestrates domain onboarding.
type DomainService struct {
	domains  repository.DomainRepository
	records  repository.DNSRecordRepository
	stalwart StalwartClient
	dns      *DNSChecker
}

// NewDomainService wires the onboarding dependencies.
func NewDomainService(
	domains repository.DomainRepository,
	records repository.DNSRecordRepository,
	stalwart StalwartClient,
	dns *DNSChecker,
) *DomainService {
	return &DomainService{domains: domains, records: records, stalwart: stalwart, dns: dns}
}

// CreateDomain validates the name, registers it in Stalwart, parses the
// generated zone into MX/SPF/DKIM/DMARC records, and persists both the domain
// and its records. On any failure after Stalwart registration it attempts to
// roll the Stalwart domain back so no orphan is left behind.
func (s *DomainService) CreateDomain(ctx context.Context, name string) (*domain.DomainDetail, error) {
	name = strings.ToLower(strings.TrimSpace(name))
	if !validDomainName(name) {
		return nil, fmt.Errorf("invalid domain name")
	}

	// Uniqueness is enforced by the DB unique constraint, but check first for
	// a friendlier error.
	if _, err := s.domains.GetByName(ctx, name); err == nil {
		return nil, fmt.Errorf("domain %q already exists", name)
	}

	stalwartDomain, err := s.stalwart.RegisterDomain(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("register domain in stalwart: %w", err)
	}

	records, err := GenerateDNSRecords(name, stalwartDomain.DNSZoneFile)
	if err != nil {
		_ = s.stalwart.DeleteDomain(ctx, stalwartDomain.ID)
		return nil, fmt.Errorf("generate dns records: %w", err)
	}
	if len(records) == 0 {
		_ = s.stalwart.DeleteDomain(ctx, stalwartDomain.ID)
		return nil, fmt.Errorf("no onboarding records generated from zone file")
	}

	d := domain.Domain{
		ID:               uuid.NewString(),
		Name:             name,
		Status:           domain.DomainPending,
		StalwartDomainID: &stalwartDomain.ID,
	}
	if err := s.domains.Create(ctx, d); err != nil {
		_ = s.stalwart.DeleteDomain(ctx, stalwartDomain.ID)
		return nil, fmt.Errorf("insert domain: %w", err)
	}

	for i := range records {
		records[i].ID = uuid.NewString()
		records[i].DomainID = d.ID
	}
	if err := s.records.CreateBatch(ctx, records); err != nil {
		// Best effort: clean up the domain row and Stalwart object.
		_ = s.domains.Delete(ctx, d.ID)
		_ = s.stalwart.DeleteDomain(ctx, stalwartDomain.ID)
		return nil, fmt.Errorf("insert dns records: %w", err)
	}

	return &domain.DomainDetail{Domain: d, Records: records}, nil
}

// ListDomains returns all portal domains.
func (s *DomainService) ListDomains(ctx context.Context) ([]domain.Domain, error) {
	return s.domains.List(ctx)
}

// GetDomain returns a domain with its DNS records.
func (s *DomainService) GetDomain(ctx context.Context, id string) (*domain.DomainDetail, error) {
	d, err := s.domains.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	records, err := s.records.ListByDomain(ctx, id)
	if err != nil {
		return nil, err
	}
	if records == nil {
		records = []domain.DNSRecord{}
	}
	return &domain.DomainDetail{Domain: *d, Records: records}, nil
}

// DeleteDomain removes a domain from the portal DB and, when present, from
// Stalwart.
func (s *DomainService) DeleteDomain(ctx context.Context, id string) error {
	d, err := s.domains.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if d.StalwartDomainID != nil && *d.StalwartDomainID != "" {
		if err := s.stalwart.DeleteDomain(ctx, *d.StalwartDomainID); err != nil {
			return fmt.Errorf("delete stalwart domain: %w", err)
		}
	}
	return s.domains.Delete(ctx, id)
}

// VerifyDomain re-checks every DNS record of a domain against the live DNS,
// updates each record's status, and promotes the domain to "verified" when
// every required record verifies. It returns the refreshed detail.
func (s *DomainService) VerifyDomain(ctx context.Context, id string) (*domain.DomainDetail, error) {
	d, err := s.domains.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	records, err := s.records.ListByDomain(ctx, id)
	if err != nil {
		return nil, err
	}

	allVerified := true
	for _, rec := range records {
		status, errMsg := s.dns.VerifyRecord(ctx, rec, d.Name)
		if err := s.records.UpdateStatus(ctx, rec.ID, status, errMsg); err != nil {
			return nil, fmt.Errorf("update record %s status: %w", rec.Name, err)
		}
		if rec.IsRequired && status != domain.DNSRecordVerified {
			allVerified = false
		}
	}

	if len(records) == 0 {
		allVerified = false
	}

	newStatus := domain.DomainDNSInProgress
	now := time.Now()
	if allVerified {
		newStatus = domain.DomainVerified
		d.VerifiedAt = &now
	}
	d.Status = newStatus
	if err := s.domains.Update(ctx, *d); err != nil {
		return nil, fmt.Errorf("update domain status: %w", err)
	}

	// Re-read records to return fresh statuses.
	refreshed, err := s.records.ListByDomain(ctx, id)
	if err != nil {
		return nil, err
	}
	if refreshed == nil {
		refreshed = []domain.DNSRecord{}
	}
	return &domain.DomainDetail{Domain: *d, Records: refreshed}, nil
}

func validDomainName(name string) bool {
	if len(name) > 253 {
		return false
	}
	return domainNameRe.MatchString(name)
}
