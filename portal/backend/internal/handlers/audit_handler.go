package handlers

// audit_handler.go serves the Sprint 1.5a audit endpoints:
//
//	GET /api/admin/audit          — filtered + paginated list
//	GET /api/admin/audit/export   — same filters as CSV (attachment)
//
// Both are mounted behind RequireAuth + RequireSuperAdmin by main.go, same
// guard as the rest of /api/admin.

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/jetswu/cloudsuite/portal/backend/internal/repository/postgres"
)

// AuditHandler serves the audit list and CSV export.
type AuditHandler struct {
	repo *postgres.AuditRepo
}

// NewAuditHandler wires the handler to the audit read repo.
func NewAuditHandler(repo *postgres.AuditRepo) *AuditHandler {
	return &AuditHandler{repo: repo}
}

// Routes mounts the audit endpoints under /api/admin/audit.
func (h *AuditHandler) Routes(r chi.Router) {
	r.Route("/api/admin/audit", func(audit chi.Router) {
		audit.Get("/", h.list)
		audit.Get("/export", h.exportCSV)
	})
}

// filter parses the shared query params. Errors are user-facing (400).
func (h *AuditHandler) filter(r *http.Request) (postgres.AuditFilter, error) {
	q := r.URL.Query()
	f := postgres.AuditFilter{
		Action:  q.Get("action"),
		Service: q.Get("service"),
		Search:  strings.TrimSpace(q.Get("search")),
		Limit:   50,
	}
	if v := q.Get("actor_id"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			return f, fmt.Errorf("actor_id must be a non-negative integer")
		}
		f.ActorID = n
	}
	for _, bound := range []struct {
		key string
		dst *time.Time
	}{
		{"from", &f.From},
		{"to", &f.To},
	} {
		v := q.Get(bound.key)
		if v == "" {
			continue
		}
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			*bound.dst = t
			continue
		}
		if t, err := time.Parse("2006-01-02", v); err == nil {
			// A bare date for "to" is inclusive: cover the whole day.
			if bound.key == "to" {
				t = t.AddDate(0, 0, 1)
			}
			*bound.dst = t
			continue
		}
		return f, fmt.Errorf("%s must be ISO 8601 or YYYY-MM-DD", bound.key)
	}
	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 500 {
			return f, fmt.Errorf("limit must be 1..500")
		}
		f.Limit = n
	}
	if v := q.Get("page"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			return f, fmt.Errorf("page must be >= 1")
		}
		f.Offset = (n - 1) * f.Limit
	}
	return f, nil
}

func (h *AuditHandler) list(w http.ResponseWriter, r *http.Request) {
	f, err := h.filter(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	items, total, err := h.repo.List(r.Context(), f)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list audit", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"total": total,
		"page":  f.Offset/f.Limit + 1,
		"limit": f.Limit,
		"items": items,
	})
}

// exportCSV streams the filtered log as a CSV attachment. The default cap is
// raised for exports so a browser download cannot drag the whole table into
// memory, while the response stays bounded.
func (h *AuditHandler) exportCSV(w http.ResponseWriter, r *http.Request) {
	f, err := h.filter(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if f.Limit == 50 && qAbsents(r, "limit") {
		f.Limit = 10000
	}
	f.Offset = 0

	items, _, err := h.repo.List(r.Context(), f)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "export audit", err)
		return
	}

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="audit-log.csv"`)

	cw := csv.NewWriter(w)
	_ = cw.Write([]string{"id", "created_at", "actor_email", "actor_type", "action",
		"target_type", "target_id", "service_code", "ip_address", "metadata"})
	for _, a := range items {
		meta, _ := json.Marshal(a.Metadata)
		_ = cw.Write([]string{
			strconv.FormatInt(a.ID, 10),
			a.CreatedAt.UTC().Format(time.RFC3339),
			a.ActorEmail,
			a.ActorType,
			a.Action,
			derefStr(a.TargetType),
			derefStr(a.TargetID),
			derefStr(a.ServiceCode),
			derefStr(a.IPAddress),
			string(meta),
		})
	}
	cw.Flush()
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func qAbsents(r *http.Request, key string) bool {
	_, ok := r.URL.Query()[key]
	return !ok
}
