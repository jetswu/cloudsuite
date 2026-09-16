// Package stalwart implements a thin JMAP client for the Stalwart management
// API. Stalwart v0.16 exposes management objects (x:Domain, x:DkimSignature,
// ...) over JMAP at POST /jmap using the "urn:stalwart:jmap" capability, NOT
// a REST /api/domain surface. Verified against the live server on 2026-09-15.
package stalwart

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/jetswu/cloudsuite/portal/backend/internal/domain"
)

const (
	jmapUsing   = "urn:stalwart:jmap"
	accountID   = "d" // admin@idchsuite.my.id, the only management account
	jmapPath    = "/jmap"
	sessionPath = "/jmap/session"
)

// Domain is a Stalwart mail domain as returned by x:Domain/get. It carries
// the server-assigned id and the generated DNS zone file (the authoritative
// list of records the customer must publish).
type Domain struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	DNSZoneFile string `json:"dnsZoneFile"`
}

// Client is a Stalwart management API client. It is safe for concurrent use.
type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

// NewClient returns a client bound to baseURL (e.g. http://cloudsuite-stalwart:8080)
// authenticated with apiKey (Bearer).
func NewClient(baseURL, apiKey string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		http:    &http.Client{Timeout: 20 * time.Second},
	}
}

// jmapRequest is the outer JMAP envelope. MethodCalls is a sequence of
// [methodName, arguments, clientCallID] triples.
type jmapRequest struct {
	Using       []string        `json:"using"`
	MethodCalls [][]interface{} `json:"methodCalls"`
}

// jmapResponse is the outer JMAP response envelope.
type jmapResponse struct {
	MethodResponses [][]interface{} `json:"methodResponses"`
}

// RegisterDomain creates a domain in Stalwart with Manual DNS + certificate
// management and the requested DKIM management mode (Sprint 1.3b), then
// reads back the created object so its DNS zone file is available to
// generate the portal's DNS records. Returns the Stalwart domain
// (id + name + zone file).
func (c *Client) RegisterDomain(ctx context.Context, name string, mode domain.DKIMMode) (Domain, error) {
	// 1. Create. The create key ("t1") is client-chosen and echoed back in
	//    created.t1.id.
	createArgs := map[string]interface{}{
		"accountId": accountID,
		"create": map[string]interface{}{
			"t1": map[string]interface{}{
				"name":                  name,
				"isEnabled":             true,
				"dkimManagement":        mapDKIMMode(mode),
				"dnsManagement":         map[string]interface{}{"@type": "Manual"},
				"certificateManagement": map[string]interface{}{"@type": "Manual"},
			},
		},
	}
	resp, err := c.call(ctx, [][]interface{}{{"x:Domain/set", createArgs, "c0"}})
	if err != nil {
		return Domain{}, fmt.Errorf("create domain: %w", err)
	}

	var stalwartID string
	// Parse created.t1.id from the x:Domain/set response.
	if created, ok := c.methodData(resp, "x:Domain/set", "created"); ok {
		if t1, ok := created.(map[string]interface{})["t1"].(map[string]interface{}); ok {
			stalwartID, _ = t1["id"].(string)
		}
	}
	if stalwartID == "" {
		return Domain{}, fmt.Errorf("create domain %q: no id in response", name)
	}

	// 2. Stalwart generates DKIM keys asynchronously, so the zone file may
	//    not yet contain the *_domainkey records right after the domain is
	//    created. Poll getDomain until the DKIM records appear (or timeout),
	//    so the portal does not persist an empty DKIM set. On timeout the
	//    domain is still returned — creation must not fail because of a slow
	//    DKIM generation.
	d, err := c.waitForDKIM(ctx, stalwartID, mode)
	if err != nil {
		return Domain{}, fmt.Errorf("read created domain: %w", err)
	}
	return d, nil
}

