package handlers

import (
	"net/http/httptest"
	"testing"
)

// The audit endpoints live behind the same guard as /api/admin; these unit
// tests cover request validation only (no DB).
func TestAuditFilterValidation(t *testing.T) {
	h := &AuditHandler{}
	cases := []struct {
		name  string
		query string
		ok    bool
	}{
		{"defaults", "", true},
		{"page+limit", "page=2&limit=25", true},
		{"actor", "actor_id=9", true},
		{"bad actor", "actor_id=x", false},
		{"negative actor", "actor_id=-1", false},
		{"limit bounds", "limit=0", false},
		{"limit high", "limit=501", false},
		{"limit edge", "limit=500", true},
		{"bad page", "page=0", false},
		{"bad from", "from=yesterday", false},
		{"date only", "from=2026-09-01&to=2026-09-18", true},
		{"rfc3339", "from=2026-09-01T00:00:00Z&to=2026-09-18T23:59:59Z", true},
		{"service none", "service=none", true},
	}
	for _, tc := range cases {
		r := httptest.NewRequest("GET", "/?"+tc.query, nil)
		_, err := h.filter(r)
		if tc.ok != (err == nil) {
			t.Errorf("%s: ok=%v want %v (err=%v)", tc.name, err == nil, tc.ok, err)
		}
	}
}
