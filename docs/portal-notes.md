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

## Nginx Routing (WAJIB)
- `/api/auth/*` → portal-frontend:3000 (route NextAuth `app/api/auth/[...nextauth]`)
- `/api/*` → portal-backend:8080 (API Go)
- `/` → portal-frontend:3000

Kalau `/api/auth/` tidak di-split ke frontend, semua endpoint NextAuth 404 (default 404 Go) → SSO gagal total.

## Troubleshooting
Lihat TROUBLESHOOTING.md.

### 502 saat callback SSO (session cookie besar)
- Gejala: login Authentik sukses, callback `/api/auth/callback/authentik` → 502 Cloudflare.
- Log nginx: `upstream sent too big header while reading response header from upstream`.
- Root cause: NextAuth set cookie session JWT (berisi accessToken Authentik) > proxy_buffer_size default nginx.
- Fix (sudah diterapkan, commit `93be5c7`): `proxy_buffer_size 16k; proxy_buffers 8 16k; proxy_busy_buffers_size 24k;` di server block portal.conf.

## Design System
- Design tokens CSS variable di `app/globals.css` (light + dark), Tailwind 4 `@custom-variant dark`.
- Primary `#1e40af`, accent `#3b82f6`, dark bg `#0f172a`, dark card `#1e293b`, border `#334155`.
- Font: Geist Sans (default Next.js 15) + Geist Mono; logo text "CloudSuite" + lucide `Cloud`.
- Dark mode = default; toggle di user menu (Moon/Sun), localStorage key `theme` ("light" mematikan dark, selainnya dark).
- Komponen shadcn: button, card, input, label, dropdown-menu, avatar, skeleton, separator; custom: service-card, user-menu, theme-toggle, me-status.
- Tambah komponen shadcn: `pnpm dlx shadcn@latest add <comp>` (jangan downgrade Tailwind 4).

## JWKS Empty Fix (Sprint 1.0b-fix)
- Gejala: dashboard prod `/api/me` → 401 "Gagal memuat data akun"; JWKS provider `portal` return `{}`.
- Root cause: provider OIDC `portal` (PK 7) tidak punya `signing_key` → go-oidc verifier tanpa key → semua token ditolak 401.
- Fix: PATCH `signing_key` provider 7 = `38b70fc0-69b8-44fa-b959-ad02ca4197da` (reuse cert existing, sama dengan provider stalwart-mail/Nextcloud/Odoo).
- Verify: `curl -s https://auth.idchsuite.my.id/application/o/portal/jwks/` → `{"keys":[{"kty":"RSA","alg":"RS256",...}]}`.

## Redirect Layanan (Sprint 1.1)
- Mail → https://webmail.idchsuite.my.id
- Drive → https://drive.idchsuite.my.id
- ERP → https://erp.idchsuite.my.id
- Buka di tab baru (`target="_blank"`, `rel="noopener noreferrer"`).
- `ServiceCard` = wrap `<Link>` seluruh card + `cursor-pointer` + hover border/bg.

## Mail SSO CORS Fix (Sprint 1.1-fix)
- Provider `stalwart-mail` (PK 6) `redirect_uris` = 2 entry:
  1. regex `https://webmail\.idchsuite\.my\.id(/[a-z]{2})?/auth/callback` (callback validation, existing).
  2. strict `https://webmail.idchsuite.my.id` (origin polos, untuk CORS browser).
- Alasan: Authentik CORS allowlist pakai literal urlparse match, regex redirect_uri tidak cukup → tambah origin polos strict.

## Admin Console (Sprint 1.2)
- URL: `/admin` (superadmin only) → redirect ke `/admin/users`.
- Akses: hanya user dengan group `cloudsuite-superadmin` (non-superadmin → redirect `/dashboard`, API → 403 `{"error":"not superadmin"}`).
- Halaman: `/admin/users` (list + create + delete user), `/admin/groups` (list + create + delete group).
- Endpoint backend: `/api/admin/users`, `/api/admin/groups` (proxy ke Authentik API via `AUTHENTIK_API_URL` + `AUTHENTIK_API_TOKEN`).
- Middleware: `RequireSuperAdmin` cek JWT claim `groups` mengandung `cloudsuite-superadmin`.
- Frontend: `middleware.ts` proteksi route `/admin/*` (unauthenticated → `/login`, non-superadmin → `/dashboard`).
- Env backend baru: `AUTHENTIK_API_URL` (base Authentik, contoh `http://authentik-server:9000`), `AUTHENTIK_API_TOKEN` (token API admin).
- Tenant admin UI (per-tenant) akan di `/manage` (Phase 2) — BUKAN bagian Sprint 1.2.

