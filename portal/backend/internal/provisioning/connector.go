package provisioning

import (
	"context"
	"crypto/rand"
	"math/big"
)

// Connector provisions one service for one user. Implementations must be
// idempotent: running Provision twice for the same user must not create
// duplicates (an existing account counts as success).
type Connector interface {
	// Service returns the provisioning_jobs.service value it handles
	// ("stalwart", "nextcloud", "odoo").
	Service() string
	// Provision creates (or verifies) the service account and returns the
	// external id to persist in user_provisioning.<service>_external_id.
	Provision(ctx context.Context, p JobPayload) (externalID string, err error)
}

// passwordAlphabet excludes visually ambiguous characters; the generated
// password only satisfies Nextcloud's "password required" field since users
// authenticate via SSO (Sprint 1.4a decision: no human-knowable passwords).
const passwordAlphabet = "abcdefghjkmnpqrstuvwxyzABCDEFGHJKMNPQRSTUVWXYZ23456789"

// RandomPassword returns a cryptographically random alphanumeric string.
func RandomPassword(n int) (string, error) {
	out := make([]byte, n)
	max := big.NewInt(int64(len(passwordAlphabet)))
	for i := range out {
		k, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		out[i] = passwordAlphabet[k.Int64()]
	}
	return string(out), nil
}
