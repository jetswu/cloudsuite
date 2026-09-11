# jmap-webmail Notes — CloudSuite

## Overview
- URL: https://webmail.idchsuite.my.id
- Image: `ghcr.io/root-fr/jmap-webmail:1.7.1` (upstream: root-fr/jmap-webmail)
- Container: `cloudsuite-jmapwebmail`
- Protokol: JMAP (native Stalwart)
- Lisensi: MIT
- Pengganti Bulwark (deprecated karena bug double-exchange) — lihat `bulwark-notes.md`
- Deployed: Sprint 0.10d (clean slate), SSO OK Sprint 0.10e (2026-09-11)

## Env Vars (di `.env`)
- `JMAPWEBMAIL_CLIENT_ID` — dari Authentik provider `stalwart-mail` (client_id = `stalwart-mail`)
- `JMAPWEBMAIL_CLIENT_SECRET` — dari Authentik provider `stalwart-mail`
- `JMAPWEBMAIL_OIDC_ISSUER_URL` — `https://auth.idchsuite.my.id/application/o/stalwart-mail` — **TANPA trailing slash!**
- `JMAPWEBMAIL_JMAP_URL` — `https://mail.idchsuite.my.id`

Di compose dipetakan ke env container:
`OAUTH_CLIENT_ID`, `OAUTH_CLIENT_SECRET`, `OAUTH_ISSUER_URL`, `JMAP_SERVER_URL`
(`OAUTH_ENABLED=true`, `OAUTH_ONLY=true`).

## Authentik Provider (`stalwart-mail`)
- Client type: confidential
- sub_mode: `user_email`
- include_claims_in_id_token: true
- redirect_uris regex: `https://webmail\.idchsuite\.my\.id(/[a-z]{2})?/auth/callback`
- grant_types: `authorization_code`, `refresh_token`
- property_mappings: 3 scope mappings (openid/email/profile) — REUSE dari provider existing
- Detail lengkap: `authentik-setup.md` § Provider Stalwart Mail

## Redirect URI Convention
- Path: `/auth/callback` (tanpa locale) atau `/{locale}/auth/callback`
- Regex optional locale: `(/[a-z]{2})?`
- BUKAN `[a-z]{2}` wajib — jmap-webmail kadang kirim callback tanpa locale
- Tanpa regex match → Authentik log "Invalid redirect URI"

## Trailing Slash Convention (PENTING)
- `OAUTH_ISSUER_URL` di .env: **TANPA** trailing slash (dipakai webmail untuk discovery)
- Issuer token `iss` (dari Authentik): **DENGAN** trailing slash
- Stalwart Directory `issuerUrl`: **DENGAN** trailing slash (validasi `iss` di Stalwart)
- Beda app beda kebutuhan — jangan asumsikan seragam

## CORS (PENTING)
- jmap-webmail fetch OIDC discovery dari BROWSER (client-side) → cross-origin ke `auth.idchsuite.my.id`
- Fix: headers CORS di nginx `mail.conf` (`add_header Access-Control-Allow-Origin https://webmail.idchsuite.my.id always` + preflight OPTIONS 204)
- (Historis: pernah dipertimbangkan Cloudflare Transform Rule di auth hostname + path `.well-known`; implementasi aktual = nginx headers di mail.conf, committed Sprint 0.10h)
- `stalwart-cli update Http --json '{"usePermissiveCors":true}'` — untuk CORS Stalwart API (beda kasus)

## Login User
- Pakai **EMAIL** (mis. `admin@idchsuite.my.id`), BUKAN username
- Setelah `Authentication.directoryId = <OIDC directory id>` di Stalwart, login via OIDC
- Konsekuensi directoryId=OIDC: admin password login (Basic CLI/webadmin) 401 — manajemen via ApiKey CLI

## ApiKey Stalwart (manajemen)
- Secret ApiKey: `/opt/cloudsuite/secrets/stalwart-apikey.txt` (600) + `STALWART_API_KEY` di `.env`
- CLI: `stalwart-cli --url http://<stalwart-ip>:8080 --api-key "$KEY" get <setting>`
- Recovery: `STALWART_RECOVERY_ADMIN` (lihat TROUBLESHOOTING.md recovery mode)

## Health
- Endpoint: `/api/health` (docker healthcheck wget spider)
- Status: healthy

## Deploy
Lihat `DEPLOYMENT.md` §12b (jmap-webmail)

## Troubleshooting
Lihat `TROUBLESHOOTING.md`
