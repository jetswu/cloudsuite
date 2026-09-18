// Package widget implements the Sprint 1.5a dashboard summaries (mail / drive
// / ERP). Data sources are the same credentials the pipeline already uses —
// Stalwart JMAP admin key, Nextcloud admin OCS + shared postgres, Odoo
// JSON-RPC — read-only, per the 1.5a contract ("jangan sentuh layanan
// existing kecuali untuk read-only widget").
//
// Field-verified against the live deployment (18 Sep 2026):
//   - Stalwart rejects sort on receivedAt/sentAt ("unsupportedSort"), so
//     recent lists fetch the newest-by-default-order window and sort
//     client-side by receivedAt.
//   - JMAP account ids on this instance are opaque (d, 9, u, j, i) and
//     matched by emailAddress client-side (x:Account/query has no email
//     filter — unsupportedFilter), mirroring provisioning/stalwart.go.
//   - Odoo runs base+mail only (no Accounting/CRM/Project/Inventory tables),
//     so the ERP widget surfaces the metrics that exist: partners, 30-day
//     mail activity, active users.
package widget

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jetswu/cloudsuite/portal/backend/internal/widgetcache"
)

const (
	mailCacheTTL  = 5 * time.Minute
	driveCacheTTL = 5 * time.Minute
	erpCacheTTL   = 15 * time.Minute

	mailKeyPrefix  = "widget:mail:"
	driveKeyPrefix = "widget:drive:"
	erpKey         = "widget:erp:global"
)

// Service computes widget summaries with Redis-backed caching. All methods
// are safe for concurrent use.
type Service struct {
	stalwartBase string
	stalwartKey  string

	ncBase   string // http://nextcloud
	ncUser   string
	ncPass   string
	ncdb     *pgxpool.Pool // nextcloud database (recent files)
	driveURL string        // public base for deep links

	odooBase   string
	odooDB     string
	odooLogin  string
	odooPass   string
	erpURL     string // public base for deep links
	webmailURL string // public base for mail deep links

	cache *widgetcache.Cache
	mu    sync.Mutex // serialises the single Redis connection
}

// New wires the widget service. ncdb and cache may be nil: without the
// Nextcloud DB the drive widget reports recent files as unavailable, without
// Redis every request fetches fresh (cache degrade, never an error).
func New(stalwartBase, stalwartKey, ncBase, ncUser, ncPass string,
	ncdb *pgxpool.Pool, driveURL string,
	odooBase, odooDB, odooLogin, odooPass, erpURL, webmailURL string,
	cache *widgetcache.Cache) *Service {
	return &Service{
		stalwartBase: strings.TrimRight(stalwartBase, "/"),
		stalwartKey:  stalwartKey,
		ncBase:       strings.TrimRight(ncBase, "/"),
		ncUser:       ncUser,
		ncPass:       ncPass,
		ncdb:         ncdb,
		driveURL:     strings.TrimRight(driveURL, "/"),
		odooBase:     strings.TrimRight(odooBase, "/"),
		odooDB:       odooDB,
		odooLogin:    odooLogin,
		odooPass:     odooPass,
		erpURL:       strings.TrimRight(erpURL, "/"),
		webmailURL:   strings.TrimRight(webmailURL, "/"),
		cache:        cache,
	}
}

// ---------- Mail ----------

// MailItem is one recent message.
type MailItem struct {
	ID         string `json:"id"`
	From       string `json:"from"`
	Subject    string `json:"subject"`
	ReceivedAt string `json:"received_at"`
	IsRead     bool   `json:"is_read"`
	DeepLink   string `json:"deep_link"`
}

// MailSummary is the /api/widgets/mail/summary payload.
type MailSummary struct {
	UnreadCount int64      `json:"unread_count"`
	Recent      []MailItem `json:"recent"`
	CachedAt    string     `json:"cached_at"`
	Provisioned bool       `json:"provisioned"`
}

