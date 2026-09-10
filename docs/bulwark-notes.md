# Bulwark Webmail — CloudSuite Staging

## Info
- **Image**: `ghcr.io/bulwarkmail/webmail:latest` (v1.9.2)
- **Container**: `cloudsuite-bulwark`
- **Network**: `cloudsuite-net`
- **Port**: 3000 (internal)
- **URL**: `https://webmail.idchsuite.my.id` (via Nginx reverse proxy)
- **Admin**: Dashboard disabled (no ADMIN_PASSWORD set)

## Environment Variables
- `JMAP_SERVER_URL=https://mail.idchsuite.my.id`
- `OAUTH_ENABLED=true`
- `OAUTH_ONLY=true`
- `OAUTH_CLIENT_ID=stalwart-mail`
- `OAUTH_CLIENT_SECRET=[REDACTED] — in /opt/cloudsuite/.env`
- `OAUTH_ISSUER_URL=https://auth.idchsuite.my.id/application/o/stalwart-mail/`
- `SESSION_SECRET=[REDACTED] — in /opt/cloudsuite/.env`
- `STALWART_FEATURES=true`
- `ALLOW_CUSTOM_JMAP_ENDPOINT=false`

## OIDC Provider (Authentik)
- Provider name: `stalwart-mail`
- Client ID: `stalwart-mail`
- Client secret: in `.env` as `BULWARK_CLIENT_SECRET`
- Redirect URI: `https://webmail.idchsuite.my.id/api/auth/callback`
- Issuer mode: `per_provider`
- Signing key: `38b70fc0-69b8-44fa-b959-ad02ca4197da` (shared with other providers)
- Invalidation flow: `3353607b-6e34-406a-95f6-74ef80852a35` (shared)
- Application slug: `stalwart-mail`

## Stalwart OIDC Directory Config
```json
{
  "@type": "Oidc",
  "description": "Authentik",
  "issuerUrl": "https://auth.idchsuite.my.id/application/o/stalwart-mail",
  "claimUsername": "email",
  "claimName": "name",
  "claimGroups": "groups",
  "requireAudience": "stalwart-mail"
}
```

## CORS Fix
- `usePermissiveCors = true` set via `stalwart-cli update Http`
- Required because Bulwark runs on different subdomain than Stalwart

## SSO Flow
1. User visits `https://webmail.idchsuite.my.id`
2. Bulwark detects no session → redirects to Authentik
3. Authentik authenticates user (akadmin)
4. Authentik returns tokens to Bulwark callback
5. Bulwark uses tokens to authenticate with Stalwart via JMAP
6. Stalwart validates tokens against Authentik OIDC endpoints
7. User sees inbox

## Deviation from PRD
- PRD planned 2 providers (stalwart-mail + bulwark-webmail) → simplified to 1 provider
- PRD OIDC Directory schema (endpoint/fields/cache) was wrong → actual schema (issuerUrl/claimUsername/claimName)
- PRD claimed "TANPA trailing slash" for OAUTH_ISSUER_URL → Authentik requires trailing slash

## Known Issues
- First login: mailbox must exist in Stalwart before OIDC login works
- DNS `webmail.idchsuite.my.id` must be configured by admin (A record → 103.117.56.35)

## Volumes
- `bulwark-config`: `/app/data/admin` (persistent config)
- `bulwark-state`: `/app/data/admin-state` (persistent state)

## Redirect URI Config

### Issue
Bulwark pakai Next.js i18n → callback URL selalu ada locale prefix: `/en/auth/callback`, `/id/auth/callback`.

### Fix di Authentik
Redirect URI harus pakai `matching_mode: regex`:
```json
{
  "matching_mode": "regex",
  "url": "https://webmail\\.idchsuite\\.my\\.id/[a-z]{2}/auth/callback",
  "redirect_uri_type": "authorization"
}
```

### Grant Types
WAJIB set: `["authorization_code", "refresh_token"]`

### Scopes
Bulwark request: `openid email profile` (offline_access juga perlu untuk refresh token).
Authentik auto-allows via "overlap" mode walau tidak explicitly configured.

## OIDC issuerUrl Trailing Slash

Stalwart OIDC Directory `issuerUrl` HARUS pakai trailing slash:
`https://auth.idchsuite.my.id/application/o/stalwart-mail/`

Bulwark env `OAUTH_ISSUER_URL` TANPA trailing slash:
`https://auth.idchsuite.my.id/application/o/stalwart-mail`

Beda karena cara masing-masing library handle URL construction.
Bulwark menambahkan `/.well-known/openid-configuration` sendiri.
Stalwart lakukan exact string match terhadap `iss` claim di JWT.

Cara cek issuer yang benar:
```bash
curl -s https://auth.idchsuite.my.id/application/o/stalwart-mail/.well-known/openid-configuration | jq -r .issuer
```

## OIDC issuerUrl trailing slash (fix3)
Stalwart OIDC Directory  **HARUS pakai trailing slash** supaya match Authentik actual issuer. Bulwark env  **TANPA trailing slash**. Beda karena cara masing-masing library handle URL normalization.

Authentik actual issuer:  (WITH trailing slash).

## Authentik sub_mode & property_mappings (fix4)
-  =  (NOT ) — supaya Authentik include  claim di ID token
-  =  scope mappings — supaya claims ,  diinclude
- Default  tidak include email claim → Stalwart  gagal

