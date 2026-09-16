package provisioning

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// NextcloudConnector provisions Nextcloud users via the OCS API and seeds the
// user_oidc mapping so the SSO uid (Authentik uid = sha256 hex, verified in
// Sprint 1.4 pre-flight) maps to the provisioned account. Sequence verified
// live in pre-flight: POST /ocs/v1.php/cloud/users {userid,password} ->
// statuscode 100; pre-insert oc_user_oidc (user_id, display_name,
// provider_id=1, sub=<uid>) makes the first login land on this account.
type NextcloudConnector struct {
	baseURL string // e.g. http://nextcloud (docker service name)
	user    string // admin account for the OCS calls
	pass    string
	ncdb    *pgxpool.Pool // nextcloud DB on the shared postgres instance
	http    *http.Client
}

// NewNextcloudConnector returns a connector for the Nextcloud OCS endpoint.
// ncdb may be nil only when the oidc mapping is managed elsewhere.
func NewNextcloudConnector(baseURL, user, pass string, ncdb *pgxpool.Pool) *NextcloudConnector {
	return &NextcloudConnector{
		baseURL: strings.TrimRight(baseURL, "/"),
		user:    user,
		pass:    pass,
		ncdb:    ncdb,
		http:    &http.Client{Timeout: 15 * time.Second},
	}
}

// Service implements Connector.
func (c *NextcloudConnector) Service() string { return "nextcloud" }

// Provision creates the Nextcloud user (idempotent) and seeds the user_oidc
// row. externalID = the Nextcloud userid (= Authentik uid).
func (c *NextcloudConnector) Provision(ctx context.Context, p JobPayload) (string, error) {
	if p.UID == "" {
		return "", fmt.Errorf("nextcloud: missing uid in job payload")
	}
	exists, err := c.userExists(ctx, p.UID)
	if err != nil {
		return "", err
	}
	if !exists {
		// Password is a throwaway: accounts authenticate via SSO only
		// (1.4a decision). It is never reported or stored.
		pw, err := RandomPassword(24)
		if err != nil {
			return "", fmt.Errorf("nextcloud: generate password: %w", err)
		}
		form := url.Values{}
		form.Set("userid", p.UID)
		form.Set("password", pw)
		body, err := c.ocs(ctx, http.MethodPost, "/cloud/users", form)
		if err != nil {
			return "", err
		}
		switch ocsCode(body) {
		case "100":
			// created
		case "102":
			// already exists (raced) — idempotent success
		default:
			return "", fmt.Errorf("nextcloud create: %s", ocsMessage(body))
		}
	}
	if err := c.seedOIDCMapping(ctx, p); err != nil {
		return "", err
	}
	return p.UID, nil
}

// userExists queries the account by userid via OCS GET (idempotency check).
func (c *NextcloudConnector) userExists(ctx context.Context, uid string) (bool, error) {
	body, err := c.ocs(ctx, http.MethodGet, "/cloud/users/"+url.PathEscape(uid), nil)
	if err != nil {
		return false, err
	}
	switch ocsCode(body) {
	case "100":
		return true, nil
	case "404":
		return false, nil
	default:
		return false, fmt.Errorf("nextcloud get user: %s", ocsMessage(body))
	}
}

// seedOIDCMapping inserts the user_oidc row (provider 1, sub = uid) so SSO
// resolves the provisioned account. Existing rows are left untouched.
func (c *NextcloudConnector) seedOIDCMapping(ctx context.Context, p JobPayload) error {
	if c.ncdb == nil {
		return nil
	}
	_, err := c.ncdb.Exec(ctx, `INSERT INTO oc_user_oidc (user_id, display_name, provider_id, sub)
		VALUES ($1, $2, 1, $3) ON CONFLICT DO NOTHING`, p.UID, p.Name, p.UID)
	if err != nil {
		return fmt.Errorf("nextcloud: seed oc_user_oidc: %w", err)
	}
	return nil
}

// ocs performs an OCS API request and returns the raw XML body.
func (c *NextcloudConnector) ocs(ctx context.Context, method, path string, form url.Values) ([]byte, error) {
	var bodyReader io.Reader
	if form != nil {
		bodyReader = strings.NewReader(form.Encode())
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+"/ocs/v1.php"+path, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("build ocs request: %w", err)
	}
	req.SetBasicAuth(c.user, c.pass)
	req.Header.Set("OCS-APIRequest", "true")
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("nextcloud ocs request: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read nextcloud response: %w", err)
	}
	if resp.StatusCode >= 500 {
		return nil, fmt.Errorf("nextcloud ocs http %d: %s", resp.StatusCode, truncate(raw))
	}
	return raw, nil
}

// ocsCode extracts the OCS meta statuscode from a response body.
var ocsCodeRe = regexp.MustCompile(`<statuscode>(\d+)</statuscode>`)

func ocsCode(body []byte) string {
	if m := ocsCodeRe.FindSubmatch(body); m != nil {
		return string(m[1])
	}
	return ""
}

// ocsMessage extracts a short human-readable message for error reporting.
var ocsMsgRe = regexp.MustCompile(`<message>([^<]*)</message>`)

func ocsMessage(body []byte) string {
	if m := ocsMsgRe.FindSubmatch(body); m != nil {
		return string(m[1])
	}
	return "unexpected response " + truncate(body)
}