// MailSummary returns the unread count and 5 most recent messages for the
// given portal user email (their Stalwart account), cached 5 minutes.
func (s *Service) MailSummary(ctx context.Context, email string) (*MailSummary, error) {
	if email == "" {
		return nil, fmt.Errorf("widget mail: no email claim")
	}
	var out MailSummary
	if s.cached(ctx, mailKeyPrefix+email, &out) {
		return &out, nil
	}

	accID, ok, err := s.stalwartAccountID(ctx, email)
	if err != nil {
		return nil, err
	}
	if !ok {
		return &MailSummary{Recent: []MailItem{}, CachedAt: nowRFC3339(), Provisioned: false}, nil
	}

	// calculateTotal with the unread filter gives unread_count; the unfiltered
	// window (server default order) is sorted client-side — Stalwart rejects
	// server-side sort on this deployment.
	var responses []jmapResponse
	body := map[string]any{
		"using": []string{"urn:ietf:params:jmap:core", "urn:ietf:params:jmap:mail"},
		"methodCalls": []any{
			[]any{"Email/query", map[string]any{"accountId": accID, "filter": map[string]any{"hasKeyword": "$unread"}, "calculateTotal": true, "limit": 1}, "qu"},
			[]any{"Email/query", map[string]any{"accountId": accID, "filter": map[string]any{}, "limit": 20}, "qd"},
			[]any{"Email/get", map[string]any{"accountId": accID, "#ids": map[string]any{"resultOf": "qd", "name": "Email/query", "path": "/ids"}, "properties": []string{"from", "subject", "receivedAt", "keywords"}}, "gd"},
		},
	}
	if err := s.jmap(ctx, body, &responses); err != nil {
		return nil, err
	}

	var unread int64
	var got []struct {
		ID         string
		From       string
		Subject    string
		ReceivedAt string
		IsRead     bool
	}
	for _, r := range responses {
		switch r.Name {
		case "Email/query":
			var q struct {
				Total int64 `json:"total"`
			}
			if err := json.Unmarshal(r.Args, &q); err == nil {
				unread = q.Total
			}
		case "Email/get":
			var g struct {
				List []struct {
					ID         string            `json:"id"`
					From       []mailAddress     `json:"from"`
					Subject    string            `json:"subject"`
					ReceivedAt string            `json:"receivedAt"`
					Keywords   map[string]string `json:"keywords"`
				} `json:"list"`
			}
			if err := json.Unmarshal(r.Args, &g); err != nil {
				continue
			}
			for _, m := range g.List {
				_, isRead := m.Keywords["$seen"]
				item := struct {
					ID         string
					From       string
					Subject    string
					ReceivedAt string
					IsRead     bool
				}{m.ID, firstAddress(m.From), m.Subject, m.ReceivedAt, isRead}
				got = append(got, item)
			}
		}
	}
	sort.Slice(got, func(i, j int) bool { return got[i].ReceivedAt > got[j].ReceivedAt })
	if len(got) > 5 {
		got = got[:5]
	}
	recent := make([]MailItem, 0, len(got))
	for _, m := range got {
		recent = append(recent, MailItem{
			ID:         m.ID,
			From:       m.From,
			Subject:    m.Subject,
			ReceivedAt: m.ReceivedAt,
			IsRead:     m.IsRead,
			DeepLink:   s.webmailURL + "/",
		})
	}
	out = MailSummary{UnreadCount: unread, Recent: recent, CachedAt: nowRFC3339(), Provisioned: true}
	s.store(ctx, mailKeyPrefix+email, out, mailCacheTTL)
	return &out, nil
}

type mailAddress struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

func firstAddress(addrs []mailAddress) string {
	if len(addrs) == 0 {
		return ""
	}
	if addrs[0].Name != "" {
		return addrs[0].Name
	}
	return addrs[0].Email
}

