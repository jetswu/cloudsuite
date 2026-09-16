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
	RegisterDomain(ctx context.Context, name string, mode domain.DKIMMode) (stalwart.Domain, error)
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
func (s *DomainService) CreateDomain(ctx context.Context, name string, dkimMode domain.DKIMMode) (*domain.DomainDetail, error) {
	name = strings.ToLower(strings.TrimSpace(name))
	if !validDomainName(name) {
		return nil, fmt.Errorf("invalid domain name")
	}
	mode, err := domain.NormalizeDKIMMode(dkimMode)
	if err != nil {
		return nil, err
	}

	// Uniqueness is enforced by the DB unique constraint, but check first for
	// a friendlier error.
	if _, err := s.domains.GetByName(ctx, name); err == nil {
		return nil, fmt.Errorf("domain %q already exists", name)
	}

	stalwartDomain, err := s.stalwart.RegisterDomain(ctx, name, mode)
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
		DKIMMode:         mode,
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

// UpdateDomainDKIMMode switches an existing domain's DKIM mode (Sprint
// 1.3b). Stalwart 0.16 only generates DKIM keys when a domain is CREATED
// (verified against the live server on 2026-09-16: updating dkimManagement
// algorithms or destroying the signatures never regenerates them), so a
// mode change recreates the Stalwart domain object: destroy the old one
// (with its signatures), register a fresh one under the same name with the
// new mode, and regenerate the DKIM DNS records from the fresh zone. The
// portal domain id, its MX/SPF/DMARC rows and the wizard URL stay stable.
// If re-registering fails, the old mode is restored best-effort so the
// domain is not left missing from Stalwart.
func (s *DomainService) UpdateDomainDKIMMode(ctx context.Context, id string, dkimMode domain.DKIMMode) (*domain.DomainDetail, error) {
	mode, err := domain.NormalizeDKIMMode(dkimMode)
	if err != nil {
		return nil, err
	}
	d, err := s.domains.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if d.StalwartDomainID == nil || *d.StalwartDomainID == "" {
		return nil, fmt.Errorf("domain has no stalwart registration")
	}
	if d.DKIMMode == mode {
		// Nothing to do; avoid churning DNS records for a no-op update.
		return s.GetDomain(ctx, id)
	}

	// 1. Destroy the old Stalwart object (signatures are destroyed first by
	//    the client, otherwise Stalwart refuses with objectIsLinked).
	if err := s.stalwart.DeleteDomain(ctx, *d.StalwartDomainID); err != nil {
		return nil, fmt.Errorf("delete old stalwart domain: %w", err)
	}

	// 2. Re-register under the same name with the new mode.
	stalwartDomain, err := s.stalwart.RegisterDomain(ctx, d.Name, mode)
	if err != nil {
		// Roll back: recreate with the previous mode so the domain is not
		// left unregistered. Fresh keys mean the stored DKIM rows are stale
		// either way, so regenerate them from the restored zone too.
		if rbDomain, rbErr := s.stalwart.RegisterDomain(ctx, d.Name, d.DKIMMode); rbErr != nil {
			return nil, fmt.Errorf("register domain with new mode %q: %w (rollback also failed: %v — domain %q is NOT registered in Stalwart)", mode, err, rbErr, d.Name)
		} else if rbErr := s.regenerateDKIMRecords(ctx, d.ID, d.Name, rbDomain.DNSZoneFile); rbErr != nil {
			return nil, fmt.Errorf("register domain with new mode %q: %w (rollback registered the old mode but failed to refresh records: %v)", mode, err, rbErr)
		}
		return nil, fmt.Errorf("register domain with new mode %q: %w (rolled back to mode %q)", mode, err, d.DKIMMode)
	}

	// 3. Regenerate the stored DKIM records from the fresh zone (new keys).
	if err := s.regenerateDKIMRecords(ctx, id, d.Name, stalwartDomain.DNSZoneFile); err != nil {
		return nil, err
	}

	// 4. Point the portal row at the new Stalwart object, persist the new
	//    mode and reset verification state.
	d.StalwartDomainID = &stalwartDomain.ID
	d.DKIMMode = mode
	d.Status = domain.DomainDNSInProgress
	d.VerifiedAt = nil
	if err := s.domains.Update(ctx, *d); err != nil {
		return nil, fmt.Errorf("update domain dkim mode: %w", err)
	}

	return s.GetDomain(ctx, id)
}

// regenerateDKIMRecords replaces the stored DKIM DNS records with ones
// generated from the given zone file. MX/SPF/DMARC rows are untouched.
func (s *DomainService) regenerateDKIMRecords(ctx context.Context, domainID, name, zone string) error {
	records, err := GenerateDNSRecords(name, zone)
	if err != nil {
		return fmt.Errorf("generate dns records: %w", err)
	}
	dkim := []domain.DNSRecord{}
	for _, rec := range records {
		if rec.Purpose == domain.PurposeDKIM {
			dkim = append(dkim, rec)
		}
	}
	if len(dkim) == 0 {
		return fmt.Errorf("no dkim records generated from zone file")
	}
	if _, err := s.records.DeleteByPurpose(ctx, domainID, domain.PurposeDKIM); err != nil {
		return fmt.Errorf("delete old dkim records: %w", err)
	}
	for i := range dkim {
		dkim[i].ID = uuid.NewString()
		dkim[i].DomainID = domainID
	}
	if err := s.records.CreateBatch(ctx, dkim); err != nil {
		return fmt.Errorf("insert regenerated dkim records: %w", err)
	}
	return nil
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
