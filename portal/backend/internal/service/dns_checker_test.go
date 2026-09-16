package service

import (
	"context"
	"net"
	"testing"

	"github.com/jetswu/cloudsuite/portal/backend/internal/domain"
)

// fakeResolver returns canned MX/TXT results and records the lookup names so
// tests can assert the checker built the correct FQDN.
type fakeResolver struct {
	mx       map[string][]*net.MX
	txt      map[string][]string
	err      error
	lookedUp []string
}

func (f *fakeResolver) LookupMX(ctx context.Context, name string) ([]*net.MX, error) {
	f.lookedUp = append(f.lookedUp, "MX:"+name)
	if f.err != nil {
		return nil, f.err
	}
	return f.mx[name], nil
}

func (f *fakeResolver) LookupTXT(ctx context.Context, name string) ([]string, error) {
	f.lookedUp = append(f.lookedUp, "TXT:"+name)
	if f.err != nil {
		return nil, f.err
	}
	return f.txt[name], nil
}

func newTestChecker(f *fakeResolver) *DNSChecker {
	return &DNSChecker{Resolver: f, Attempts: 1}
}

func TestVerifyRecordMXLookupDomainNotValue(t *testing.T) {
	f := &fakeResolver{
		mx: map[string][]*net.MX{
			"example.com": {{
				Host: "mail.example.com.",
				Pref: 10,
			}},
		},
	}
	c := newTestChecker(f)

	rec := domain.DNSRecord{Purpose: domain.PurposeMX, Name: "@", Value: "mail.example.com"}
	status, errMsg := c.VerifyRecord(context.Background(), rec, "example.com")
	if status != domain.DNSRecordVerified {
		t.Fatalf("status = %q (err=%v), want verified", status, errMsg)
	}
	// Must have looked up the DOMAIN, not the record's value.
	if len(f.lookedUp) != 1 || f.lookedUp[0] != "MX:example.com" {
		t.Fatalf("lookup = %v, want [MX:example.com]", f.lookedUp)
	}
}

func TestVerifyRecordMXNormalizesTrailingDot(t *testing.T) {
	f := &fakeResolver{
		mx: map[string][]*net.MX{
			"example.com": {{
				Host: "MAIL.example.com.", // trailing dot + uppercase
				Pref: 10,
			}},
		},
	}
	c := newTestChecker(f)

	rec := domain.DNSRecord{Purpose: domain.PurposeMX, Name: "@", Value: "mail.example.com"}
	status, _ := c.VerifyRecord(context.Background(), rec, "example.com")
	if status != domain.DNSRecordVerified {
		t.Fatalf("status = %q, want verified (trailing dot / case must be normalised)", status)
	}
}

func TestVerifyRecordMXNXDomainIsFailed(t *testing.T) {
	f := &fakeResolver{err: &net.DNSError{Err: "no such host", IsNotFound: true}}
	c := newTestChecker(f)

	rec := domain.DNSRecord{Purpose: domain.PurposeMX, Name: "@", Value: "mail.example.com"}
	status, errMsg := c.VerifyRecord(context.Background(), rec, "example.com")
	if status != domain.DNSRecordFailed {
		t.Fatalf("status = %q, want failed", status)
	}
	if errMsg == nil || *errMsg == "" {
		t.Fatal("expected non-empty error detail for NXDOMAIN")
	}
}

func TestVerifyRecordSPFLookupApex(t *testing.T) {
	f := &fakeResolver{
		txt: map[string][]string{
			"example.com": {"v=spf1 mx -all"},
		},
	}
	c := newTestChecker(f)

	rec := domain.DNSRecord{Purpose: domain.PurposeSPF, Name: "@", Value: "v=spf1 mx -all"}
	status, _ := c.VerifyRecord(context.Background(), rec, "example.com")
	if status != domain.DNSRecordVerified {
		t.Fatalf("status = %q, want verified", status)
	}
	if len(f.lookedUp) != 1 || f.lookedUp[0] != "TXT:example.com" {
		t.Fatalf("lookup = %v, want [TXT:example.com]", f.lookedUp)
	}
}

