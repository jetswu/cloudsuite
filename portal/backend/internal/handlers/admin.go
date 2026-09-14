package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"

	"github.com/jetswu/cloudsuite/portal/backend/internal/auth"
	"github.com/jetswu/cloudsuite/portal/backend/internal/domain"
	"github.com/jetswu/cloudsuite/portal/backend/internal/repository/authentik"
)

// AdminHandler serves the /api/admin CRUD endpoints for users, groups, roles.
// It depends only on the authentik.Repository interface.
type AdminHandler struct {
	repo authentik.Repository
}

// NewAdminHandler wires the admin handler to its repository.
func NewAdminHandler(repo authentik.Repository) *AdminHandler {
	return &AdminHandler{repo: repo}
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

		admin.Get("/groups", a.listGroups)
		admin.Post("/groups", a.createGroup)
		admin.Put("/groups/{uuid}", a.updateGroup)
		admin.Delete("/groups/{uuid}", a.deleteGroup)

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
	var req domain.UserRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if strings.TrimSpace(req.Username) == "" || strings.TrimSpace(req.Name) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "username and name are required"})
		return
	}
	user, err := a.repo.CreateUser(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadGateway, "create user", err)
		return
	}
	writeJSON(w, http.StatusCreated, user)
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
