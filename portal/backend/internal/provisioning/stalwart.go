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
)

// StalwartConnector provisions mail accounts on Stalwart via its JMAP API.
// Syntax verified live on this deployment (Sprint 1.4 pre-flight, 16 Sep 2026):
// x:Account/set create {"@type":"User","name":<local>,"domainId":<id>} — the
// server generates emailAddress = name@domain; duplicates answer
// primaryKeyViolation. No password is set: mailbox access is SSO-only (1.4a
// decision; app passwords are a 1.5 backlog item).
type StalwartConnector struct {
	baseURL string // e.g. http://<stalwart>:8080 (no trailing slash)
	apiKey  string
	http    *http.Client
}

// NewStalwartConnector returns a connector for the Stalwart JMAP endpoint.
func NewStalwartConnector(baseURL, apiKey string) *StalwartConnector {
	return &StalwartConnector{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		http:    &http.Client{Timeout: 15 * time.Second},
	}
}

// Service implements Connector.
func (c *StalwartConnector) Service() string { return "stalwart" }

// Provision creates the mail account (idempotent: an existing account counts
// as success and returns its id). externalID = Stalwart account id.
func (c *StalwartConnector) Provision(ctx context.Context, p JobPayload) (string, error) {
	at := strings.Index(p.UserEmail, "@")
	if at <= 0 || at == len(p.UserEmail)-1 {
		return "", fmt.Errorf("invalid email %q", p.UserEmail)
	}
	local, dom := p.UserEmail[:at], p.UserEmail[at+1:]

	// Idempotency pre-check: query by full email address.
	if id, exists, err := c.queryAccount(ctx, p.UserEmail); err != nil {
		return "", err
	} else if exists {
		return id, nil
	}

	domainID, err := c.domainID(ctx, dom)
	if err != nil {
		return "", err
	}

	var created struct {
		Created map[string]struct {
			ID string `json:"id"`
		} `json:"created"`
		NotCreated map[string]struct {
			Description string `json:"description"`
		} `json:"notCreated"`
	}
	args, err := c.call(ctx, "x:Account/set", map[string]any{
		"create": map[string]any{
			"c1": map[string]any{"@type": "User", "name": local, "domainId": domainID},
		},
	}, []string{"urn:ietf:params:jmap:core", "urn:stalwart:jmap", "urn:ietf:params:jmap:principals"})
	if err != nil {
		return "", err
	}
	if err := json.Unmarshal(args, &created); err != nil {
		return "", fmt.Errorf("decode account/set response: %w", err)
	}
	if ent, ok := created.Created["c1"]; ok && ent.ID != "" {
		return ent.ID, nil
	}
	if nc, ok := created.NotCreated["c1"]; ok {
		if strings.Contains(nc.Description, "primaryKeyViolation") {
			// Raced with another worker: the account now exists — treat as
			// idempotent success and resolve its id.
			id, exists, qerr := c.queryAccount(ctx, p.UserEmail)
			if qerr != nil {
				return "", qerr
			}
			if exists {
				return id, nil
			}
		}
		return "", fmt.Errorf("stalwart create rejected: %s", nc.Description)
	}
	return "", fmt.Errorf("stalwart create: unexpected response %s", truncate(args))
}

// Deprovision destroys the Stalwart account for p.UserEmail (Sprint 1.4b).
// Idempotent: a missing account is a successful no-op. Accounts that are
// members of a group must be unlinked first — Stalwart refuses to destroy a
// linked principal (verified during the 1.4a-fix cleanup).
func (c *StalwartConnector) Deprovision(ctx context.Context, p JobPayload) error {
	if p.UserEmail == "" {
		return fmt.Errorf("stalwart deprovision: missing user_email in job payload")
	}
	id, exists, err := c.queryAccount(ctx, p.UserEmail)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}

	// Unlink group memberships before destroy: memberGroupIds maps group ids
	// to true, so patching each to false removes the link.
	groups, err := c.memberGroups(ctx, id)
	if err != nil {
		return err
	}
	for gid := range groups {
		if err := c.setAccount(ctx, id, map[string]any{
			"memberGroupIds": map[string]any{gid: false},
		}); err != nil {
			return fmt.Errorf("stalwart unlink group %s: %w", gid, err)
		}
	}

	// JMAP has no {Type}/destroy method — deletion goes through
	// x:Account/set with the destroy argument (RFC 8620 section 5.3).
	args, err := c.call(ctx, "x:Account/set", map[string]any{
		"destroy": []string{id},
	}, []string{"urn:ietf:params:jmap:core", "urn:stalwart:jmap", "urn:ietf:params:jmap:principals"})
	if err != nil {
		return fmt.Errorf("stalwart destroy: %w", err)
	}
	var res struct {
		Destroyed    []string `json:"destroyed"`
		NotDestroyed map[string]struct {
			Description string `json:"description"`
		} `json:"notDestroyed"`
	}
	if err := json.Unmarshal(args, &res); err != nil {
		return fmt.Errorf("decode account/set destroy response: %w", err)
	}
	for _, v := range res.NotDestroyed {
		return fmt.Errorf("stalwart destroy rejected: %s", v.Description)
	}
	return nil
}

