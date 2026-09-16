package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"

	"github.com/jetswu/cloudsuite/portal/backend/internal/auth"
	"github.com/jetswu/cloudsuite/portal/backend/internal/domain"
	"github.com/jetswu/cloudsuite/portal/backend/internal/provisioning"
	"github.com/jetswu/cloudsuite/portal/backend/internal/repository/authentik"
)

// emailRe is a deliberately simple email shape check: local@domain.tld. It
// matches the frontend validation and exists to catch empty or malformed
// addresses before they reach Authentik and break SSO login later.
var emailRe = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

func validEmail(v string) bool {
	return emailRe.MatchString(strings.TrimSpace(v))
}

// AdminHandler serves the /api/admin CRUD endpoints for users, groups, roles.
// It depends only on the authentik.Repository interface.
type AdminHandler struct {
	repo authentik.Repository
	prov *provisioning.Service // nil = provisioning disabled (1.4a)
}

// NewAdminHandler wires the admin handler to its repository. prov may be nil,
// which disables the provisioning side-effects.
func NewAdminHandler(repo authentik.Repository, prov *provisioning.Service) *AdminHandler {
	return &AdminHandler{repo: repo, prov: prov}
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
	writeJSON(w, http.StatusOK, users)
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
		User:              user,
		Password:          req.Password,
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
	writeJSON(w, http.StatusOK, user)
}

func (a *AdminHandler) deleteUser(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid user id"})
		return
	}
	if err := a.repo.DeleteUser(r.Context(), id); err != nil {
		writeError(w, http.StatusBadGateway, "delete user", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
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
	writeJSON(w, http.StatusOK, group)
}

func (a *AdminHandler) deleteGroup(w http.ResponseWriter, r *http.Request) {
	uuid := chi.URLParam(r, "uuid")
	if err := a.repo.DeleteGroup(r.Context(), uuid); err != nil {
		writeError(w, http.StatusBadGateway, "delete group", err)
		return
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
