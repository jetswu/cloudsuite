//go:build e2e

package service

// Sprint 1.3b doc 3.5 — manual test plan automated against the REAL Stalwart
// + Portal DB (run inside the compose network). Creates throwaway test
// domains, asserts DKIM mode behaviour, then deletes them. Never touches
// production domains.

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jetswu/cloudsuite/portal/backend/internal/database"
	"github.com/jetswu/cloudsuite/portal/backend/internal/domain"
	"github.com/jetswu/cloudsuite/portal/backend/internal/repository/postgres"
	"github.com/jetswu/cloudsuite/portal/backend/internal/repository/stalwart"
)

func newE2EService(t *testing.T) *DomainService {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	pool, err := database.Connect(ctx, database.Config{
		Host:     envOr("PORTAL_DB_HOST", "postgres"),
		Port:     envOr("PORTAL_DB_PORT", "5432"),
		Name:     envOr("PORTAL_DB_NAME", "portal"),
		User:     envOr("PORTAL_DB_USER", "cloudsuite"),
		Password: os.Getenv("PORTAL_DB_PASSWORD"),
	})
	if err != nil {
		t.Fatalf("connect portal db: %v", err)
	}
	t.Cleanup(pool.Close)

	client := stalwart.NewClient(
		envOr("STALWART_API_URL", "http://cloudsuite-stalwart:8080"),
		os.Getenv("STALWART_API_KEY"),
	)
	return NewDomainService(
		postgres.NewDomainRepo(pool),
		postgres.NewDNSRecordRepo(pool),
		client,
		NewDNSChecker(),
	)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func countPurpose(recs []domain.DNSRecord, p domain.DNSRecordPurpose) int {
	n := 0
	for _, r := range recs {
		if r.Purpose == p {
			n++
		}
	}
	return n
}

func TestE2EDKIMModeSelection(t *testing.T) {
	ctx := context.Background()
	svc := newE2EService(t)

	cases := []struct {
		name      string
		reqMode   domain.DKIMMode // "" exercises the default
		wantMode  domain.DKIMMode
		wantDKIM  int
	}{
		{name: "test-rsa.local", reqMode: "", wantMode: domain.DKIMModeRSA, wantDKIM: 1},
		{name: "test-ed25519.local", reqMode: domain.DKIMModeEd25519, wantMode: domain.DKIMModeEd25519, wantDKIM: 1},
		{name: "test-dual.local", reqMode: domain.DKIMModeDual, wantMode: domain.DKIMModeDual, wantDKIM: 2},
	}

	for _, tc := range cases {
		t.Run("create/"+string(tc.wantMode), func(t *testing.T) {
			detail, err := svc.CreateDomain(ctx, tc.name, tc.reqMode)
			if err != nil {
				t.Fatalf("CreateDomain(%s, %q): %v", tc.name, tc.reqMode, err)
			}
			t.Cleanup(func() {
				_ = svc.DeleteDomain(context.Background(), detail.Domain.ID)
			})
			if detail.Domain.DKIMMode != tc.wantMode {
				t.Fatalf("dkim_mode = %q, want %q", detail.Domain.DKIMMode, tc.wantMode)
			}
			if got := countPurpose(detail.Records, domain.PurposeDKIM); got != tc.wantDKIM {
				t.Fatalf("dkim records = %d, want %d", got, tc.wantDKIM)
			}
			// MX/SPF/DMARC always present regardless of mode.
			for _, p := range []domain.DNSRecordPurpose{domain.PurposeMX, domain.PurposeSPF, domain.PurposeDMARC} {
				if got := countPurpose(detail.Records, p); got != 1 {
					t.Fatalf("%s records = %d, want 1", p, got)
				}
			}
		})
	}

	// doc 3.5d: change mode on an existing domain -> regenerate DNS.
	t.Run("update/rsa-to-dual", func(t *testing.T) {
		detail, err := svc.CreateDomain(ctx, "test-mode-change.local", domain.DKIMModeRSA)
		if err != nil {
			t.Fatalf("CreateDomain: %v", err)
		}
		t.Cleanup(func() {
			_ = svc.DeleteDomain(context.Background(), detail.Domain.ID)
		})
		if got := countPurpose(detail.Records, domain.PurposeDKIM); got != 1 {
			t.Fatalf("pre-update dkim records = %d, want 1", got)
		}
		mxBefore := countPurpose(detail.Records, domain.PurposeMX)
		spfBefore := countPurpose(detail.Records, domain.PurposeSPF)
		dmarcBefore := countPurpose(detail.Records, domain.PurposeDMARC)

		updated, err := svc.UpdateDomainDKIMMode(ctx, detail.Domain.ID, domain.DKIMModeDual)
		if err != nil {
			t.Fatalf("UpdateDomainDKIMMode: %v", err)
		}
		if updated.Domain.DKIMMode != domain.DKIMModeDual {
			t.Fatalf("dkim_mode = %q, want dual", updated.Domain.DKIMMode)
		}
		if updated.Domain.Status != domain.DomainDNSInProgress {
			t.Fatalf("status = %q, want dns_in_progress", updated.Domain.Status)
		}
		if got := countPurpose(updated.Records, domain.PurposeDKIM); got != 2 {
			t.Fatalf("post-update dkim records = %d, want 2", got)
		}
		if countPurpose(updated.Records, domain.PurposeMX) != mxBefore ||
			countPurpose(updated.Records, domain.PurposeSPF) != spfBefore ||
			countPurpose(updated.Records, domain.PurposeDMARC) != dmarcBefore {
			t.Fatal("MX/SPF/DMARC records changed during mode update")
		}

		// No-op update (same mode) must succeed without churning records.
		noop, err := svc.UpdateDomainDKIMMode(ctx, detail.Domain.ID, domain.DKIMModeDual)
		if err != nil {
			t.Fatalf("no-op UpdateDomainDKIMMode: %v", err)
		}
		if got := countPurpose(noop.Records, domain.PurposeDKIM); got != 2 {
			t.Fatalf("post-noop dkim records = %d, want 2", got)
		}
	})
}
