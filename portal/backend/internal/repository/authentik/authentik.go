// Package authentik implements the Authentik admin repository: a thin HTTP
// client over the Authentik v3 API for CRUD on users, groups, and roles.
package authentik

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jetswu/cloudsuite/portal/backend/internal/domain"
)

const (
	pathUsers  = "/api/v3/core/users/"
	pathGroups = "/api/v3/core/groups/"
	pathRoles  = "/api/v3/rbac/roles/"
)

// Repository defines the admin operations the handlers depend on. The
// AuthentikClient is the concrete implementation; tests can substitute a fake.
type Repository interface {
	ListUsers(ctx context.Context) ([]domain.User, error)
	ListGroups(ctx context.Context) ([]domain.Group, error)
	ListRoles(ctx context.Context) ([]domain.Role, error)
	CreateUser(ctx context.Context, req domain.UserRequest) (domain.User, error)
	SetUserPassword(ctx context.Context, id int, password string) error
	CreateGroup(ctx context.Context, req domain.GroupRequest) (domain.Group, error)
	CreateRole(ctx context.Context, req domain.RoleRequest) (domain.Role, error)
	UpdateUser(ctx context.Context, id int, req domain.UserRequest) (domain.User, error)
	UpdateGroup(ctx context.Context, uuid string, req domain.GroupRequest) (domain.Group, error)
	UpdateRole(ctx context.Context, uuid string, req domain.RoleRequest) (domain.Role, error)
	DeleteUser(ctx context.Context, id int) error
	DeleteGroup(ctx context.Context, uuid string) error
	DeleteRole(ctx context.Context, uuid string) error
	SetUserGroups(ctx context.Context, id int, groupUUIDs []string) error
	ListGroupMembers(ctx context.Context, uuid string) ([]domain.User, error)
	AddGroupMember(ctx context.Context, uuid string, userPK int) error
	RemoveGroupMember(ctx context.Context, uuid string, userPK int) error
}

// Client is the Authentik API client. It is safe for concurrent use.
type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

// NewClient returns a client bound to baseURL (e.g. http://authentik-server:9000)
// authenticated with token.
func NewClient(baseURL, token string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		http:    &http.Client{Timeout: 15 * time.Second},
	}
}

// paginated wraps the Authentik paginated list envelope.
type paginated[T any] struct {
	Results []T `json:"results"`
}

func (c *Client) ListUsers(ctx context.Context) ([]domain.User, error) {
	var out paginated[domain.User]
	if err := c.do(ctx, http.MethodGet, pathUsers, nil, &out); err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	return normalizeUsers(out.Results), nil
}

// GetUser fetches a single user by primary key. The uid field (sha256 hex
// subject) is what Nextcloud/Odoo use as the SSO identity.
func (c *Client) GetUser(ctx context.Context, id int) (domain.User, error) {
	var out domain.User
	if err := c.do(ctx, http.MethodGet, pathUsers+strconv.Itoa(id)+"/", nil, &out); err != nil {
		return domain.User{}, fmt.Errorf("get user %d: %w", id, err)
	}
	normalizeUser(&out)
	return out, nil
}

func (c *Client) ListGroups(ctx context.Context) ([]domain.Group, error) {
	var out paginated[domain.Group]
	if err := c.do(ctx, http.MethodGet, pathGroups, nil, &out); err != nil {
		return nil, fmt.Errorf("list groups: %w", err)
	}
	return normalizeGroups(out.Results), nil
}

func (c *Client) ListRoles(ctx context.Context) ([]domain.Role, error) {
	var out paginated[domain.Role]
	if err := c.do(ctx, http.MethodGet, pathRoles, nil, &out); err != nil {
		return nil, fmt.Errorf("list roles: %w", err)
	}
	return out.Results, nil
}

func (c *Client) CreateUser(ctx context.Context, req domain.UserRequest) (domain.User, error) {
	var out domain.User
	if err := c.do(ctx, http.MethodPost, pathUsers, req, &out); err != nil {
		return out, fmt.Errorf("create user: %w", err)
	}
	normalizeUser(&out)
	return out, nil
}

// SetUserPassword sets a user's initial password via the dedicated Authentik
// set_password endpoint. Authentik does not accept a password on the create
// payload, so this must be called after CreateUser for a new account to be
// able to log in.
func (c *Client) SetUserPassword(ctx context.Context, id int, password string) error {
	path := pathUsers + strconv.Itoa(id) + "/set_password/"
	body := struct {
		Password string `json:"password"`
	}{Password: password}
	if err := c.do(ctx, http.MethodPost, path, body, nil); err != nil {
		return fmt.Errorf("set user %d password: %w", id, err)
	}
	return nil
}

func (c *Client) CreateGroup(ctx context.Context, req domain.GroupRequest) (domain.Group, error) {
	var out domain.Group
	if err := c.do(ctx, http.MethodPost, pathGroups, req, &out); err != nil {
		return out, fmt.Errorf("create group: %w", err)
	}
	normalizeGroup(&out)
	return out, nil
}

func (c *Client) CreateRole(ctx context.Context, req domain.RoleRequest) (domain.Role, error) {
	var out domain.Role
	if err := c.do(ctx, http.MethodPost, pathRoles, req, &out); err != nil {
		return out, fmt.Errorf("create role: %w", err)
	}
	return out, nil
}

func (c *Client) UpdateUser(ctx context.Context, id int, req domain.UserRequest) (domain.User, error) {
	var out domain.User
	path := pathUsers + strconv.Itoa(id) + "/"
	if err := c.do(ctx, http.MethodPut, path, req, &out); err != nil {
		return out, fmt.Errorf("update user %d: %w", id, err)
	}
	normalizeUser(&out)
	return out, nil
}

