package handlers

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/jetswu/cloudsuite/portal/backend/internal/domain"
	"github.com/jetswu/cloudsuite/portal/backend/internal/repository/postgres"
	"github.com/jetswu/cloudsuite/portal/backend/internal/service"
)

// DomainHandler serves the /api/admin/domains endpoints for domain onboarding.
// It depends on the DomainService orchestration layer.
type DomainHandler struct {
	svc   *service.DomainService
	audit *service.AuditLogger // nil = audit disabled (1.5a)
}

// NewDomainHandler wires the domain handler to its service. audit may be nil,
// which disables the audit trail.
func NewDomainHandler(svc *service.DomainService, audit *service.AuditLogger) *DomainHandler {
	return &DomainHandler{svc: svc, audit: audit}
}

// Routes mounts the domain endpoints under /api/admin/domains. requireAuth and
// requireSuperAdmin are applied by the caller (they already guard the whole
// /api/admin subtree in main.go).
func (d *DomainHandler) Routes(r chi.Router) {
	r.Route("/api/admin/domains", func(domains chi.Router) {
		domains.Get("/", d.listDomains)
		domains.Post("/", d.createDomain)
		domains.Get("/{id}", d.getDomain)
		domains.Patch("/{id}", d.updateDomain)
		domains.Delete("/{id}", d.deleteDomain)
		domains.Post("/{id}/verify", d.verifyDomain)
		domains.Get("/{id}/records", d.listRecords)
	})
}

func (d *DomainHandler) listDomains(w http.ResponseWriter, r *http.Request) {
	domains, err := d.svc.ListDomains(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list domains", err)
		return
	}
	writeJSON(w, http.StatusOK, domains)
}

func (d *DomainHandler) createDomain(w http.ResponseWriter, r *http.Request) {
	var req domain.CreateDomainRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}

	detail, err := d.svc.CreateDomain(r.Context(), req.Name, req.DKIMMode)
	if err != nil {
		// Distinguish validation errors from upstream failures.
		if isUserError(err) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeError(w, http.StatusBadGateway, "create domain", err)
		return
	}
	writeJSON(w, http.StatusCreated, detail)
	if d.audit != nil {
		e := auditEntryFor(r)
		e.Action, e.TargetType, e.TargetID = "domain.create", "domain", detail.Domain.ID
		e.ServiceCode = "mail"
		e.Metadata = map[string]any{"name": detail.Domain.Name, "dkim_mode": string(req.DKIMMode)}
		_ = d.audit.Log(r.Context(), e)
	}
}

func (d *DomainHandler) getDomain(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	detail, err := d.svc.GetDomain(r.Context(), id)
	if err != nil {
		if errors.Is(err, postgres.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "domain not found"})
			return
		}
		writeError(w, http.StatusInternalServerError, "get domain", err)
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

// updateDomain handles PATCH /api/admin/domains/{id}: switching the DKIM
// mode regenerates the domain's DKIM DNS records (Sprint 1.3b).
func (d *DomainHandler) updateDomain(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req domain.UpdateDomainRequest
	if !decodeBody(w, r, &req) {
		return
	}
	detail, err := d.svc.UpdateDomainDKIMMode(r.Context(), id, req.DKIMMode)
	if err != nil {
		if errors.Is(err, postgres.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "domain not found"})
			return
		}
		if isUserError(err) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeError(w, http.StatusBadGateway, "update domain dkim mode", err)
		return
	}
	writeJSON(w, http.StatusOK, detail)
	if d.audit != nil {
		e := auditEntryFor(r)
		e.Action, e.TargetType, e.TargetID = "domain.update_dkim", "domain", id
		e.ServiceCode = "mail"
		e.Metadata = map[string]any{"dkim_mode": string(req.DKIMMode)}
		_ = d.audit.Log(r.Context(), e)
	}
}

func (d *DomainHandler) deleteDomain(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := d.svc.DeleteDomain(r.Context(), id); err != nil {
		if errors.Is(err, postgres.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "domain not found"})
			return
		}
		writeError(w, http.StatusBadGateway, "delete domain", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
	if d.audit != nil {
		e := auditEntryFor(r)
		e.Action, e.TargetType, e.TargetID = "domain.delete", "domain", id
		_ = d.audit.Log(r.Context(), e)
	}
}

func (d *DomainHandler) verifyDomain(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	detail, err := d.svc.VerifyDomain(r.Context(), id)
	if err != nil {
		if errors.Is(err, postgres.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "domain not found"})
			return
		}
		writeError(w, http.StatusInternalServerError, "verify domain", err)
		return
	}
	writeJSON(w, http.StatusOK, detail)
	if d.audit != nil {
		e := auditEntryFor(r)
		e.Action, e.TargetType, e.TargetID = "domain.verify", "domain", id
		e.ServiceCode = "mail"
		e.Metadata = map[string]any{"status": string(detail.Domain.Status)}
		_ = d.audit.Log(r.Context(), e)
	}
}

func (d *DomainHandler) listRecords(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	detail, err := d.svc.GetDomain(r.Context(), id)
	if err != nil {
		if errors.Is(err, postgres.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "domain not found"})
			return
		}
		writeError(w, http.StatusInternalServerError, "get domain records", err)
		return
	}
	writeJSON(w, http.StatusOK, detail.Records)
}

// isUserError reports whether the error is a client input error (bad domain
// name, duplicate) rather than an upstream failure.
func isUserError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return msg == "invalid domain name" ||
		msg == "no onboarding records generated from zone file" ||
		strings.HasPrefix(msg, "invalid dkim mode") ||
		msg == "domain has no stalwart registration" ||
		strings.Contains(msg, "already exists")
}