// memberGroups returns the group ids the account currently belongs to.
func (c *StalwartConnector) memberGroups(ctx context.Context, id string) (map[string]bool, error) {
	args, err := c.call(ctx, "x:Account/get", map[string]any{
		"ids":        []string{id},
		"properties": []string{"id", "memberGroupIds"},
	}, []string{"urn:ietf:params:jmap:core", "urn:stalwart:jmap", "urn:ietf:params:jmap:principals"})
	if err != nil {
		return nil, err
	}
	var out struct {
		List []struct {
			ID             string         `json:"id"`
			MemberGroupIDs map[string]any `json:"memberGroupIds,omitempty"`
		} `json:"list"`
	}
	if err := json.Unmarshal(args, &out); err != nil {
		return nil, fmt.Errorf("decode account/get response: %w", err)
	}
	groups := map[string]bool{}
	for _, a := range out.List {
		for gid := range a.MemberGroupIDs {
			groups[gid] = true
		}
	}
	return groups, nil
}

// setAccount patches account fields via x:Account/set update.
func (c *StalwartConnector) setAccount(ctx context.Context, id string, patch map[string]any) error {
	args, err := c.call(ctx, "x:Account/set", map[string]any{
		"update": map[string]any{id: patch},
	}, []string{"urn:ietf:params:jmap:core", "urn:stalwart:jmap", "urn:ietf:params:jmap:principals"})
	if err != nil {
		return err
	}
	var res struct {
		NotUpdated map[string]struct {
			Description string `json:"description"`
		} `json:"notUpdated"`
	}
	if err := json.Unmarshal(args, &res); err != nil {
		return fmt.Errorf("decode account/set update response: %w", err)
	}
	for _, v := range res.NotUpdated {
		return fmt.Errorf("stalwart update rejected: %s", v.Description)
	}
	return nil
}

// queryAccount lists accounts and matches by emailAddress. Stalwart does not
// support a server-side email filter on x:Account/query (verified live:
// unsupportedFilter), so matching happens client-side.
func (c *StalwartConnector) queryAccount(ctx context.Context, email string) (string, bool, error) {
	var resp struct {
		List []struct {
			ID           string `json:"id"`
			Name         string `json:"name"`
			EmailAddress string `json:"emailAddress"`
		} `json:"list"`
	}
	args, err := c.call(ctx, "x:Account/get", map[string]any{
		"#ids":        map[string]any{"resultOf": "q", "name": "x:Account/query", "path": "/ids"},
		"properties":  []string{"id", "name", "emailAddress"},
	}, []string{"urn:ietf:params:jmap:core", "urn:stalwart:jmap"}, `["x:Account/query",{"filter":{},"limit":50},"q"]`)
	if err != nil {
		return "", false, err
	}
	if err := json.Unmarshal(args, &resp); err != nil {
		return "", false, fmt.Errorf("decode account/get response: %w", err)
	}
	for _, a := range resp.List {
		if strings.EqualFold(a.EmailAddress, email) || (a.EmailAddress == "" && strings.EqualFold(a.Name, email)) {
			return a.ID, true, nil
		}
	}
	return "", false, nil
}

// domainID resolves the Stalwart domain id for a DNS name via
// x:Domain/query + x:Domain/get (probe-20 shape).
func (c *StalwartConnector) domainID(ctx context.Context, name string) (string, error) {
	var resp struct {
		List []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"list"`
	}
	args, err := c.call(ctx, "x:Domain/get", map[string]any{
		"#ids":       map[string]any{"resultOf": "q", "name": "x:Domain/query", "path": "/ids"},
		"properties": []string{"id", "name"},
	}, []string{"urn:ietf:params:jmap:core", "urn:stalwart:jmap"}, `["x:Domain/query",{"filter":{},"limit":100},"q"]`)
	if err != nil {
		return "", err
	}
	if err := json.Unmarshal(args, &resp); err != nil {
		return "", fmt.Errorf("decode domain/get response: %w", err)
	}
	for _, d := range resp.List {
		if strings.EqualFold(d.Name, name) {
			return d.ID, nil
		}
	}
	return "", fmt.Errorf("stalwart: domain %s not found", name)
}

// call sends a JMAP request and returns the raw arguments of the response for
// the named method. extraCalls appends prerequisite method calls (e.g. the
// "q" query part that a #ids back-reference resolves against).
func (c *StalwartConnector) call(ctx context.Context, name string, args map[string]any, using []string, extraCalls ...string) (json.RawMessage, error) {
	rawArgs, err := json.Marshal(args)
	if err != nil {
		return nil, fmt.Errorf("marshal jmap args: %w", err)
	}
	calls := make([]json.RawMessage, 0, len(extraCalls)+1)
	for _, e := range extraCalls {
		calls = append(calls, json.RawMessage(e))
	}
	calls = append(calls, json.RawMessage(`[`+mustQuote(name)+`,`+string(rawArgs)+`,"g"]`))

	body, err := json.Marshal(map[string]any{"using": using, "methodCalls": calls})
	if err != nil {
		return nil, fmt.Errorf("marshal jmap request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/jmap", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build jmap request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("stalwart jmap request: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read stalwart response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("stalwart jmap http %d: %s", resp.StatusCode, truncate(raw))
	}

	var parsed struct {
		MethodResponses [][]json.RawMessage `json:"methodResponses"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("decode jmap response: %w", err)
	}
	for _, mr := range parsed.MethodResponses {
		if len(mr) >= 2 && string(mr[0]) == mustQuote(name) {
			return mr[1], nil
		}
	}
	return nil, fmt.Errorf("stalwart jmap: no %s response", name)
}

func mustQuote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func truncate(b []byte) string {
	s := string(b)
	if len(s) > 300 {
		s = s[:300] + "..."
	}
	return s
}
