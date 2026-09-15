package service

import (
	"testing"

	"github.com/jetswu/cloudsuite/portal/backend/internal/domain"
)

const sampleZone = `v1-ed25519-20260915._domainkey.test-domain.local. IN TXT "v=DKIM1; k=ed25519; h=sha256; p=zN74YCVeURzBde/RM8Znf2EJCuRqHtSKFjqxKRK24Dg="
v1-rsa-20260915._domainkey.test-domain.local. IN TXT (
    "v=DKIM1; k=rsa; h=sha256; p=MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAvqt"
    "c5B4qKcAKAKAKKAKAKKAKAKKAKAKKAKAKAKAKAKKAKAKAKAKKAKAKKAKAKKAKAKKAKAKKAKKAKAK"
)
test-domain.local. IN TXT "v=spf1 mx -all"
test-domain.local. IN MX 10 mail.idchsuite.my.id.
_dmarc.test-domain.local. IN TXT "v=DMARC1; p=reject; rua=mailto:postmaster@test-domain.local"
_caldavs._tcp.test-domain.local. IN SRV 0 1 443 mail.idchsuite.my.id.
mail.test-domain.local. IN TXT "v=spf1 a -all"
`

func TestGenerateDNSRecords(t *testing.T) {
	recs, err := GenerateDNSRecords("test-domain.local", sampleZone)
	if err != nil {
		t.Fatalf("GenerateDNSRecords: %v", err)
	}

	byPurpose := map[domain.DNSRecordPurpose]domain.DNSRecord{}
	for _, r := range recs {
		byPurpose[r.Purpose] = r
	}

	// MX
	mx, ok := byPurpose[domain.PurposeMX]
	if !ok {
		t.Fatal("missing MX record")
	}
	if mx.Name != "@" || mx.Value != "mail.idchsuite.my.id" || mx.RecordType != "MX" {
		t.Errorf("MX = %+v, want @ / mail.idchsuite.my.id / MX", mx)
	}
	if mx.Priority == nil || *mx.Priority != 10 {
		t.Errorf("MX priority = %v, want 10", mx.Priority)
	}

	// SPF
	spf, ok := byPurpose[domain.PurposeSPF]
	if !ok {
		t.Fatal("missing SPF record")
	}
	if spf.Name != "@" || spf.Value != "v=spf1 mx -all" || spf.RecordType != "TXT" {
		t.Errorf("SPF = %+v, want @ / v=spf1 mx -all / TXT", spf)
	}

	// DMARC
	dmarc, ok := byPurpose[domain.PurposeDMARC]
	if !ok {
		t.Fatal("missing DMARC record")
	}
	if dmarc.Name != "_dmarc" || !startsWith(dmarc.Value, "v=DMARC1") {
		t.Errorf("DMARC = %+v, want _dmarc / v=DMARC1...", dmarc)
	}

	// DKIM: two records expected (ed25519 + rsa)
	dkimCount := 0
	for _, r := range recs {
		if r.Purpose == domain.PurposeDKIM {
			dkimCount++
			if r.RecordType != "TXT" || !startsWith(r.Value, "v=DKIM1") {
				t.Errorf("DKIM record = %+v, want TXT / v=DKIM1...", r)
			}
		}
	}
	if dkimCount != 2 {
		t.Errorf("DKIM count = %d, want 2 (ed25519 + rsa)", dkimCount)
	}
}

func TestGenerateDNSRecordsEmpty(t *testing.T) {
	if _, err := GenerateDNSRecords("", "anything"); err == nil {
		t.Fatal("expected error for empty domain name")
	}
}

func startsWith(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