// stalwartAccountID resolves the JMAP account id for an email via
// x:Account/query + get (client-side email match, verified shape).
func (s *Service) stalwartAccountID(ctx context.Context, email string) (string, bool, error) {
	var responses []jmapResponse
	body := map[string]any{
		"using": []string{"urn:ietf:params:jmap:core", "urn:ietf:params:jmap:principals"},
		"methodCalls": []any{
			[]any{"x:Account/query", map[string]any{"filter": map[string]any{}, "limit": 50}, "q"},
			[]any{"x:Account/get", map[string]any{"#ids": map[string]any{"resultOf": "q", "name": "x:Account/query", "path": "/ids"}, "properties": []string{"id", "name", "emailAddress"}}, "g"},
		},
	}
	if err := s.jmap(ctx, body, &responses); err != nil {
		return "", false, err
	}
	for _, r := range responses {
		if r.Name != "x:Account/get" {
			continue
		}
		var g struct {
			List []struct {
				ID           string `json:"id"`
				EmailAddress string `json:"emailAddress"`
				Name         string `json:"name"`
			} `json:"list"`
		}
		if err := json.Unmarshal(r.Args, &g); err != nil {
			continue
		}
		for _, a := range g.List {
			if strings.EqualFold(a.EmailAddress, email) || (a.EmailAddress == "" && strings.EqualFold(a.Name, email)) {
				return a.ID, true, nil
			}
		}
	}
	return "", false, nil
}

// jmapResponse is one entry of the positional methodResponses array:
// a [name, args, callID] triple, NOT an object (JMAP RFC 8620 §3.3).
type jmapResponse struct {
	Name string
	Args json.RawMessage
}

// jmap POSTs a JMAP envelope to Stalwart with the admin API key and decodes
// the positional methodResponses array.
func (s *Service) jmap(ctx context.Context, body map[string]any, out any) error {
	raw, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal jmap: %w", err)
	}
	res, err := s.post(ctx, s.stalwartBase+"/jmap", "application/json", raw,
		map[string]string{"Authorization": "Bearer " + s.stalwartKey, "Accept": "application/json"})
	if err != nil {
		return fmt.Errorf("stalwart jmap: %w", err)
	}
	if res.status != 200 {
		return fmt.Errorf("stalwart jmap http %d: %s", res.status, truncate(res.body))
	}
	var envelope struct {
		MethodResponses [][]json.RawMessage `json:"methodResponses"`
	}
	if err := json.Unmarshal(res.body, &envelope); err != nil {
		return fmt.Errorf("decode jmap response: %w", err)
	}
	outs := make([]jmapResponse, 0, len(envelope.MethodResponses))
	for _, t := range envelope.MethodResponses {
		if len(t) != 3 {
			continue
		}
		var name string
		if err := json.Unmarshal(t[0], &name); err != nil {
			return fmt.Errorf("decode jmap response name: %w", err)
		}
		outs = append(outs, jmapResponse{Name: name, Args: t[1]})
	}
	switch out := out.(type) {
	case *[]jmapResponse:
		*out = outs
		return nil
	default:
		return fmt.Errorf("jmap: unsupported out type %T (use *[]jmapResponse)", out)
	}
}

// ---------- Drive ----------

// DriveFile is one recent file.
type DriveFile struct {
	Name       string `json:"name"`
	Size       int64  `json:"size"`
	ModifiedAt string `json:"modified_at"`
	Mime       string `json:"mime"`
	DeepLink   string `json:"deep_link"`
}

// DriveSummary is the /api/widgets/drive/summary payload.
type DriveSummary struct {
	Storage struct {
		UsedBytes  int64   `json:"used_bytes"`
		TotalBytes int64   `json:"total_bytes"` // -3 (unlimited) passes through
		Percent    float64 `json:"percent"`     // 0 when unlimited
	} `json:"storage"`
	Recent      []DriveFile `json:"recent"`
	CachedAt    string      `json:"cached_at"`
	Provisioned bool        `json:"provisioned"`
}

// DriveSummary returns the user's storage usage (OCS quota, verified live)
// and the 5 most recently modified files (shared-postgres oc_filecache),
// cached 5 minutes.
func (s *Service) DriveSummary(ctx context.Context, uid string) (*DriveSummary, error) {
	if uid == "" {
		return nil, fmt.Errorf("widget drive: no uid (authentik user lookup failed)")
	}
	var out DriveSummary
	if s.cached(ctx, driveKeyPrefix+uid, &out) {
		return &out, nil
	}

	used, total, err := s.ncQuota(ctx, uid)
	if err != nil {
		// A user that exists in Authentik but was never provisioned (or was
		// deleted) has no Nextcloud account: report it, don't fail the call.
		return &DriveSummary{Recent: []DriveFile{}, CachedAt: nowRFC3339(), Provisioned: false}, nil
	}
	out.Storage.UsedBytes = used
	out.Storage.TotalBytes = total
	if total > 0 {
		out.Storage.Percent = float64(used) / float64(total) * 100
	}
	out.Recent = s.ncRecentFiles(ctx, uid)
	out.CachedAt = nowRFC3339()
	out.Provisioned = true
	s.store(ctx, driveKeyPrefix+uid, out, driveCacheTTL)
	return &out, nil
}

