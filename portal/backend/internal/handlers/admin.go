package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/rs/zerolog/log"

	"github.com/jetswu/cloudsuite/portal/backend/internal/auth"
	"github.com/jetswu/cloudsuite/portal/backend/internal/domain"
	"github.com/jetswu/cloudsuite/portal/backend/internal/provisioning"
	"github.com/jetswu/cloudsuite/portal/backend/internal/repository/authentik"
	"github.com/jetswu/cloudsuite/portal/backend/internal/service"
)

// emailRe is a deliberately simple email shape check: local@domain.tld. It
// matches the frontend validation and exists to catch empty or malformed
// addresses before they reach Authentik and break SSO login later.
var emailRe = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

// AdminUserWithProvisioning wraps a user with its Sprint 1.4b provisioning
// read model (nil when the user was never provisioned, e.g. pre-1.4a users).
// Defined in handlers to avoid a domain -> provisioning import cycle.
type AdminUserWithProvisioning struct {
	domain.User
	Provisioning *provisioning.ProvisioningState `json:"provisioning,omitempty"`
}

func validEmail(v string) bool {
	return emailRe.MatchString(strings.TrimSpace(v))
}

// AdminHandler serves the /api/admin CRUD endpoints for users, groups, roles.
// It depends only on the authentik.Repository interface.
type AdminHandler struct {
	repo  authentik.Repository
	prov  *provisioning.Service // nil = provisioning disabled (1.4a)
	audit *service.AuditLogger  // nil = audit disabled (1.5a)
}

// NewAdminHandler wires the admin handler to its repository. prov may be nil,
// which disables the provisioning side-effects; audit may be nil, which
// disables the audit trail.
func NewAdminHandler(repo authentik.Repository, prov *provisioning.Service, audit *service.AuditLogger) *AdminHandler {
	return &AdminHandler{repo: repo, prov: prov, audit: audit}
}

// auditEntryFor builds an AuditEntry from the request context: actor from the
// verified claims, client IP from the reverse-proxy headers, raw UA. Target
// fields are filled by the caller. Shared by all admin handlers (1.5a).
func auditEntryFor(r *http.Request) service.AuditEntry {
	e := service.AuditEntry{ActorType: "user", IPAddress: clientIP(r), UserAgent: r.UserAgent()}
	if claims, ok := r.Context().Value(claimsKey).(*Claims); ok && claims != nil {
		e.ActorEmail = claims.Email
	}
	return e
}

// clientIP prefers the reverse-proxy headers (nginx sets X-Forwarded-For /
// X-Real-IP) and falls back to the TCP peer address.
func clientIP(r *http.Request) string {
	if xf := r.Header.Get("X-Forwarded-For"); xf != "" {
		if i := strings.IndexByte(xf, ','); i >= 0 {
			xf = xf[:i]
		}
		return strings.TrimSpace(xf)
	}
	if xr := r.Header.Get("X-Real-IP"); xr != "" {
		return strings.TrimSpace(xr)
	}
	return r.RemoteAddr
}

// RequireSuperAdmin guards routes that require membership in the super-admin
// group. It must run after RequireAuth (which injects *Claims).
func (a *AdminHandler) RequireSuperAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := r.Context().Value(claimsKey).(*Claims)
		if !ok || claims == nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		if !auth.HasSuperAdmin(claims.Groups) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "not superadmin"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