// waitForDKIM polls the created/updated domain until its dnsZoneFile contains
// the DKIM selector records for every algorithm the requested mode requires.
// Stalwart writes those asynchronously (Automatic DKIM management), so the
// first getDomain may return a zone without them — and after a mode switch
// the OLD algorithm's record satisfies a naive "_domainkey" check while the
// new signature is still being generated. Matching on the k= algorithm tags
// avoids returning early with a stale DKIM set. It gives up after ~10s
// (5 attempts, 2s apart) and returns the last zone seen.
func (c *Client) waitForDKIM(ctx context.Context, id string, mode domain.DKIMMode) (Domain, error) {
	const (
		maxAttempts = 5
		delay       = 2 * time.Second
	)

	var (
		last Domain
		err  error
	)
	for i := 0; i < maxAttempts; i++ {
		last, err = c.getDomain(ctx, id)
		if err != nil {
			return Domain{}, err
		}
		if zoneHasAlgorithms(last.DNSZoneFile, mode) {
			return last, nil
		}
		time.Sleep(delay)
	}

	log.Warn().
		Str("domain_id", id).
		Str("mode", string(mode)).
		Msg("DKIM records for requested algorithm(s) did not appear in zone file after polling; proceeding with last zone (may be incomplete)")
	return last, nil
}

// zoneHasAlgorithms reports whether the zone file carries DKIM TXT records
// for every algorithm required by the mode (k=rsa / k=ed25519 tags).
func zoneHasAlgorithms(zone string, mode domain.DKIMMode) bool {
	hasRSA := strings.Contains(zone, "k=rsa")
	hasEd := strings.Contains(zone, "k=ed25519")
	switch mode {
	case domain.DKIMModeEd25519:
		return hasEd
	case domain.DKIMModeDual:
		return hasRSA && hasEd
	default:
		return hasRSA
	}
}

// mapDKIMMode maps the portal DKIMMode onto Stalwart's dkimManagement
// object. Stalwart 0.16 (verified against the live server 2026-09-16) models
// DKIM management as @type "Automatic" with an `algorithms` map of enabled
// algorithm names; "Dkim1RsaSha256" / "Dkim1Ed25519Sha256" are algorithm
// names, NOT @type values (passing them as @type fails with invalidPatch:
// "Missing or invalid '@type' property"). Restricting the map to a single
// algorithm yields "RSA only" / "Ed25519 only" signing.
func mapDKIMMode(mode domain.DKIMMode) map[string]interface{} {
	algorithms := map[string]interface{}{ // default: rsa only
		"Dkim1RsaSha256": true,
	}
	switch mode {
	case domain.DKIMModeEd25519:
		algorithms = map[string]interface{}{
			"Dkim1Ed25519Sha256": true,
		}
	case domain.DKIMModeDual:
		algorithms = map[string]interface{}{
			"Dkim1RsaSha256":      true,
			"Dkim1Ed25519Sha256": true,
		}
	}
	return map[string]interface{}{
		"@type":      "Automatic",
		"algorithms": algorithms,
	}
}

