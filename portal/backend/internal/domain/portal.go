// Package domain: portal domain onboarding read models (Sprint 1.3).
// These are the Portal database entities, distinct from the Authentik user
// models in domain.go.
package domain

import "time"

// DomainStatus enumerates the lifecycle of an onboarded domain.
//   pending          -> created, not yet in DNS setup
//   dns_in_progress  -> DNS setup started, some records pending
//   verified         -> all required DNS records verified
//   active           -> provisioning completed (Sprint 1.4+)
//   error            -> a failure occurred (e.g. Stalwart register failed)
type DomainStatus string

const (
	DomainPending        DomainStatus = "pending"
	DomainDNSInProgress  DomainStatus = "dns_in_progress"
	DomainVerified       DomainStatus = "verified"
	DomainActive         DomainStatus = "active"
	DomainError          DomainStatus = "error"
)

// DNSRecordStatus enumerates the verification state of a single DNS record.
type DNSRecordStatus string

const (
	DNSRecordPending  DNSRecordStatus = "pending"
	DNSRecordVerified DNSRecordStatus = "verified"
	DNSRecordFailed   DNSRecordStatus = "failed"
	DNSRecordMismatch DNSRecordStatus = "mismatch"
)

// DNSRecordPurpose labels what a record is for: mx, spf, dkim, dmarc.
type DNSRecordPurpose string

const (
	PurposeMX    DNSRecordPurpose = "mx"
	PurposeSPF   DNSRecordPurpose = "spf"
	PurposeDKIM  DNSRecordPurpose = "dkim"
	PurposeDMARC DNSRecordPurpose = "dmarc"
)

// Domain is a customer mail domain onboarded by the superadmin.
type Domain struct {
	ID               string       `json:"id"`
	Name             string       `json:"name"`
	TenantID         *string      `json:"tenant_id,omitempty"`
	Status           DomainStatus `json:"status"`
	StalwartDomainID *string      `json:"stalwart_domain_id,omitempty"`
	VerifiedAt       *time.Time   `json:"verified_at,omitempty"`
	CreatedAt        time.Time    `json:"created_at"`
	UpdatedAt        time.Time    `json:"updated_at"`
}

// DNSRecord is one DNS record required to onboard a domain.
type DNSRecord struct {
	ID            string           `json:"id"`
	DomainID      string           `json:"domain_id"`
	RecordType    string           `json:"record_type"` // MX | TXT
	Name          string           `json:"name"`        // @ | mail | _dmarc | <selector>._domainkey
	Value         string           `json:"value"`
	Priority      *int             `json:"priority,omitempty"` // MX only
	Purpose       DNSRecordPurpose `json:"purpose"`
	IsRequired    bool             `json:"is_required"`
	Status        DNSRecordStatus  `json:"status"`
	LastCheckedAt *time.Time       `json:"last_checked_at,omitempty"`
	LastError     *string          `json:"last_error,omitempty"`
	CreatedAt     time.Time        `json:"created_at"`
	UpdatedAt     time.Time        `json:"updated_at"`
}

// CreateDomainRequest is the POST /api/admin/domains payload.
type CreateDomainRequest struct {
	Name string `json:"name"`
}

// DomainDetail bundles a domain with its DNS records for the detail endpoint.
type DomainDetail struct {
	Domain  Domain      `json:"domain"`
	Records []DNSRecord `json:"records"`
}
