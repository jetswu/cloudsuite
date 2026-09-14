# Portal Notes — CloudSuite

## Overview
- URL: https://portal.idchsuite.my.id
- Arsitektur: Nginx → /api → portal-backend (Go), / → portal-frontend (Next.js)
- Container: cloudsuite-portal-backend, cloudsuite-portal-frontend

## Auth Flow
- Next.js (NextAuth) handle OIDC ke Authentik
- Access token disimpan di session
- Setiap call Go API kirim Bearer token
- Go validate JWT via Authentik JWKS

## Authentik Provider
- Nama: portal (PK 7)
- Redirect URI: https://portal\.idchsuite\.my\.id/api/auth/callback/authentik
- Client type: confidential
- sub_mode: user_email

## Env Vars
- PORTAL_URL
- PORTAL_AUTHENTIK_CLIENT_ID
- PORTAL_AUTHENTIK_CLIENT_SECRET
- PORTAL_AUTHENTIK_ISSUER
- PORTAL_AUTH_SECRET

## Deploy
Lihat PANDUAN-DEPLOY.md Bagian 12c.

## Troubleshooting
Lihat TROUBLESHOOTING.md.