func TestVerifyRecordDKIMBuildsFQDN(t *testing.T) {
	f := &fakeResolver{
		txt: map[string][]string{
			"v1-rsa-20260915._domainkey.example.com": {`v=DKIM1; k=rsa; p=abc123`},
		},
	}
	c := newTestChecker(f)

	rec := domain.DNSRecord{
		Purpose: domain.PurposeDKIM,
		Name:    "v1-rsa-20260915._domainkey",
		Value:   "v=DKIM1; k=rsa; p=abc123",
	}
	status, _ := c.VerifyRecord(context.Background(), rec, "example.com")
	if status != domain.DNSRecordVerified {
		t.Fatalf("status = %q, want verified", status)
	}
	if len(f.lookedUp) != 1 || f.lookedUp[0] != "TXT:v1-rsa-20260915._domainkey.example.com" {
		t.Fatalf("lookup = %v, want full DKIM FQDN", f.lookedUp)
	}
}

func TestVerifyRecordDMARCBuildsFQDN(t *testing.T) {
	f := &fakeResolver{
		txt: map[string][]string{
			"_dmarc.example.com": {`v=DMARC1; p=reject`},
		},
	}
	c := newTestChecker(f)

	rec := domain.DNSRecord{Purpose: domain.PurposeDMARC, Name: "_dmarc", Value: "v=DMARC1; p=reject"}
	status, _ := c.VerifyRecord(context.Background(), rec, "example.com")
	if status != domain.DNSRecordVerified {
		t.Fatalf("status = %q, want verified", status)
	}
	if len(f.lookedUp) != 1 || f.lookedUp[0] != "TXT:_dmarc.example.com" {
		t.Fatalf("lookup = %v, want full DMARC FQDN", f.lookedUp)
	}
}

func TestVerifyRecordSPFCaseInsensitiveContains(t *testing.T) {
	f := &fakeResolver{
		txt: map[string][]string{
			"example.com": {`v=SPF1 mx -all`}, // uppercase differs from expected
		},
	}
	c := newTestChecker(f)

	rec := domain.DNSRecord{Purpose: domain.PurposeSPF, Name: "@", Value: "v=spf1 mx -all"}
	status, _ := c.VerifyRecord(context.Background(), rec, "example.com")
	if status != domain.DNSRecordVerified {
		t.Fatalf("status = %q, want verified (SPF must be case-insensitive)", status)
	}
}

func TestVerifyRecordMismatch(t *testing.T) {
	f := &fakeResolver{
		mx: map[string][]*net.MX{
			"example.com": {{Host: "other.example.com.", Pref: 10}},
		},
	}
	c := newTestChecker(f)

	rec := domain.DNSRecord{Purpose: domain.PurposeMX, Name: "@", Value: "mail.example.com"}
	status, _ := c.VerifyRecord(context.Background(), rec, "example.com")
	if status != domain.DNSRecordMismatch {
		t.Fatalf("status = %q, want mismatch", status)
	}
}

func TestVerifyRecordUnsupportedPurpose(t *testing.T) {
	f := &fakeResolver{}
	c := newTestChecker(f)

	rec := domain.DNSRecord{Purpose: domain.DNSRecordPurpose("srv"), Name: "@", Value: "x"}
	status, errMsg := c.VerifyRecord(context.Background(), rec, "example.com")
	if status != domain.DNSRecordFailed {
		t.Fatalf("status = %q, want failed", status)
	}
	if errMsg == nil || *errMsg == "" {
		t.Fatal("expected error detail for unsupported purpose")
	}
}

func TestFQDN(t *testing.T) {
	cases := []struct{ name, domainName, want string }{
		{"", "example.com", "example.com"},
		{"@", "example.com", "example.com"},
		{"_dmarc", "example.com", "_dmarc.example.com"},
		{"sel._domainkey", "example.com", "sel._domainkey.example.com"},
	}
	for _, c := range cases {
		if got := fqdn(c.name, c.domainName); got != c.want {
			t.Errorf("fqdn(%q, %q) = %q, want %q", c.name, c.domainName, got, c.want)
		}
	}
}

