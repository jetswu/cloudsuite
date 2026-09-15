// Package service holds the DNS verification logic used by the domain
// onboarding wizard (Sprint 1.3). It uses the Go standard resolver, with a
// short timeout and one retry to absorb DNS propagation latency.
package service

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/jetswu/cloudsuite/portal/backend/internal/domain"
)

// DNSChecker verifies that a DNS record has been published and matches the
// expected value.
type DNSChecker struct {
	// Resolver defaults to the system resolver; overridable for tests.
	Resolver *net.Resolver
	// Attempts is the number of lookup attempts before giving up (default 2).
	Attempts int
}

// NewDNSChecker returns a DNSChecker with sensible defaults.
func NewDNSChecker() *DNSChecker {
	return &DNSChecker{
		Resolver: net.DefaultResolver,
		Attempts: 2,
	}
}

// VerifyRecord checks a single DNS record against the live DNS. It returns
// (status, errorDetail). errorDetail is nil on success or mismatch (a mismatch
// is a normal state, not an error); it is only set when the lookup itself
// failed (NXDOMAIN, no records, timeout).
func (c *DNSChecker) VerifyRecord(ctx context.Context, rec domain.DNSRecord) (domain.DNSRecordStatus, *string) {
	switch rec.RecordType {
	case "MX":
		return c.checkMX(ctx, rec.Value)
	case "TXT":
		return c.checkTXT(ctx, rec.Name, rec.Value)
	default:
		msg := "unsupported record type: " + rec.RecordType
		return domain.DNSRecordFailed, &msg
	}
}

// checkMX verifies the expected MX host appears among the domain's MX records.
// rec.Value for MX is the full hostname to match (e.g. "mail.idchsuite.my.id."),
// compared after normalising trailing dots and lowercasing.
func (c *DNSChecker) checkMX(ctx context.Context, expected string) (domain.DNSRecordStatus, *string) {
	expected = normalizeHost(expected)
	if expected == "" {
		msg := "empty MX expected value"
		return domain.DNSRecordFailed, &msg
	}

	var lastErr error
	for i := 0; i < c.attempts(); i++ {
		lookupCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		records, err := c.Resolver.LookupMX(lookupCtx, expected)
		cancel()
		if err == nil {
			for _, mx := range records {
				if normalizeHost(mx.Host) == expected {
					return domain.DNSRecordVerified, nil
				}
			}
			msg := fmt.Sprintf("MX %s not found in records", expected)
			return domain.DNSRecordMismatch, &msg
		}
		lastErr = err
		time.Sleep(500 * time.Millisecond)
	}
	msg := fmt.Sprintf("MX lookup failed: %v", lastErr)
	return domain.DNSRecordFailed, &msg
}

// checkTXT verifies the expected value appears among the TXT records at the
// given name (e.g. "@", "_dmarc", "v1-rsa-..._domainkey"). The comparison is
// tolerant of whitespace and quote-wrapping: the published value may be split
// across quoted strings or carry a trailing dot on the name.
func (c *DNSChecker) checkTXT(ctx context.Context, name, expected string) (domain.DNSRecordStatus, *string) {
	expected = strings.TrimSpace(expected)
	if expected == "" {
		msg := "empty TXT expected value"
		return domain.DNSRecordFailed, &msg
	}

	var lastErr error
	for i := 0; i < c.attempts(); i++ {
		lookupCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		records, err := c.Resolver.LookupTXT(lookupCtx, name)
		cancel()
		if err == nil {
			joined := strings.Join(records, "")
			if normalizeTXT(joined) == normalizeTXT(expected) {
				return domain.DNSRecordVerified, nil
			}
			msg := "TXT record value mismatch"
			return domain.DNSRecordMismatch, &msg
		}
		lastErr = err
		time.Sleep(500 * time.Millisecond)
	}
	msg := fmt.Sprintf("TXT lookup failed: %v", lastErr)
	return domain.DNSRecordFailed, &msg
}

func (c *DNSChecker) attempts() int {
	if c.Attempts <= 0 {
		return 2
	}
	return c.Attempts
}

// normalizeHost lowercases and strips a single trailing dot for MX comparison.
func normalizeHost(h string) string {
	h = strings.TrimSpace(h)
	h = strings.ToLower(h)
	h = strings.TrimSuffix(h, ".")
	return h
}

// normalizeTXT collapses all whitespace so that quoted-split TXT values match.
func normalizeTXT(s string) string {
	return strings.Join(strings.Fields(s), "")
}
