package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/jetswu/cloudsuite/portal/backend/internal/domain"
)

// fakeRepo is a minimal authentik.Repository stub for createUser validation tests.
type fakeRepo struct {
	created domain.User
}

func (f *fakeRepo) ListUsers(ctx context.Context) ([]domain.User, error)   { return nil, nil }
func (f *fakeRepo) ListGroups(ctx context.Context) ([]domain.Group, error)  { return nil, nil }
func (f *fakeRepo) ListRoles(ctx context.Context) ([]domain.Role, error)    { return nil, nil }
func (f *fakeRepo) CreateUser(ctx context.Context, req domain.UserRequest) (domain.User, error) {
	f.created = domain.User{PK: 99, Username: req.Username, Name: req.Name, Email: req.Email}
	return f.created, nil
}
func (f *fakeRepo) SetUserPassword(ctx context.Context, id int, pw string) error { return nil }
func (f *fakeRepo) CreateGroup(ctx context.Context, req domain.GroupRequest) (domain.Group, error) {
	return domain.Group{}, nil
}
func (f *fakeRepo) CreateRole(ctx context.Context, req domain.RoleRequest) (domain.Role, error) {
	return domain.Role{}, nil
}
func (f *fakeRepo) UpdateUser(ctx context.Context, id int, req domain.UserRequest) (domain.User, error) {
	return domain.User{}, nil
}
func (f *fakeRepo) UpdateGroup(ctx context.Context, uuid string, req domain.GroupRequest) (domain.Group, error) {
	return domain.Group{}, nil
}
func (f *fakeRepo) UpdateRole(ctx context.Context, uuid string, req domain.RoleRequest) (domain.Role, error) {
	return domain.Role{}, nil
}
func (f *fakeRepo) DeleteUser(ctx context.Context, id int) error                  { return nil }
func (f *fakeRepo) DeleteGroup(ctx context.Context, uuid string) error            { return nil }
func (f *fakeRepo) DeleteRole(ctx context.Context, uuid string) error             { return nil }
func (f *fakeRepo) SetUserGroups(ctx context.Context, id int, uuids []string) error {
	return nil
}
func (f *fakeRepo) ListGroupMembers(ctx context.Context, uuid string) ([]domain.User, error) {
	return nil, nil
}
func (f *fakeRepo) AddGroupMember(ctx context.Context, uuid string, pk int) error { return nil }
func (f *fakeRepo) RemoveGroupMember(ctx context.Context, uuid string, pk int) error {
	return nil
}

// routeCreateUser mounts createUser on a chi router (no auth middleware; we test
// the validation logic directly).
func routeCreateUser(repo *fakeRepo) http.Handler {
	a := NewAdminHandler(repo, nil)
	r := chi.NewRouter()
	r.Post("/api/admin/users", a.createUser)
	return r
}

func doCreate(t *testing.T, h http.Handler, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/admin/users", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

func TestCreateUser_RejectsMissingEmail(t *testing.T) {
	repo := &fakeRepo{}
	h := routeCreateUser(repo)
	rr := doCreate(t, h, `{"username":"nouser","name":"No Email","password":"abc12345"}`)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", rr.Code)
	}
	var out map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatalf("bad json: %v", err)
	}
	if out["error"] != "email is required" {
		t.Fatalf("want error 'email is required', got %q", out["error"])
	}
	if repo.created.PK != 0 {
		t.Fatalf("CreateUser must not be called on invalid email, but repo.created.PK=%d", repo.created.PK)
	}
}

func TestCreateUser_RejectsInvalidEmail(t *testing.T) {
	repo := &fakeRepo{}
	h := routeCreateUser(repo)
	rr := doCreate(t, h, `{"username":"nouser","name":"Bad Email","email":"not-an-email","password":"abc12345"}`)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", rr.Code)
	}
	if repo.created.PK != 0 {
		t.Fatalf("CreateUser must not be called on invalid email")
	}
}

func TestCreateUser_AcceptsValidEmail(t *testing.T) {
	repo := &fakeRepo{}
	h := routeCreateUser(repo)
	rr := doCreate(t, h, `{"username":"okuser","name":"Ok User","email":"okuser@idchsuite.my.id","password":"abc12345"}`)
	if rr.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d (body=%s)", rr.Code, rr.Body.String())
	}
	if repo.created.PK != 99 || repo.created.Email != "okuser@idchsuite.my.id" {
		t.Fatalf("CreateUser not invoked with correct payload: %+v", repo.created)
	}
}