// ncQuota returns used/total bytes from the OCS user object
// (data.quota.used / .total; total -3 = unlimited).
func (s *Service) ncQuota(ctx context.Context, uid string) (int64, int64, error) {
	res, err := s.post(ctx, s.ncBase+"/ocs/v1.php/cloud/users/"+uid+"?format=json", "", nil,
		map[string]string{
			"Authorization":  "Basic " + basicAuth(s.ncUser, s.ncPass),
			"OCS-APIRequest": "true",
		})
	if err != nil {
		return 0, 0, fmt.Errorf("nextcloud ocs: %w", err)
	}
	if res.status != 200 {
		return 0, 0, fmt.Errorf("nextcloud ocs http %d", res.status)
	}
	var q struct {
		OCS struct {
			Data struct {
				Quota struct {
					Used  int64 `json:"used"`
					Total int64 `json:"total"`
				} `json:"quota"`
			} `json:"data"`
		} `json:"ocs"`
	}
	if err := json.Unmarshal(res.body, &q); err != nil {
		return 0, 0, fmt.Errorf("decode ocs quota: %w", err)
	}
	return q.OCS.Data.Quota.Used, q.OCS.Data.Quota.Total, nil
}

// ncRecentFiles reads the 5 newest files from the user's home storage.
// Requires the shared-postgres Nextcloud DB pool; without it the list is
// empty (storage numbers still render).
func (s *Service) ncRecentFiles(ctx context.Context, uid string) []DriveFile {
	out := []DriveFile{}
	if s.ncdb == nil {
		return out
	}
	rows, err := s.ncdb.Query(ctx, `
		SELECT f.name, f.size, f.mtime, COALESCE(m.mimetype, ''),
		       COALESCE(f.path, '')
		FROM oc_filecache f
		JOIN oc_storages st ON st.numeric_id = f.storage AND st.id = $1
		LEFT JOIN oc_mimetypes m ON m.id = f.mimetype
		WHERE f.path LIKE 'files/%' AND f.path NOT LIKE 'files_versions/%'
		ORDER BY f.mtime DESC
		LIMIT 5`, "home::"+uid)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var name, mime, path string
		var size, mtime int64
		if err := rows.Scan(&name, &size, &mtime, &mime, &path); err != nil {
			continue
		}
		dir := ""
		if i := strings.LastIndex(path, "/"); i > len("files") {
			dir = path[len("files"):i] // e.g. /Photos
		}
		link := s.driveURL + "/apps/files/?dir=/" + strings.TrimPrefix(dir, "/")
		if dir == "" {
			link = s.driveURL + "/apps/files/?dir=/"
		}
		out = append(out, DriveFile{
			Name:       name,
			Size:       size,
			ModifiedAt: time.Unix(mtime, 0).UTC().Format(time.RFC3339),
			Mime:       mime,
			DeepLink:   link,
		})
	}
	return out
}

// ---------- ERP ----------

// ErpMetric is one named metric card.
type ErpMetric struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Value int64  `json:"value"`
}

// ErpSummary is the /api/widgets/erp/summary payload. The deployment runs
// Odoo base+mail only, so the metrics are the ones that exist there.
type ErpSummary struct {
	Metrics  []ErpMetric `json:"metrics"`
	CachedAt string      `json:"cached_at"`
}

