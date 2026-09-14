package auth

import (
	"context"
	"fmt"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

// NewVerifier initializes the OIDC provider against the Authentik issuer
// and returns an ID token verifier bound to the client ID.
func NewVerifier(ctx context.Context, issuer, clientID string) (*oidc.IDTokenVerifier, error) {
	provider, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		return nil, fmt.Errorf("oidc provider %s: %w", issuer, err)
	}
	return provider.Verifier(&oidc.Config{ClientID: clientID}), nil
}

// NewOAuth2Config builds an oauth2 config (kept for future flows, e.g. client
// credentials or server-side login).
func NewOAuth2Config(issuer, clientID, clientSecret, redirectURL string, scopes []string) (*oauth2.Config, error) {
	provider, err := oidc.NewProvider(context.Background(), issuer)
	if err != nil {
		return nil, fmt.Errorf("oidc provider %s: %w", issuer, err)
	}
	return &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint:     provider.Endpoint(),
		RedirectURL:  redirectURL,
		Scopes:       scopes,
	}, nil
}
