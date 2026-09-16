// Package service holds the DNS verification logic used by the domain
// onboarding wizard (Sprint 1.3). It uses the Go standard resolver, with a
// short timeout and one retry to absorb DNS propagation latency.
package service

import (
	"context"
	"fmt"
	"net"
	"regexp"
	"strings"
	"time"

	"github.com/jetswu/cloudsuite/portal/backend/internal/domain"
)

// DNSResolver is the minimal lookup surface the checker needs. It is an
// interface so tests can substitute a fake; *net.Resolver satisfies it.
type DNSResolver interface {
	LookupMX(ctx context.Context, name string) ([]*net.MX, error)
	LookupTXT(ctx context.Context, name string) ([]string, error)
}

// DNSChecker verifies that a DNS record has been published and matches the
// expected value.
type DNSChecker struct {
	// Resolver defaults to the system resolver; overridable for tests.
	Resolver DNSResolver
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

// VerifyRecord checks a single DNS record against the live DNS. domainName is
// the fully-qualified domain the record belongs to (e.g. "example.com"). It is
// required because records are stored with a name RELATIVE to the domain
// ("@", "_dmarc", "<selector>._domainkey"), so the checker must combine them
// with domainName to build the correct lookup name. It returns
// (status, errorDetail). errorDetail is nil on success or mismatch (a mismatch
// is a normal state, not an error); it is only set when the lookup itself
// failed (NXDOMAIN, no records, timeout).
func (c *DNSChecker) VerifyRecord(ctx context.Context, rec domain.DNSRecord, domainName string) (domain.DNSRecordStatus, *string) {
	domainName = normalizeHost(domainName)

	switch rec.Purpose {
	case domain.PurposeMX:
		// Look up the MX records of the DOMAIN (not the expected host), then
		// confirm the expected host appears among them.
		return c.checkMX(ctx, domainName, rec.Value)
	case domain.PurposeSPF:
		// SPF is published as a TXT record at the domain apex.
		return c.checkTXT(ctx, domainName, rec.Value, true)
	case domain.PurposeDKIM:
		// rec.Name is relative: "<selector>._domainkey" → full name.
		// DKIM gets its own path: long keys are split into multiple TXT
		// character-strings that must be joined, and zone-file formatting
		// artifacts must be normalised away (see checkDKIM).
		return c.checkDKIM(ctx, fqdn(rec.Name, domainName), rec.Value)
	case domain.PurposeDMARC:
		// rec.Name is relative: "_dmarc" → full name.
		return c.checkTXT(ctx, fqdn(rec.Name, domainName), rec.Value, true)
	default:
		msg := "unsupported record purpose: " + string(rec.Purpose)
		return domain.DNSRecordFailed, &msg
	}
}

// checkMX verifies the expected MX host appears among domainName's MX records.
// expected is the full hostname to match (e.g. "mail.idchsuite.my.id"),
// compared after normalising trailing dots and lowercasing.
func (c *DNSChecker) checkMX(ctx context.Context, domainName, expected string) (domain.DNSRecordStatus, *string) {
	expected = normalizeHost(expected)
	if expected == "" {
		msg := "empty MX expected value"
		return domain.DNSRecordFailed, &msg
	}

	var lastErr error
	for i := 0; i < c.attempts(); i++ {
		lookupCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		records, err := c.Resolver.LookupMX(lookupCtx, domainName)
		cancel()
		if err == nil {
			for _, mx := range records {
				if normalizeHost(mx.Host) == expected {
					return domain.DNSRecordVerified, nil
				}
			}
			msg := fmt.Sprintf("MX %s not found in records for %s", expected, domainName)
			return domain.DNSRecordMismatch, &msg
		}
		lastErr = err
		time.Sleep(500 * time.Millisecond)
	}
	msg := fmt.Sprintf("MX lookup failed: %v", lastErr)
	return domain.DNSRecordFailed, &msg
}

// checkTXT verifies the expected value appears among the TXT records at the
// given fully-qualified name. Matching is by substring, case-insensitively;
// used by SPF and DMARC. Published TXT values are stripped of surrounding
// quotes and whitespace before comparison. DKIM does NOT go through here —
// see checkDKIM.
func (c *DNSChecker) checkTXT(ctx context.Context, fullName, expected string, caseInsensitive bool) (domain.DNSRecordStatus, *string) {
	expected = strings.TrimSpace(expected)
	if expected == "" {
		msg := "empty TXT expected value"
		return domain.DNSRecordFailed, &msg
	}

	var lastErr error
	for i := 0; i < c.attempts(); i++ {
		lookupCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		records, err := c.Resolver.LookupTXT(lookupCtx, fullName)
		cancel()
		if err == nil {
			for _, txt := range records {
				if containsTXT(txt, expected, caseInsensitive) {
					return domain.DNSRecordVerified, nil
				}
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

// checkDKIM verifies the expected DKIM value appears in the TXT records at
// the given fully-qualified name. DKIM keys exceed the 255-character per-
// string DNS limit, so authoritative servers (e.g. Cloudflare) split them
// into multiple adjacent character-strings that Go's LookupTXT surfaces as
// separate slice elements; the published value is the concatenation of all
// strings in order. Zone-file formatting artifacts (parentheses, extra
// whitespace — how Stalwart-generated records are stored) are stripped from
// both sides before a case-sensitive contains comparison.
func (c *DNSChecker) checkDKIM(ctx context.Context, fullName, expected string) (domain.DNSRecordStatus, *string) {
	expected = normalizeDKIM(expected)
	if expected == "" {
		msg := "empty DKIM expected value"
		return domain.DNSRecordFailed, &msg
	}

	var lastErr error
	for i := 0; i < c.attempts(); i++ {
		lookupCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		records, err := c.Resolver.LookupTXT(lookupCtx, fullName)
		cancel()
		if err == nil {
			joined := normalizeDKIM(strings.Join(records, ""))
			if strings.Contains(joined, expected) {
				return domain.DNSRecordVerified, nil
			}
			msg := "DKIM record value mismatch"
			return domain.DNSRecordMismatch, &msg
		}
		lastErr = err
		time.Sleep(500 * time.Millisecond)
	}
	msg := fmt.Sprintf("TXT lookup failed: %v", lastErr)
	return domain.DNSRecordFailed, &msg
}

// dkimSpaceRe collapses runs of whitespace (tabs/newlines appear in
// multi-line zone-file values) into single spaces before they are removed.
var dkimSpaceRe = regexp.MustCompile(`\s+`)

// normalizeDKIM strips formatting artifacts from a DKIM TXT value so dirty
// stored values (zone-file form: parens, multi-space splits) compare equal
// to live DNS responses. DKIM tag syntax has no significant spaces (the p=
// payload is base64), so all whitespace is removed from both sides.
func normalizeDKIM(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, `"`)  // outer quotes
	s = strings.Trim(s, "()") // zone-file parens
	s = dkimSpaceRe.ReplaceAllString(s, " ")
	return strings.ReplaceAll(s, " ", "")
}

func (c *DNSChecker) attempts() int {
	if c.Attempts <= 0 {
		return 2
	}
	return c.Attempts
}

// fqdn returns the fully-qualified lookup name for a relative record name.
// The apex name "@" resolves to the bare domain; everything else gets the
// domain appended (e.g. "_dmarc" → "_dmarc.example.com").
func fqdn(name, domainName string) string {
	name = strings.TrimSpace(name)
	if name == "" || name == "@" {
		return domainName
	}
	name = strings.TrimSuffix(name, ".")
	return name + "." + domainName
}

// containsTXT reports whether txt (a published TXT value) contains the
// expected value, after stripping surrounding quotes/whitespace. SPF and
// DMARC are case-insensitive; DKIM is case-sensitive.
func containsTXT(txt, expected string, caseInsensitive bool) bool {
	txt = strings.Trim(strings.TrimSpace(txt), `"`)
	expected = strings.Trim(strings.TrimSpace(expected), `"`)
	if caseInsensitive {
		return strings.Contains(strings.ToLower(txt), strings.ToLower(expected))
	}
	return strings.Contains(txt, expected)
}

// normalizeHost lowercases and strips a single trailing dot for MX comparison.
func normalizeHost(h string) string {
	h = strings.TrimSpace(h)
	h = strings.ToLower(h)
	h = strings.TrimSuffix(h, ".")
	return h
}
