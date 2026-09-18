package service

import (
	"context"
	"testing"
)

// A nil AuditLogger (dev/test wiring without a portal DB) must be a no-op:
// audit must never break a request.
func TestAuditLoggerNilIsNoop(t *testing.T) {
	var a *AuditLogger
	err := a.Log(context.Background(), AuditEntry{Action: "user.create"})
	if err != nil {
		t.Fatalf("nil AuditLogger must be a no-op, got %v", err)
	}
}