// ListDomains returns all Stalwart domains (id + name).
func (c *Client) ListDomains(ctx context.Context) ([]Domain, error) {
	resp, err := c.call(ctx, [][]interface{}{
		{"x:Domain/query", map[string]interface{}{"accountId": accountID}, "c0"},
	})
	if err != nil {
		return nil, fmt.Errorf("list domains: %w", err)
	}
	ids := []string{}
	if q, ok := c.methodData(resp, "x:Domain/query", "ids"); ok {
		for _, id := range c.asStringSlice(q) {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return []Domain{}, nil
	}

	out := []Domain{}
	for _, id := range ids {
		d, err := c.getDomain(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("get domain %s: %w", id, err)
		}
		out = append(out, d)
	}
	return out, nil
}

// DeleteDomain removes a domain and its linked DKIM signatures from Stalwart.
// Stalwart refuses to destroy a Domain while DkimSignature objects still
// reference it (objectIsLinked), so the DKIM signatures are destroyed first.
func (c *Client) DeleteDomain(ctx context.Context, id string) error {
	// 1. Find DKIM signatures linked to this domain.
	dkimIDs, err := c.listDKIMSignatures(ctx, id)
	if err != nil {
		return fmt.Errorf("list dkim signatures: %w", err)
	}

	// 2. Destroy DKIM signatures, then the domain, in a single method-call
	//    sequence. Order matters: DKIM before Domain.
	calls := [][]interface{}{}
	if len(dkimIDs) > 0 {
		calls = append(calls, []interface{}{"x:DkimSignature/set", map[string]interface{}{
			"accountId": accountID,
			"destroy":   dkimIDs,
		}, "c0"})
	}
	calls = append(calls, []interface{}{"x:Domain/set", map[string]interface{}{
		"accountId": accountID,
		"destroy":   []string{id},
	}, "c1"})

	resp, err := c.call(ctx, calls)
	if err != nil {
		return fmt.Errorf("delete domain: %w", err)
	}

	// Surface any notDestroyed objects.
	for i, mr := range resp.MethodResponses {
		if len(mr) >= 2 {
			if data, ok := mr[1].(map[string]interface{}); ok {
				if nd, ok := data["notDestroyed"].(map[string]interface{}); ok && len(nd) > 0 {
					return fmt.Errorf("delete domain (call %d): notDestroyed: %v", i, nd)
				}
			}
		}
	}
	return nil
}

// getDomain fetches a single domain by id.
func (c *Client) getDomain(ctx context.Context, id string) (Domain, error) {
	resp, err := c.call(ctx, [][]interface{}{
		{"x:Domain/get", map[string]interface{}{"accountId": accountID, "ids": []string{id}}, "c0"},
	})
	if err != nil {
		return Domain{}, err
	}
	if list, ok := c.methodData(resp, "x:Domain/get", "list"); ok {
		if arr, ok := list.([]interface{}); ok && len(arr) > 0 {
			if obj, ok := arr[0].(map[string]interface{}); ok {
				d := Domain{
					ID:   stringVal(obj["id"]),
					Name: stringVal(obj["name"]),
				}
				d.DNSZoneFile = stringVal(obj["dnsZoneFile"])
				return d, nil
			}
		}
	}
	return Domain{}, fmt.Errorf("domain %s not found", id)
}

// listDKIMSignatures returns the ids of DkimSignature objects that reference
// the given domain (matched via a query filter on domainId).
func (c *Client) listDKIMSignatures(ctx context.Context, domainID string) ([]string, error) {
	resp, err := c.call(ctx, [][]interface{}{
		{"x:DkimSignature/query", map[string]interface{}{
			"accountId": accountID,
			"filter":    map[string]interface{}{"domainId": domainID},
		}, "c0"},
	})
	if err != nil {
		return nil, err
	}
	ids := []string{}
	if q, ok := c.methodData(resp, "x:DkimSignature/query", "ids"); ok {
		ids = c.asStringSlice(q)
	}
	return ids, nil
}

// call POSTs a JMAP request and decodes the response envelope.
func (c *Client) call(ctx context.Context, methodCalls [][]interface{}) (*jmapResponse, error) {
	reqBody := jmapRequest{
		Using:       []string{jmapUsing},
		MethodCalls: methodCalls,
	}
	buf, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal jmap request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+jmapPath, bytes.NewReader(buf))
	if err != nil {
		return nil, fmt.Errorf("build jmap request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("jmap request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
		return nil, fmt.Errorf("stalwart jmap %s: %s", resp.Status, strings.TrimSpace(string(raw)))
	}

	var out jmapResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode jmap response: %w", err)
	}
	return &out, nil
}

// methodData extracts a named field from the arguments object of the first
// method response matching methodName. Returns (nil, false) if absent.
func (c *Client) methodData(resp *jmapResponse, methodName, field string) (interface{}, bool) {
	for _, mr := range resp.MethodResponses {
		if len(mr) < 2 {
			continue
		}
		name, _ := mr[0].(string)
		if name != methodName {
			continue
		}
		args, ok := mr[1].(map[string]interface{})
		if !ok {
			return nil, false
		}
		v, ok := args[field]
		return v, ok
	}
	return nil, false
}

func (c *Client) asStringSlice(v interface{}) []string {
	arr, ok := v.([]interface{})
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, e := range arr {
		if s, ok := e.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func stringVal(v interface{}) string {
	s, _ := v.(string)
	return s
}
