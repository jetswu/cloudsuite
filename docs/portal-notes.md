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

## Domain Onboarding + DKIM Mode Selection (Sprint 1.3 / 1.3b / 1.3-fix3)
- Admin → Domains → Tambah Domain: nama + pilih Mode DKIM (RSA saja [default] / Ed25519 saja / Dual) dengan tooltip penjelasan per opsi. Pilih Ed25519 muncul warning merah (provider lama bisa reject).
- Setup page `/admin/domains/{id}/setup`: tombol "Ubah Mode DKIM" → modal radio + warning → PATCH `/api/admin/domains/{id}` `{dkim_mode}`.
- Semantik mode (live-verified Stalwart 0.16): `dkimManagement = {"@type":"Automatic","algorithms":{...}}`; rsa = hanya `Dkim1RsaSha256`, ed25519 = hanya `Dkim1Ed25519Sha256`, dual = keduanya. Nama algorithm BUKAN nilai `@type`.
- Ubah mode = recreate domain Stalwart (key hanya digenerate saat create); portal domain id + record MX/SPF/DMARC tetap, DKIM di-regenerate, status `dns_in_progress`, verify ulang dari wizard.
- Migration `00003_add_dkim_mode.sql`: kolom `domains.dkim_mode` default `rsa` + CHECK constraint; existing domain otomatis `rsa`.
- End-to-end test bertag `e2e` (`internal/service/e2e_dkim_mode_test.go`) jalan di network compose dengan Stalwart+DB nyata; create 4 domain test lalu cleanup otomatis.

## Provisioning Service (Sprint 1.4a)
- Arsitektur: Portal `POST /api/admin/users` (handler CreateUser) -> validasi -> insert `provisioning_jobs` + LPUSH Redis -> worker `cloudsuite-portal-worker` (BRPOP) -> 3 connectors.
- Job types: `provision` | `deprovision` (deprovision baru jalan di Sprint 1.4b; worker 1.4a menolak dengan pesan eksplisit "not supported in Sprint 1.4a").
- Services: `stalwart` (create account via JMAP `x:Account/set`, SSO-only tanpa password), `nextcloud` (OCS create user + pre-insert mapping `oc_user_oidc` provider 1, sub=uid Authentik), `odoo` (DEFERRED - tidak create apa pun; status `active` + external_id `deferred-first-login`; akun Odoo auto-create saat first login SSO).
- Status job: `queued -> running -> success|failed` (CHECK constraint migration 00004); retry max 3 attempt (`max_attempts`).
- Retry: exponential backoff 30s -> 2m -> 8m (base 30s x4 per attempt, cap 8m; `backoff()` di service.go).
- Sweep recovery: tiap 60 detik worker re-enqueue (a) job `queued` lebih tua dari 2 menit (enqueue Redis hilang) dan (b) job `failed` yang `next_retry_at` sudah lewat. Terbukti live: job odoo di-inject DB-only (tanpa Redis) diangkat sweep lalu sukses.
- Identitas di-resolve worker via Authentik API `GET /core/users/{pk}/` (trailing slash wajib - DRF 404 tanpa itu) memakai pk user portal.
- Validasi sebelum trigger: format email + domain terdaftar di tabel `domains` (via list domain Stalwart).
- Idempotent: terbukti live di Stalwart (job duplikat -> tetap tepat 1 akun, tanpa error); connector Nextcloud cek user exist via OCS dulu; Odoo deferred idempotent by design.
- Queue: Redis list key `provisioning:jobs` (LPUSH/BRPOP FIFO); payload JSON raw. Redis client RESP minimal ditulis sendiri (tanpa dependency baru).
- Persist: `provisioning_jobs` (per service) + `user_provisioning` (agregat per user: `*_status`, `*_external_id`).
- Env worker: REDIS_ADDR/REDIS_PASSWORD, AUTHENTIK_API_URL/AUTHENTIK_API_TOKEN/AUTHENTIK_ISSUER/AUTHENTIK_CLIENT_ID, STALWART_API_URL/STALWART_API_KEY, NEXTCLOUD_BASE_URL (WAJIB host canonical `https://drive.idchsuite.my.id` - `http://nextcloud` di-redirect ke HTML), NEXTCLOUD_ADMIN_USER/NEXTCLOUD_ADMIN_PASS, ODOO_BASE_URL/ODOO_DB/ODOO_ADMIN_USER/ODOO_ADMIN_PASSWORD.
- Worker TIDAK menjalankan migration (hindari race goose dengan backend) - migration jalan di portal-backend saja.

