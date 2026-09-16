// Package domain: portal domain onboarding read models (Sprint 1.3).
// These are the Portal database entities, distinct from the Authentik user
// models in domain.go.
package domain

import (
	"fmt"
	"strings"
	"time"
)

// DomainStatus enumerates the lifecycle of an onboarded domain.
//   pending          -> created, not yet in DNS setup
//   dns_in_progress  -> DNS setup started, some records pending
//   verified         -> all required DNS records verified
//   active           -> provisioning completed (Sprint 1.4+)
//   error            -> a failure occurred (e.g. Stalwart register failed)
type DomainStatus string

const (
	DomainPending       DomainStatus = "pending"
	DomainDNSInProgress DomainStatus = "dns_in_progress"
	DomainVerified      DomainStatus = "verified"
	DomainActive        DomainStatus = "active"
	DomainError         DomainStatus = "error"
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

// DKIMMode enumerates the DKIM key management modes Stalwart supports
// (Sprint 1.3b). The portal stores the admin's choice and mirrors it into
// Stalwart's dkimManagement on domain create and update.
type DKIMMode string

const (
	// DKIMModeRSA signs with RSA only — universally supported by receivers.
	DKIMModeRSA DKIMMode = "rsa"
	// DKIMModeEd25519 signs with Ed25519 only — fast and compact, but legacy
	// receivers may reject the signature.
	DKIMModeEd25519 DKIMMode = "ed25519"
	// DKIMModeDual signs with both keys (Stalwart "Automatic" management).
	DKIMModeDual DKIMMode = "dual"
)

// DefaultDKIMMode is applied when a request omits dkim_mode.
const DefaultDKIMMode = DKIMModeRSA

// ValidDKIMMode reports whether m is one of the supported mode values.
func ValidDKIMMode(m DKIMMode) bool {
	switch m {
	case DKIMModeRSA, DKIMModeEd25519, DKIMModeDual:
		return true
	}
	return false
}

// NormalizeDKIMMode canonicalises user-supplied input: trims and lowercases,
// maps an empty value to the default (rsa), and rejects unknown values.
func NormalizeDKIMMode(m DKIMMode) (DKIMMode, error) {
	m = DKIMMode(strings.ToLower(strings.TrimSpace(string(m))))
	if m == "" {
		return DefaultDKIMMode, nil
	}
	if !ValidDKIMMode(m) {
		return "", fmt.Errorf("invalid dkim mode %q (want rsa|ed25519|dual)", m)
	}
	return m, nil
}

// Domain is a customer mail domain onboarded by the superadmin.
type Domain struct {
	ID               string       `json:"id"`
	Name             string       `json:"name"`
	TenantID         *string      `json:"tenant_id,omitempty"`
	Status           DomainStatus `json:"status"`
	StalwartDomainID *string      `json:"stalwart_domain_id,omitempty"`
	DKIMMode         DKIMMode     `json:"dkim_mode"`
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
	Name     string   `json:"name"`
	DKIMMode DKIMMode `json:"dkim_mode,omitempty"` // default: rsa
}

// UpdateDomainRequest is the PATCH /api/admin/domains/{id} payload
// (Sprint 1.3b): changing dkim_mode regenerates the DKIM DNS records.
type UpdateDomainRequest struct {
	DKIMMode DKIMMode `json:"dkim_mode"`
}

// DomainDetail bundles a domain with its DNS records for the detail endpoint.
type DomainDetail struct {
	Domain  Domain      `json:"domain"`
	Records []DNSRecord `json:"records"`
}