// Routes mounts the admin subrouter on the given chi.Router. requireAuth must
// be the RequireAuth middleware that injects *Claims into the context; it runs
// before RequireSuperAdmin.
func (a *AdminHandler) Routes(r chi.Router, requireAuth func(http.Handler) http.Handler) {
	r.Route("/api/admin", func(admin chi.Router) {
		admin.Use(requireAuth)
		admin.Use(a.RequireSuperAdmin)

		admin.Get("/users", a.listUsers)
		admin.Post("/users", a.createUser)
		admin.Put("/users/{id}", a.updateUser)
		admin.Delete("/users/{id}", a.deleteUser)
		admin.Put("/users/{id}/groups", a.setUserGroups)
		admin.Post("/users/{id}/retry-provision", a.retryProvision)
		admin.Get("/users/{id}/provisioning", a.userProvisioning)

		admin.Get("/groups", a.listGroups)
		admin.Post("/groups", a.createGroup)
		admin.Put("/groups/{uuid}", a.updateGroup)
		admin.Delete("/groups/{uuid}", a.deleteGroup)
		admin.Get("/groups/{uuid}/members", a.listGroupMembers)
		admin.Post("/groups/{uuid}/members", a.addGroupMember)
		admin.Delete("/groups/{uuid}/members/{pk}", a.removeGroupMember)

		admin.Get("/roles", a.listRoles)
		admin.Post("/roles", a.createRole)
		admin.Put("/roles/{uuid}", a.updateRole)
		admin.Delete("/roles/{uuid}", a.deleteRole)
	})
}

func (a *AdminHandler) listUsers(w http.ResponseWriter, r *http.Request) {
	users, err := a.repo.ListUsers(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, "list users", err)
		return
	}

	// Sprint 1.4b: merge per-user provisioning states into the list payload.
	// Users without a provisioning row keep provisioning == nil (pre-1.4a).
	states := map[int]provisioning.ProvisioningState{}
	if a.prov != nil {
		if s, err := a.prov.ProvisioningStates(r.Context()); err == nil {
			states = s
		} else {
			log.Error().Err(err).Msg("read provisioning states for users list")
		}
	}
	out := make([]AdminUserWithProvisioning, 0, len(users))
	for _, u := range users {
		item := AdminUserWithProvisioning{User: u}
		if st, ok := states[u.PK]; ok {
			item.Provisioning = &st
		}
		out = append(out, item)
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *AdminHandler) createUser(w http.ResponseWriter, r *http.Request) {
	var req domain.CreateUserRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if strings.TrimSpace(req.Username) == "" || strings.TrimSpace(req.Name) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "username and name are required"})
		return
	}
	if !validEmail(req.Email) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "email is required"})
		return
	}
	if err := validatePassword(req.Password); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	// Provisioning precheck (1.4a): reject invalid addresses and unregistered
	// mail domains before the Authentik user exists. Provisioning itself is
	// asynchronous and happens in the worker.
	if a.prov != nil {
		if err := a.prov.Precheck(r.Context(), req.Email); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
	}

	user, err := a.repo.CreateUser(r.Context(), req.UserRequest)
	if err != nil {
		writeError(w, http.StatusBadGateway, "create user", err)
		return
	}
	if err := a.repo.SetUserPassword(r.Context(), user.PK, req.Password); err != nil {
		writeError(w, http.StatusBadGateway, "set user password", err)
		return
	}

	// Sprint 1.5a: audit trail — user.create. Audit failures never fail the
	// request (Log is best-effort by contract).
	if a.audit != nil {
		e := auditEntryFor(r)
		e.Action, e.TargetType, e.TargetID = "user.create", "user", strconv.Itoa(user.PK)
		e.ServiceCode = "authentik"
		e.Metadata = map[string]any{"username": user.Username, "email": user.Email}
		_ = a.audit.Log(r.Context(), e)
	}

	// Trigger provisioning (1.4a): enqueue stalwart/nextcloud/odoo jobs. A
	// failure here must not fail the request — the jobs/state persist in the
	// portal DB and the worker sweep re-enqueues lost messages.
	provisioningQueued := false
	if a.prov != nil {
		if _, err := a.prov.ProvisionUser(r.Context(), user.PK, user.Email); err != nil {
			log.Error().Err(err).Int("user_id", user.PK).Msg("provisioning enqueue failed (user created; worker will retry)")
		} else {
			provisioningQueued = true
		}
	}

	writeJSON(w, http.StatusCreated, domain.CreateUserResponse{
		User:               user,
		Password:           req.Password,
		ProvisioningQueued: provisioningQueued,
	})
}

