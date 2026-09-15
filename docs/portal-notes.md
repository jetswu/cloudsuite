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

## Admin Console Full CRUD + Group Assignment (Sprint 1.2b)
- Fitur: full CRUD end-to-end user, group, dan role (bukan hanya create/delete). User form dialog (create + edit), group form dialog (create + edit), search + filter status.
- Group assignment: `PUT /api/admin/users/{id}/groups` (ganti membership penuh via `SetUserGroups`), `GET/POST/DELETE /api/admin/groups/{uuid}/members` (kelola anggota group via `ListGroupMembers`/`AddGroupMember`/`RemoveGroupMember`).
- Redirect fix: menu user (dropdown) menampilkan link "Admin" hanya untuk superadmin (`isSuperAdmin` prop dari `groups` session); `admin-shell` mengirim `isSuperAdmin` sehingga menu konsisten.
- Nested null fix (Fase F): Authentik serialize relasi nested yang tidak di-populate sebagai JSON `null` → Go decode jadi slice `nil` → re-marshal jadi `null`. Fix double-safety:
  1. Backend `repository/authentik/authentik.go`: helper `normalizeUser`/`normalizeGroup`/`normalizeUsers`/`normalizeGroups` memastikan semua slice read-model non-nil `[]` (dipanggil di `ListUsers`, `ListGroups`, `CreateUser`, `CreateGroup`, `UpdateUser`, `UpdateGroup`, `ListGroupMembers`).
  2. Frontend: guard `(u.groups_obj ?? [])` di `users-manager.tsx`, `(g.users ?? [])` di `groups-manager.tsx`.
- Domain: `User.GroupsObj []GroupRef`, `Group.UsersObj []User`, `SetGroupsRequest`, `MemberRequest` — semua slice field read-model TANPA `omitempty` agar selalu serialize sebagai array.
- Endpoint backend tambahan: `PUT /users/{id}/groups`, `GET /groups/{uuid}/members`, `POST /groups/{uuid}/members`, `DELETE /groups/{uuid}/members/{pk}`.
- Komponen frontend baru: `user-form-dialog.tsx`, `group-members-dialog.tsx`, `confirm-dialog.tsx`; hook `use-groups.ts`; shadcn `dialog`, `alert-dialog`, `checkbox`, `popover`.
- Cloudflare 1010 (Fase G): hanya memblokir fingerprint Python `urllib` (Bot Fight Mode), curl default UA dan browser UA keduanya HTTP 200 — tidak ada perubahan konfigurasi Cloudflare.

## Admin Add User + Password Handling (Sprint 1.2c)
- Model invitation dihapus. Ganti: admin create user + set password awal (sales-led, customer belum punya email aktif).
- Frontend: `lib/password-generator.ts` (CSPRNG `crypto.getRandomValues`, upper+lower+digit+symbol, Fisher–Yates shuffle). `user-form-dialog.tsx` field password mode auto-generate (default, readonly + Regenerate + Copy) / manual (show/hide toggle). Validasi min 8 char + huruf + angka. Edit mode TIDAK menampilkan password.
- Dialog sukses `user-created-dialog.tsx`: tampilkan username + password sekali, tombol Copy, warning kuat; setelah tutup password di-clear dari state (`users-manager.tsx` hold `createdUser`/`createdPassword` sementara, bukan persist).
- Backend: `CreateUserRequest` (embed `UserRequest` + `password`), `CreateUserResponse {user, password}`. Handler `POST /api/admin/users` decode `CreateUserRequest`, validasi password (`validatePassword`), lalu dua langkah: `CreateUser` → `SetUserPassword`, return `201 {user, password}`.
- PENTING — Authentik TIDAK menerima field `password` di body `POST /core/users/` (UserSerializer tanpa field password). Password di-set via endpoint terpisah `POST /core/users/{pk}/set_password/` (body `{"password": "..."}`, permission `authentik_core.reset_user_password`, return 204). Repository method baru `SetUserPassword` memakai endpoint ini.
- `UserResponse`/read-model `domain.User` TIDAK punya field password — tidak pernah expose password di list/update/read.

## Implicit Consent Flow (Sprint 1.2d)
- OIDC provider `portal` (PK 7) `authorization_flow` diubah ke `default-provider-authorization-implicit-consent` (`2c1630b0-4c43-4da1-aeeb-679a15dc5d18`).
- Efek UX: login user (baru maupun returning) langsung masuk ke Portal tanpa consent screen "Application requires following permissions".
- Berlaku juga untuk provider own-app lain: Nextcloud (PK 2), Odoo (PK 3), Stalwart Mail (PK 6).
- `redirect_uris`/`grant_types`/`property_mappings` TIDAK berubah. Detail: `docs/authentik-setup.md` → "Sprint 1.2d Implicit Consent Flow".