// ErpSummary returns the ERP metrics via JSON-RPC search_count, cached 15
// minutes. The ERP widget is site-wide (base install has no per-user data).
func (s *Service) ErpSummary(ctx context.Context) (*ErpSummary, error) {
	var out ErpSummary
	if s.cached(ctx, erpKey, &out) {
		return &out, nil
	}
	uid, err := s.odooAuth(ctx)
	if err != nil {
		return nil, err
	}

	since := time.Now().UTC().AddDate(0, 0, -30).Format("2006-01-02 15:04:05")
	partners, err := s.odooCount(ctx, uid, "res.partner", []any{})
	if err != nil {
		return nil, err
	}
	messages, err := s.odooCount(ctx, uid, "mail.message", []any{[]any{"date", ">=", since}})
	if err != nil {
		return nil, err
	}
	users, err := s.odooCount(ctx, uid, "res.users", []any{[]any{"active", "=", true}})
	if err != nil {
		return nil, err
	}

	out = ErpSummary{
		Metrics: []ErpMetric{
			{Key: "partners_total", Label: "Kontak terdaftar", Value: partners},
			{Key: "messages_30d", Label: "Aktivitas pesan 30 hari", Value: messages},
			{Key: "users_active", Label: "User ERP aktif", Value: users},
		},
		CachedAt: nowRFC3339(),
	}
	s.store(ctx, erpKey, out, erpCacheTTL)
	return &out, nil
}

// odooAuth authenticates against /web/session/authenticate (verified envelope)
// and returns the numeric uid.
func (s *Service) odooAuth(ctx context.Context) (int, error) {
	params := map[string]any{"db": s.odooDB, "login": s.odooLogin, "password": s.odooPass}
	var res struct {
		Result struct {
			UID int `json:"uid"`
		} `json:"result"`
		Error *rpcError `json:"error"`
	}
	if err := s.odooCall(ctx, "/web/session/authenticate", params, &res); err != nil {
		return 0, err
	}
	if res.Error != nil {
		return 0, fmt.Errorf("odoo auth error: %s", res.Error.msg())
	}
	if res.Result.UID == 0 {
		return 0, fmt.Errorf("odoo auth rejected (uid=0)")
	}
	return res.Result.UID, nil
}

// odooCount runs a search_count through /jsonrpc (execute_kw, verified path).
func (s *Service) odooCount(ctx context.Context, uid int, model string, domain []any) (int64, error) {
	params := map[string]any{
		"service": "object",
		"method":  "execute_kw",
		"args":    []any{s.odooDB, uid, s.odooPass, model, "search_count", []any{domain}},
	}
	var res struct {
		Result int64     `json:"result"`
		Error  *rpcError `json:"error"`
	}
	if err := s.odooCall(ctx, "/jsonrpc", params, &res); err != nil {
		return 0, err
	}
	if res.Error != nil {
		return 0, fmt.Errorf("odoo search_count %s: %s", model, res.Error.msg())
	}
	return res.Result, nil
}

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

func (s *Service) odooCall(ctx context.Context, path string, rpcParams map[string]any, out any) error {
	raw, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "method": "call", "params": rpcParams})
	if err != nil {
		return fmt.Errorf("marshal odoo request: %w", err)
	}
	res, err := s.post(ctx, s.odooBase+path, "application/json", raw, nil)
	if err != nil {
		return fmt.Errorf("odoo request: %w", err)
	}
	if res.status != 200 {
		return fmt.Errorf("odoo http %d: %s", res.status, truncate(res.body))
	}
	if err := json.Unmarshal(res.body, out); err != nil {
		return fmt.Errorf("decode odoo response: %w", err)
	}
	return nil
}

// ---------- cache + http plumbing ----------

// cached fills out from Redis when a fresh entry exists.
func (s *Service) cached(ctx context.Context, key string, out any) bool {
	if s.cache == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	raw, err := s.cache.Get(key)
	if err != nil || raw == "" {
		return false
	}
	if err := json.Unmarshal([]byte(raw), out); err != nil {
		return false
	}
	return true
}

// store writes the summary to Redis; failures are silent (cache is
// best-effort — a down Redis must not break the widgets).
func (s *Service) store(ctx context.Context, key string, v any, ttl time.Duration) {
	if s.cache == nil {
		return
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_ = s.cache.Set(key, string(raw), ttl)
}

func nowRFC3339() string { return time.Now().UTC().Format(time.RFC3339) }

func truncate(b []byte) string {
	s := string(b)
	if len(s) > 200 {
		s = s[:200] + "…"
	}
	return s
}