## De-provisioning + UI Status (Sprint 1.4b)
- Endpoint admin: `DELETE /api/admin/users/{id}` (soft-delete akun Authentik + enqueue 3 job deprovision) dan `POST /api/admin/users/{id}/retry-provision` (re-enqueue job gagal/pending; 202). `GET /api/admin/users` membawa agregat provisioning per user (`provisioning: {stalwart, nextcloud, odoo}`) → badge Mail/Drive/ERP di `/admin/users` (polling 5s saat ada job in-flight).
- Migration `00005_deprovision_status.sql`: enum status ditambah `pending_delete` (CHECK constraint diperluas); jalan otomatis di portal-backend.
- Worker `handleDeprovision` dispatch per service; urutan handler: tulis status `pending_delete` + insert jobs DULU → LPUSH Redis belakangan → delete akun Authentik paling akhir (hindari race worker vs UPDATE status).
- Guard anti re-create: job `provision` untuk user berstatus `pending_delete`/`deleted` di-skip worker (menutup akar anomali re-create 1.4a-fix).
- Connector Stalwart destroy: JMAP TIDAK punya method `{Type}/destroy` — hapus via `x:Account/set` argumen `destroy:[ids]` (RFC 8620 §5.3), setelah resolve id by email (`x:Account/query`) + unlink dari group (`x:Group/set`).
- Connector Nextcloud destroy: OCS DELETE + SQL `oc_users` + `oc_user_oidc` (kolom `user_id`) — OCS tidak menghapus row `oc_users`.
- Connector Odoo destroy: SQL langsung relasi → res_user → res_partner (by login/email), dalam transaksi.
- Ketiga connector idempotent: objek sudah tidak ada = success — job deprovision fresh aman dipakai menutup status tertinggal (jangan reset attempts job failed).
- E2E terverifikasi 17 Sep: testcycle14b create → 3x active (external_id terisi) → delete via API → 3x deleted; akun hilang dari `x:Account/query`; job deprovision success attempts=1 via kode baru (bukan workaround CLI).

## Audit Log (Sprint 1.5a)
- Tabel `audit_logs` (migration 00006): append-only via trigger PL/pgSQL (UPDATE/DELETE → exception). Migration dibungkus `-- +goose StatementBegin/End` (goose split per `;` memecah body function — lihat TROUBLESHOOTING).
- `AuditLogger` (internal/service/audit.go): non-blocking (insert async `context.WithoutCancel`, nil-safe = no-op, error audit tidak pernah memutus request).
- Handler ter-instrument (11 aksi): user.create/update/delete/set_groups, group.create/update/delete, domain.create/delete/verify/update_dkim, provisioning.retry. `service_code` domain = `mail` (konsistensi filter).
- API (superadmin): `GET /api/admin/audit` (filter actor/action/service/from/to/search + limit max 200 + offset) dan `GET /api/admin/audit/export` (CSV, cap 10k).
- UI `/admin/audit`: tabel + filter + pagination + Export CSV + modal detail metadata JSON; menu sidebar "Audit Log".

## Widget Stats (Sprint 1.5a)
- Endpoint (JWT): `GET /api/widgets/{mail,drive,erp}/summary` (internal/widget + widgetcache Redis).
- Cache: mail 5m (key per email), drive 5m (key per uid), erp 15m (global).
- Mail: JMAP admin API key — resolve accountId per email (`x:Account/query`, id opaque) → satu request batch: `Email/query` filter `$unread` + `calculateTotal` (unread count) + `Email/query` limit 20 + `Email/get` (`#ids` ref ke query) untuk 5 pesan terbaru. Sort `receivedAt` TIDAK didukung Stalwart → fetch + sort client-side.
- Drive: OCS quota user (admin) + `oc_filecache` DB Nextcloud untuk 5 file terakhir (JOIN mount).
- ERP (metrik ADAPTIF — deployment hanya base+mail): Kontak (res_partner), Aktivitas pesan 30h (mail_message), User aktif (res_users) via JSON-RPC `search_count`. Invoice/stock/tasks tidak memungkinkan sampai modul di-install.
- Identitas dari JWT SAJA (tanpa Authentik API): mail=email claim; drive=`sub` (uid hex) = userid Nextcloud (mapping `oc_user_oidc` sub=uid, Sprint 1.4a).
- Soft-degrade: kegagalan per-widget → 200 `{unavailable:true,error}`; UI tampilkan "Data tidak tersedia" + tombol retry; dashboard tetap utuh.