func TestVerifyRecordDKIMJoinsMultiStringTXT(t *testing.T) {
	// Cloudflare splits a long RSA key into two character-strings (the
	// continuation starts with a space) and the Stalwart zone-file form
	// leaves a trailing ")". The stored expected value is the dirty variant:
	// multiple spaces at the split point plus the trailing paren.
	f := &fakeResolver{
		txt: map[string][]string{
			"v1-rsa-20260915._domainkey.example.com": {
				"v=DKIM1; k=rsa; h=sha256; p=MIIBIjANBgkqhkiG9FP+a3S",
				" Ma0G5CKrestwwIDAQAB)",
			},
		},
	}
	c := newTestChecker(f)

	rec := domain.DNSRecord{
		Purpose: domain.PurposeDKIM,
		Name:    "v1-rsa-20260915._domainkey",
		Value:   "v=DKIM1; k=rsa; h=sha256; p=MIIBIjANBgkqhkiG9FP+a3S    Ma0G5CKrestwwIDAQAB)",
	}
	status, errMsg := c.VerifyRecord(context.Background(), rec, "example.com")
	if status != domain.DNSRecordVerified {
		t.Fatalf("status = %q (err=%v), want verified (multi-string join + normalisation)", status, errMsg)
	}
}

func TestVerifyRecordDKIMEd25519SingleString(t *testing.T) {
	f := &fakeResolver{
		txt: map[string][]string{
			"v1-ed25519-20260915._domainkey.example.com": {
				"v=DKIM1; k=ed25519; p=11qYAYKxCrfVSZ7QWAsNLOzZ6w==",
			},
		},
	}
	c := newTestChecker(f)

	rec := domain.DNSRecord{
		Purpose: domain.PurposeDKIM,
		Name:    "v1-ed25519-20260915._domainkey",
		Value:   "v=DKIM1; k=ed25519; p=11qYAYKxCrfVSZ7QWAsNLOzZ6w==",
	}
	status, errMsg := c.VerifyRecord(context.Background(), rec, "example.com")
	if status != domain.DNSRecordVerified {
		t.Fatalf("status = %q (err=%v), want verified (single-string Ed25519)", status, errMsg)
	}
}

func TestVerifyRecordDKIMToleratesExtraSpacesInExpected(t *testing.T) {
	f := &fakeResolver{
		txt: map[string][]string{
			"v1-rsa-20260915._domainkey.example.com": {
				"v=DKIM1; k=rsa; p=AAAABBBBCCCC",
			},
		},
	}
	c := newTestChecker(f)

	rec := domain.DNSRecord{
		Purpose: domain.PurposeDKIM,
		Name:    "v1-rsa-20260915._domainkey",
		Value:   "v=DKIM1;  k=rsa;    p=AAAABBBBCCCC",
	}
	status, errMsg := c.VerifyRecord(context.Background(), rec, "example.com")
	if status != domain.DNSRecordVerified {
		t.Fatalf("status = %q (err=%v), want verified (extra spaces in expected normalised)", status, errMsg)
	}
}

func TestVerifyRecordDKIMMismatch(t *testing.T) {
	f := &fakeResolver{
		txt: map[string][]string{
			"v1-rsa-20260915._domainkey.example.com": {
				"v=DKIM1; k=rsa; p=AAAABBBBCCCC",
			},
		},
	}
	c := newTestChecker(f)

	rec := domain.DNSRecord{
		Purpose: domain.PurposeDKIM,
		Name:    "v1-rsa-20260915._domainkey",
		Value:   "v=DKIM1; k=rsa; p=XXXXYYYYZZZZ",
	}
	status, _ := c.VerifyRecord(context.Background(), rec, "example.com")
	if status != domain.DNSRecordMismatch {
		t.Fatalf("status = %q, want mismatch (different key must NOT verify)", status)
	}
}

func TestNormalizeDKIM(t *testing.T) {
	cases := []struct{ in, want string }{
		{"v=DKIM1; k=rsa; p=AB", "v=DKIM1;k=rsa;p=AB"},
		{`"v=DKIM1; k=rsa; p=AB"`, "v=DKIM1;k=rsa;p=AB"},
		{"v=DKIM1;  k=rsa;    p=AB)", "v=DKIM1;k=rsa;p=AB"},
		{"(v=DKIM1; k=rsa; p=AB )", "v=DKIM1;k=rsa;p=AB"},
		{"v=DKIM1;	k=rsa;\np=AB", "v=DKIM1;k=rsa;p=AB"},
	}
	for _, c := range cases {
		if got := normalizeDKIM(c.in); got != c.want {
			t.Errorf("normalizeDKIM(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