func (c *Client) UpdateGroup(ctx context.Context, uuid string, req domain.GroupRequest) (domain.Group, error) {
	var out domain.Group
	path := pathGroups + uuid + "/"
	if err := c.do(ctx, http.MethodPut, path, req, &out); err != nil {
		return out, fmt.Errorf("update group %s: %w", uuid, err)
	}
	normalizeGroup(&out)
	return out, nil
}

func (c *Client) UpdateRole(ctx context.Context, uuid string, req domain.RoleRequest) (domain.Role, error) {
	var out domain.Role
	path := pathRoles + uuid + "/"
	if err := c.do(ctx, http.MethodPut, path, req, &out); err != nil {
		return out, fmt.Errorf("update role %s: %w", uuid, err)
	}
	return out, nil
}

func (c *Client) DeleteUser(ctx context.Context, id int) error {
	path := pathUsers + strconv.Itoa(id) + "/"
	if err := c.do(ctx, http.MethodDelete, path, nil, nil); err != nil {
		return fmt.Errorf("delete user %d: %w", id, err)
	}
	return nil
}

func (c *Client) DeleteGroup(ctx context.Context, uuid string) error {
	path := pathGroups + uuid + "/"
	if err := c.do(ctx, http.MethodDelete, path, nil, nil); err != nil {
		return fmt.Errorf("delete group %s: %w", uuid, err)
	}
	return nil
}

func (c *Client) DeleteRole(ctx context.Context, uuid string) error {
	path := pathRoles + uuid + "/"
	if err := c.do(ctx, http.MethodDelete, path, nil, nil); err != nil {
		return fmt.Errorf("delete role %s: %w", uuid, err)
	}
	return nil
}

// SetUserGroups replaces the full group membership of a user. An empty slice
// clears all memberships (Authentik treats a present "groups" field as the
// complete desired set).
func (c *Client) SetUserGroups(ctx context.Context, id int, groupUUIDs []string) error {
	path := pathUsers + strconv.Itoa(id) + "/"
	body := domain.SetGroupsRequest{Groups: groupUUIDs}
	if err := c.do(ctx, http.MethodPatch, path, body, nil); err != nil {
		return fmt.Errorf("set user %d groups: %w", id, err)
	}
	return nil
}

// ListGroupMembers returns the users that belong to a group, decoded from the
// group serializer's users_obj field.
func (c *Client) ListGroupMembers(ctx context.Context, uuid string) ([]domain.User, error) {
	var group domain.Group
	path := pathGroups + uuid + "/"
	if err := c.do(ctx, http.MethodGet, path, nil, &group); err != nil {
		return nil, fmt.Errorf("list group %s members: %w", uuid, err)
	}
	return normalizeUsers(group.UsersObj), nil
}

func (c *Client) AddGroupMember(ctx context.Context, uuid string, userPK int) error {
	path := pathGroups + uuid + "/add_user/"
	body := domain.MemberRequest{PK: userPK}
	if err := c.do(ctx, http.MethodPost, path, body, nil); err != nil {
		return fmt.Errorf("add user %d to group %s: %w", userPK, uuid, err)
	}
	return nil
}

func (c *Client) RemoveGroupMember(ctx context.Context, uuid string, userPK int) error {
	path := pathGroups + uuid + "/remove_user/"
	body := domain.MemberRequest{PK: userPK}
	if err := c.do(ctx, http.MethodPost, path, body, nil); err != nil {
		return fmt.Errorf("remove user %d from group %s: %w", userPK, uuid, err)
	}
	return nil
}

// normalizeUser ensures every slice field on the read model is a non-nil
// empty slice. Authentik serializes unpopulated nested relations as JSON null,
// which Go decodes to a nil slice and re-marshals to null; the frontend
// expects arrays, so we normalize to [] for consistent output.
func normalizeUser(u *domain.User) {
	if u.Groups == nil {
		u.Groups = []string{}
	}
	if u.GroupsObj == nil {
		u.GroupsObj = []domain.GroupRef{}
	}
	if u.Roles == nil {
		u.Roles = []string{}
	}
}

func normalizeUsers(users []domain.User) []domain.User {
	if users == nil {
		users = []domain.User{}
	}
	for i := range users {
		normalizeUser(&users[i])
	}
	return users
}

func normalizeGroup(g *domain.Group) {
	if g.Users == nil {
		g.Users = []int{}
	}
	if g.UsersObj == nil {
		g.UsersObj = []domain.User{}
	}
	if g.Roles == nil {
		g.Roles = []string{}
	}
	if g.Parents == nil {
		g.Parents = []string{}
	}
	for i := range g.UsersObj {
		normalizeUser(&g.UsersObj[i])
	}
}

func normalizeGroups(groups []domain.Group) []domain.Group {
	if groups == nil {
		groups = []domain.Group{}
	}
	for i := range groups {
		normalizeGroup(&groups[i])
	}
	return groups
}

// apiError is the Authentik error envelope.
type apiError struct {
	Detail string `json:"detail"`
}

// do performs a JSON request and decodes the response into out (may be nil).
func (c *Client) do(ctx context.Context, method, path string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal body: %w", err)
		}
		reader = bytes.NewReader(buf)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		var ae apiError
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
		if err := json.Unmarshal(raw, &ae); err != nil || ae.Detail == "" {
			return fmt.Errorf("authentik %s: %s", resp.Status, strings.TrimSpace(string(raw)))
		}
		return fmt.Errorf("authentik %s: %s", resp.Status, ae.Detail)
	}

	if out == nil || resp.StatusCode == http.StatusNoContent {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}
