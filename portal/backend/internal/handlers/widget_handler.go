package handlers

// widget_handler.go serves the Sprint 1.5a dashboard widget summaries:
//
//	GET /api/widgets/mail/summary   — unread + 5 recent (Stalwart JMAP)
//	GET /api/widgets/drive/summary  — storage + 5 recent files (Nextcloud)
//	GET /api/widgets/erp/summary    — base-install metrics (Odoo RPC)
//
// Mounted behind RequireAuth by main.go (any authenticated portal user, no
// superadmin gate — the data is the caller's own).
//
// Identity mapping (verified Sprint 1.4 / 1.5a):
//   - Mail: the caller's email claim matches the Stalwart account
//     emailAddress (client-side match; provisioning sets name = local part
//     with server-generated emailAddress).
//   - Drive: the JWT `sub` claim IS the Authentik uid (sha256 hex), which is
//     the Nextcloud userid (oc_user_oidc.sub = uid seeding, 1.4a).
//   - ERP: site-wide metrics from the service account.

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/jetswu/cloudsuite/portal/backend/internal/widget"
)

// WidgetHandler serves the dashboard widget endpoints.
type WidgetHandler struct {
	widget *widget.Service
}

// NewWidgetHandler wires the handler to the widget service.
func NewWidgetHandler(widget *widget.Service) *WidgetHandler {
	return &WidgetHandler{widget: widget}
}

// Routes mounts the widget routes under /api/widgets.
func (h *WidgetHandler) Routes(r chi.Router) {
	r.Route("/api/widgets", func(w chi.Router) {
		w.Get("/mail/summary", h.mailSummary)
		w.Get("/drive/summary", h.driveSummary)
		w.Get("/erp/summary", h.erpSummary)
	})
}

func (h *WidgetHandler) mailSummary(w http.ResponseWriter, r *http.Request) {
	claims, _ := r.Context().Value(claimsKey).(*Claims)
	if claims == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	summary, err := h.widget.MailSummary(r.Context(), claims.Email)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"unavailable": true,
			"error":       err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

func (h *WidgetHandler) driveSummary(w http.ResponseWriter, r *http.Request) {
	claims, _ := r.Context().Value(claimsKey).(*Claims)
	if claims == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	summary, err := h.widget.DriveSummary(r.Context(), claims.Sub)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"unavailable": true,
			"error":       err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

func (h *WidgetHandler) erpSummary(w http.ResponseWriter, r *http.Request) {
	claims, _ := r.Context().Value(claimsKey).(*Claims)
	if claims == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	summary, err := h.widget.ErpSummary(r.Context())
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"unavailable": true,
			"error":       err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, summary)
}
