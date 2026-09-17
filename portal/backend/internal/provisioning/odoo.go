package provisioning

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// OdooConnector verifies the Odoo provisioning path. Per the approved 1.4a
// decision, Odoo users are NOT pre-created: the account appears automatically
// through the Authentik OIDC provider on first login. The job therefore only
// verifies the JSON-RPC service path (authenticate) and checks whether the
// user already exists. Verification always succeeds the job — absence of the
// user is the expected "deferred" state, not a failure. Endpoint and envelope
// verified live in pre-flight: POST /web/session/authenticate → uid=2.
type OdooConnector struct {
	baseURL string // e.g. https://erp.idchsuite.my.id
	db      string
	login   string
	pass    string
	odb     *pgxpool.Pool // odoo DB on the shared postgres instance (deprovision)
	http    *http.Client
}

// NewOdooConnector returns a connector for the Odoo JSON-RPC endpoint.
// odb may be nil only when de-provisioning is not wired (Sprint 1.4b: SQL
// cleanup runs on the Odoo database, mirroring the verified 1.4a-fix cleanup).
func NewOdooConnector(baseURL, db, login, pass string, odb *pgxpool.Pool) *OdooConnector {
	return &OdooConnector{
		baseURL: strings.TrimRight(baseURL, "/"),
		db:      db,
		login:   login,
		pass:    pass,
		odb:     odb,
		http:    &http.Client{Timeout: 20 * time.Second},
	}
}

// Service implements Connector.
func (c *OdooConnector) Service() string { return "odoo" }

// Provision verifies the service path and the user state. externalID is
// "user-<id>" when the account already exists, "deferred-first-login" when it
// will be created by OIDC on first login.
func (c *OdooConnector) Provision(ctx context.Context, p JobPayload) (string, error) {
	if p.UserEmail == "" {
		return "", fmt.Errorf("odoo: missing user_email in job payload")
	}
	uid, err := c.authenticate(ctx)
	if err != nil {
		return "", err
	}
	if uid == 0 {
		return "", fmt.Errorf("odoo authenticate rejected (uid=0)")
	}

	ids, err := c.search(ctx, uid, p.UserEmail)
	if err != nil {
		return "", err
	}
	if len(ids) > 0 {
		return fmt.Sprintf("user-%d", ids[0]), nil
	}
	return "deferred-first-login", nil
}

// Deprovision removes the Odoo user created by the OIDC auto-create
// (Sprint 1.4b). The account is found by login (email); removal follows the
// dependency order verified in the 1.4a-fix cleanup: group memberships and
// ir_model_data references first, then res_users + res_partner. Idempotent:
// an absent login is a successful no-op. With odb == nil the job still
// succeeds when the user does not exist, and fails when SQL is required but
// unavailable (fail loud, never leave a half-deleted ERP account).
func (c *OdooConnector) Deprovision(ctx context.Context, p JobPayload) error {
	if p.UserEmail == "" {
		return fmt.Errorf("odoo deprovision: missing user_email in job payload")
	}
	uid, err := c.authenticate(ctx)
	if err != nil {
		return err
	}
	if uid == 0 {
		return fmt.Errorf("odoo authenticate rejected (uid=0)")
	}

	ids, err := c.search(ctx, uid, p.UserEmail)
	if err != nil {
		return err
	}
	if len(ids) == 0 {
		return nil // never provisioned / already gone — idempotent success
	}

	if c.odb == nil {
		return fmt.Errorf("odoo deprovision: odoo db pool not configured")
	}
	if err := c.deleteUserSQL(ctx, ids[0]); err != nil {
		return err
	}
	return nil
}

// deleteUserSQL removes one res.users row plus its dependencies in the order
// that satisfies Odoo's foreign keys. res_partner deletion happens after the
// user row that referenced it.
func (c *OdooConnector) deleteUserSQL(ctx context.Context, userID int) error {
	tx, err := c.odb.Begin(ctx)
	if err != nil {
		return fmt.Errorf("odoo deprovision begin: %w", err)
	}
	defer tx.Rollback(ctx)

	partnerID := 0
	err = tx.QueryRow(ctx, `SELECT partner_id FROM res_users WHERE id = $1`, userID).Scan(&partnerID)
	if err != nil {
		return fmt.Errorf("odoo deprovision: load user %d: %w", userID, err)
	}

	stmts := []struct {
		q    string
		args []any
	}{
		{`DELETE FROM res_groups_users_rel WHERE uid = $1`, []any{userID}},
		{`DELETE FROM ir_model_data WHERE model = 'res.users' AND res_id = $1`, []any{userID}},
		{`DELETE FROM ir_model_data WHERE model = 'res.partner' AND res_id = $1`, []any{partnerID}},
		{`DELETE FROM res_users WHERE id = $1`, []any{userID}},
		{`DELETE FROM res_partner WHERE id = $1`, []any{partnerID}},
	}
	for _, s := range stmts {
		if _, err := tx.Exec(ctx, s.q, s.args...); err != nil {
			return fmt.Errorf("odoo deprovision: %s: %w", s.q, err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("odoo deprovision commit: %w", err)
	}
	return nil
}

// rpcError is the JSON-RPC error member of an Odoo response.
type rpcError struct {
	Message string `json:"message"`
	Data    struct {
		Message string `json:"message"`
	} `json:"data"`
}

func (e *rpcError) msg() string {
	if e.Message != "" {
		return e.Message
	}
	return e.Data.Message
}

// authenticate logs the connector in and returns the Odoo user id. The reply
// wraps the session object: {"jsonrpc":"2.0","result":{"uid":2,...}}.
func (c *OdooConnector) authenticate(ctx context.Context) (int, error) {
	params := map[string]any{"db": c.db, "login": c.login, "password": c.pass}
	var res struct {
		Result struct {
			UID int `json:"uid"`
		} `json:"result"`
		Error *rpcError `json:"error"`
	}
	if err := c.call(ctx, "/web/session/authenticate", params, &res); err != nil {
		return 0, err
	}
	if res.Error != nil {
		return 0, fmt.Errorf("odoo auth error: %s", res.Error.msg())
	}
	return res.Result.UID, nil
}

// search runs object.execute_kw res.users search for a login address.
func (c *OdooConnector) search(ctx context.Context, uid int, email string) ([]int, error) {
	domain := []any{[]any{"login", "=", email}}
	params := map[string]any{
		"service": "object",
		"method":  "execute_kw",
		"args":    []any{c.db, uid, c.pass, "res.users", "search", []any{domain}, map[string]any{"limit": 1}},
	}
	var res struct {
		Result []int     `json:"result"`
		Error  *rpcError `json:"error"`
	}
	// The external API endpoint for object calls is /jsonrpc with a
	// params:{service,method,args} envelope (/web/dataset/call_kw answers 404
	// to this shape — verified live 16 Sep 2026).
	if err := c.call(ctx, "/jsonrpc", params, &res); err != nil {
		return nil, err
	}
	if res.Error != nil {
		return nil, fmt.Errorf("odoo search error: %s", res.Error.msg())
	}
	return res.Result, nil
}

// call posts a JSON-RPC envelope and decodes the response into out.
func (c *OdooConnector) call(ctx context.Context, path string, rpcParams map[string]any, out any) error {
	body, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"method":  "call",
		"params":  rpcParams,
	})
	if err != nil {
		return fmt.Errorf("marshal odoo request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build odoo request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("odoo request: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("read odoo response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("odoo http %d: %s", resp.StatusCode, truncate(raw))
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("decode odoo response: %w", err)
	}
	return nil
}