// validatePassword enforces the minimum password policy for a newly created
// user: at least 8 characters with at least one letter and one digit.
func validatePassword(pw string) error {
	if len(pw) < 8 {
		return errors.New("password must be at least 8 characters")
	}
	var hasLetter, hasDigit bool
	for _, r := range pw {
		switch {
		case r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z':
			hasLetter = true
		case r >= '0' && r <= '9':
			hasDigit = true
		}
		if hasLetter && hasDigit {
			return nil
		}
	}
	if !hasLetter {
		return errors.New("password must contain at least one letter")
	}
	return errors.New("password must contain at least one digit")
}

func (a *AdminHandler) updateUser(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid user id"})
		return
	}
	var req domain.UserRequest
	if !decodeBody(w, r, &req) {
		return
	}
	user, err := a.repo.UpdateUser(r.Context(), id, req)
	if err != nil {
		writeError(w, http.StatusBadGateway, "update user", err)
		return
	}
	if a.audit != nil {
		e := auditEntryFor(r)
		e.Action, e.TargetType, e.TargetID = "user.update", "user", strconv.Itoa(id)
		e.ServiceCode = "authentik"
		e.Metadata = map[string]any{"name": req.Name, "email": req.Email}
		_ = a.audit.Log(r.Context(), e)
	}
	writeJSON(w, http.StatusOK, user)
}

func (a *AdminHandler) deleteUser(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid user id"})
		return
	}

	// Sprint 1.4b: enqueue the three deprovision jobs BEFORE the Authentik
	// delete. The jobs carry the email; the worker pulls uid + service state
	// from user_provisioning. Enqueue failure must not block the delete —
	// the state rows persist and the retry endpoint can re-enqueue.
	deprovisionQueued := false
	if a.prov != nil {
		// Read the provisioning state first: it survives the Authentik delete
		// and feeds both the jobs and the status flip below.
		email, err := a.prov.UserEmail(r.Context(), id)
		if err == nil && email != "" {
			if _, err := a.prov.DeprovisionUser(r.Context(), id, email); err != nil {
				log.Error().Err(err).Int("user_id", id).Msg("deprovision enqueue failed (deleting user; retry endpoint can re-enqueue)")
			} else {
				deprovisionQueued = true
			}
			// Claim the user for deprovisioning: pending_delete stops any
			// still-queued provision job from re-creating resources.
			if err := a.prov.MarkPendingDelete(r.Context(), id, email); err != nil {
				log.Error().Err(err).Int("user_id", id).Msg("mark pending_delete failed")
			}
		} else {
			// Pre-1.4a user (no provisioning row): nothing to deprovision by
			// email. Doc 1.4b 1.2 says enqueue anyway — but without any email
			// the connectors cannot address the account, so the jobs would be
			// dead letters. Keep it visible instead of silent.
			log.Info().Int("user_id", id).Msg("no user_provisioning row; skipping deprovision enqueue")
		}
	}

	if err := a.repo.DeleteUser(r.Context(), id); err != nil {
		writeError(w, http.StatusBadGateway, "delete user", err)
		return
	}
	if a.audit != nil {
		e := auditEntryFor(r)
		e.Action, e.TargetType, e.TargetID = "user.delete", "user", strconv.Itoa(id)
		e.Metadata = map[string]any{"deprovision_queued": deprovisionQueued}
		_ = a.audit.Log(r.Context(), e)
	}
	writeJSON(w, http.StatusOK, map[string]bool{
		"deleted":            true,
		"deprovision_queued": deprovisionQueued,
	})
}

