// Package domain holds read models and request payloads for the admin API.
// Fields map 1:1 to the Authentik v3 serializers (verified against the live
// OpenAPI schema for Authentik 2026.8.2).
package domain

// User is the Authentik core user read model.
type User struct {
	PK          int        `json:"pk"`
	Username    string     `json:"username"`
	Name        string     `json:"name"`
	Email       string     `json:"email"`
	IsActive    bool       `json:"is_active"`
	IsSuperuser bool       `json:"is_superuser"`
	LastLogin   *string    `json:"last_login,omitempty"`
	DateJoined  string     `json:"date_joined"`
	Groups      []string   `json:"groups"`
	GroupsObj   []GroupRef `json:"groups_obj"`
	Roles       []string   `json:"roles"`
}

// GroupRef is a compact group reference embedded in a user's groups_obj.
type GroupRef struct {
	PK   string `json:"pk"`
	Name string `json:"name"`
}

// Group is the Authentik core group read model.
type Group struct {
	PK          string   `json:"pk"`
	Name        string   `json:"name"`
	IsSuperuser bool     `json:"is_superuser"`
	Users       []int    `json:"users"`
	UsersObj    []User   `json:"users_obj"`
	Roles       []string `json:"roles"`
	Parents     []string `json:"parents"`
}

// Role is the Authentik RBAC role read model.
type Role struct {
	PK   string `json:"pk"`
	Name string `json:"name"`
}

// UserRequest is the create/update payload for a core user.
// Pointers mark optional fields; nil means "leave unchanged" on update.
type UserRequest struct {
	Username string   `json:"username"`
	Name     string   `json:"name"`
	Email    string   `json:"email,omitempty"`
	IsActive *bool    `json:"is_active,omitempty"`
	Groups   []string `json:"groups,omitempty"`
	Roles    []string `json:"roles,omitempty"`
}

// GroupRequest is the create/update payload for a core group.
type GroupRequest struct {
	Name        string   `json:"name"`
	IsSuperuser *bool    `json:"is_superuser,omitempty"`
	Users       []int    `json:"users,omitempty"`
	Roles       []string `json:"roles,omitempty"`
	Parents     []string `json:"parents,omitempty"`
}

// RoleRequest is the create/update payload for an RBAC role.
type RoleRequest struct {
	Name string `json:"name"`
}

// SetGroupsRequest replaces the full group membership of a user.
// Groups is NOT omitempty: an empty slice clears all memberships.
type SetGroupsRequest struct {
	Groups []string `json:"groups"`
}

// MemberRequest adds/removes a single user from a group by PK.
type MemberRequest struct {
	PK int `json:"pk"`
}