// retryProvision re-enqueues failed provisioning jobs (Sprint 1.4b 1.3):
// POST /api/admin/users/{id}/retry-provision {service: stalwart|nextcloud|odoo|all}
func (a *AdminHandler) retryProvision(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid user id"})
		return
	}
	var req struct {
		Service string `json:"service"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	svc := strings.TrimSpace(req.Service)
	switch svc {
	case "stalwart", "nextcloud", "odoo", "all":
	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "service must be stalwart|nextcloud|odoo|all"})
		return
	}

	// The user may already be gone from Authentik (deleted user retry) —
	// identity then comes from the user_provisioning row.
	email, err := a.prov.UserEmail(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "no provisioning state for this user"})
			return
		}
		writeError(w, http.StatusBadGateway, "retry-provision: read state", err)
		return
	}

	// Deprovisioning users may not be re-provisioned (guard in the worker as
	// well — this check keeps the API answer honest instead of silently
	// enqueueing jobs that will be skipped).
	states, err := a.prov.ServiceStates(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusBadGateway, "retry-provision: read statuses", err)
		return
	}
	for _, s := range states {
		if provisioning.DeprovisioningStates()[s] {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "user is being deprovisioned; provision retry is not allowed"})
			return
		}
	}

	jobs, err := a.prov.LatestJobs(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusBadGateway, "retry-provision: read jobs", err)
		return
	}
	latest := map[string]provisioning.JobRow{}
	for _, j := range jobs {
		latest[j.Service] = j
	}

	targets := provisioning.Services
	if svc != "all" {
		targets = []string{svc}
	}

	var queued []string
	for _, t := range targets {
		j, ok := latest[t]
		if ok && (j.Status == provisioning.StatusQueued || j.Status == provisioning.StatusRunning) {
			continue // already in flight — retry would duplicate work
		}
		// Enqueue a fresh job for the service (both provision and deprovision
		// retries re-run the latest action; a missing job defaults to
		// provision for live users).
		action := provisioning.ActionProvision
		if ok && j.Action == provisioning.ActionDeprovision {
			action = provisioning.ActionDeprovision
		}
		jobIDs, err := a.prov.RetryService(r.Context(), id, email, action, t)
		if err != nil {
			log.Error().Err(err).Int("user_id", id).Str("service", t).Msg("retry-provision enqueue failed")
			continue
		}
		queued = append(queued, jobIDs...)
	}
	if len(queued) == 0 {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "nothing to retry (jobs already queued/running or no failed job)"})
		return
	}
	if a.audit != nil {
		e := auditEntryFor(r)
		e.Action, e.TargetType, e.TargetID = "provisioning.retry", "user", strconv.Itoa(id)
		e.Metadata = map[string]any{"service": svc, "jobs": queued}
		_ = a.audit.Log(r.Context(), e)
	}
	writeJSON(w, http.StatusAccepted, map[string]any{
		"queued":  queued,
		"service": svc,
	})
}

// userProvisioning returns the per-service provisioning state for the user
// detail view: GET /api/admin/users/{id}/provisioning
func (a *AdminHandler) userProvisioning(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid user id"})
		return
	}
	state, err := a.prov.ProvisioningState(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "no provisioning state for this user"})
			return
		}
		writeError(w, http.StatusBadGateway, "read provisioning state", err)
		return
	}
	writeJSON(w, http.StatusOK, state)
}

func (a *AdminHandler) setUserGroups(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid user id"})
		return
	}
	var req domain.SetGroupsRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if req.Groups == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "groups is required (send [] to clear)"})
		return
	}
	if err := a.repo.SetUserGroups(r.Context(), id, req.Groups); err != nil {
		writeError(w, http.StatusBadGateway, "set user groups", err)
		return
	}
	if a.audit != nil {
		e := auditEntryFor(r)
		e.Action, e.TargetType, e.TargetID = "user.set_groups", "user", strconv.Itoa(id)
		e.Metadata = map[string]any{"groups": req.Groups}
		_ = a.audit.Log(r.Context(), e)
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *AdminHandler) listGroupMembers(w http.ResponseWriter, r *http.Request) {
	uuid := chi.URLParam(r, "uuid")
	members, err := a.repo.ListGroupMembers(r.Context(), uuid)
	if err != nil {
		writeError(w, http.StatusBadGateway, "list group members", err)
		return
	}
	writeJSON(w, http.StatusOK, members)
}

func (a *AdminHandler) addGroupMember(w http.ResponseWriter, r *http.Request) {
	uuid := chi.URLParam(r, "uuid")
	var req domain.MemberRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if req.PK <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "pk must be a positive integer"})
		return
	}
	if err := a.repo.AddGroupMember(r.Context(), uuid, req.PK); err != nil {
		writeError(w, http.StatusBadGateway, "add group member", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *AdminHandler) removeGroupMember(w http.ResponseWriter, r *http.Request) {
	uuid := chi.URLParam(r, "uuid")
	pk, err := strconv.Atoi(chi.URLParam(r, "pk"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid user pk"})
		return
	}
	if pk <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "pk must be a positive integer"})
		return
	}
	if err := a.repo.RemoveGroupMember(r.Context(), uuid, pk); err != nil {
		writeError(w, http.StatusBadGateway, "remove group member", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *AdminHandler) listGroups(w http.ResponseWriter, r *http.Request) {
	groups, err := a.repo.ListGroups(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, "list groups", err)
		return
	}
	writeJSON(w, http.StatusOK, groups)
}

func (a *AdminHandler) createGroup(w http.ResponseWriter, r *http.Request) {
	var req domain.GroupRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}
	group, err := a.repo.CreateGroup(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadGateway, "create group", err)
		return
	}
	if a.audit != nil {
		e := auditEntryFor(r)
		e.Action, e.TargetType, e.TargetID = "group.create", "group", group.PK
		e.Metadata = map[string]any{"name": group.Name}
		_ = a.audit.Log(r.Context(), e)
	}
	writeJSON(w, http.StatusCreated, group)
}

func (a *AdminHandler) updateGroup(w http.ResponseWriter, r *http.Request) {
	uuid := chi.URLParam(r, "uuid")
	var req domain.GroupRequest
	if !decodeBody(w, r, &req) {
		return
	}
	group, err := a.repo.UpdateGroup(r.Context(), uuid, req)
	if err != nil {
		writeError(w, http.StatusBadGateway, "update group", err)
		return
	}
	if a.audit != nil {
		e := auditEntryFor(r)
		e.Action, e.TargetType, e.TargetID = "group.update", "group", uuid
		e.Metadata = map[string]any{"name": req.Name}
		_ = a.audit.Log(r.Context(), e)
	}
	writeJSON(w, http.StatusOK, group)
}

func (a *AdminHandler) deleteGroup(w http.ResponseWriter, r *http.Request) {
	uuid := chi.URLParam(r, "uuid")
	if err := a.repo.DeleteGroup(r.Context(), uuid); err != nil {
		writeError(w, http.StatusBadGateway, "delete group", err)
		return
	}
	if a.audit != nil {
		e := auditEntryFor(r)
		e.Action, e.TargetType, e.TargetID = "group.delete", "group", uuid
		_ = a.audit.Log(r.Context(), e)
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *AdminHandler) listRoles(w http.ResponseWriter, r *http.Request) {
	roles, err := a.repo.ListRoles(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, "list roles", err)
		return
	}
	writeJSON(w, http.StatusOK, roles)
}

func (a *AdminHandler) createRole(w http.ResponseWriter, r *http.Request) {
	var req domain.RoleRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}
	role, err := a.repo.CreateRole(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadGateway, "create role", err)
		return
	}
	writeJSON(w, http.StatusCreated, role)
}

func (a *AdminHandler) updateRole(w http.ResponseWriter, r *http.Request) {
	uuid := chi.URLParam(r, "uuid")
	var req domain.RoleRequest
	if !decodeBody(w, r, &req) {
		return
	}
	role, err := a.repo.UpdateRole(r.Context(), uuid, req)
	if err != nil {
		writeError(w, http.StatusBadGateway, "update role", err)
		return
	}
	writeJSON(w, http.StatusOK, role)
}

func (a *AdminHandler) deleteRole(w http.ResponseWriter, r *http.Request) {
	uuid := chi.URLParam(r, "uuid")
	if err := a.repo.DeleteRole(r.Context(), uuid); err != nil {
		writeError(w, http.StatusBadGateway, "delete role", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// decodeBody decodes a JSON request body, capping at 1 MiB, and writes a 400
// on malformed input.
func decodeBody(w http.ResponseWriter, r *http.Request, v any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	if err := dec.Decode(v); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json body"})
		return false
	}
	return true
}

// writeError logs and writes a JSON error response without leaking internals.
func writeError(w http.ResponseWriter, status int, op string, err error) {
	log.Error().Err(err).Str("op", op).Msg("admin request failed")
	// Never expose upstream detail to the client.
	writeJSON(w, status, map[string]string{"error": "upstream Authentik request failed"})
}
