# Authentik Baseline Setup — CloudSuite (staging)

- **Authentik version:** 2026.8.2 (server + worker, image `ghcr.io/goauthentik/server:2026.8.2`)
- **Domain:** `auth.idchsuite.my.id` (via nginx `auth.conf`, upstream `authentik-server:9000`)
- **Tenant default domain:** `auth.idchsuite.my.id`
- **Base URL:** `https://auth.idchsuite.my.id`

## Brand

- **Name:** CloudSuite (`Brand CloudSuite`)
- **Default brand:** yes, domain `auth.idchsuite.my.id`
- **Authentication flow:** `default-authentication-flow` (`Welcome to authentik!`, pk `4063fc1a-cde4-4ab6-801a-9ef5b3cc6b7f`)
- **Brand pk:** `198d2e4e-8e22-4c4c-a5cf-783766e50e1f`
- **Verifikasi:** `curl -skL https://auth.idchsuite.my.id/` → `HTTP:200`, `<title>CloudSuite</title>`

## Groups

| Group | pk |
|---|---|
| `cloudsuite-superadmin` | `0175716b-408c-4b6a-a764-817f4d5d3a5a` |
| `cloudsuite-tenant-admin` | `d58df92a-cd22-4216-ace5-2d890d56d49b` |
| `cloudsuite-users` | `953d2315-8bbf-4809-9415-0391bf4cabd9` |

- `akadmin` (pk 6) adalah member `cloudsuite-superadmin` (+ bawaan `authentik Admins`).
- Verifikasi: `GET /api/v3/core/users/6/` → `groups: ['authentik Admins', 'cloudsuite-superadmin']`.

## Provider template (OIDC)

- **Name:** `CloudSuite Services Template`, pk `1` (oauth2)
- **Application slug:** `template-cloudsuite-services` (Application pk `c48c953b-15fa-4c00-a9db-d58a827ac2e9`)
- **Client ID:** `SQtwKPEkkyg5XV08kMK2bemOI8guqeI8`
- **Client type:** confidential, `redirect_uris: []` (template — isi saat layanan riil didaftarkan)
- **Authorization flow:** `default-provider-authorization-explicit-consent` (`7f475261-68fa-45de-9226-de2be34014a5`)
- **Invalidation flow:** `default-provider-invalidation-flow` (`3353607b-6e34-406a-95f6-74ef80852a35`)
- **Signing key:** bawaan `authentik Self-signed Certificate` (`38b70fc0-69b8-44fa-b959-ad02ca4197da`)
- **Subject mode:** `hashed_user_id`, **Issuer mode:** `global`, property mappings default (openid/email/profile)
- **Client secret:** tersimpan di `/opt/cloudsuite/.env` sebagai `AUTHENTIK_TEMPLATE_CLIENT_SECRET` — TIDAK ditulis di dokumen ini.
- **Well-known:** `https://auth.idchsuite.my.id/application/o/template-cloudsuite-services/.well-known/openid-configuration` → `HTTP:200`, berisi `issuer`, `authorization_endpoint`, `token_endpoint`, `jwks_uri`.

## Catatan IPv6 (PENTING)

Kernel staging **IPv6 mati total**. Authentik default bind `[::]:9000/9443/9300` → `OSError 97 EAFNOSUPPORT` di `server.rs:33`, restart-loop ±45 detik.
Fix: override di **kedua** service (server + worker):

```yaml
AUTHENTIK_LISTEN__HTTP: 0.0.0.0:9000
AUTHENTIK_LISTEN__HTTPS: 0.0.0.0:9443
AUTHENTIK_LISTEN__METRICS: 0.0.0.0:9300
```

Jangan hapus baris ini saat edit compose.

## Catatan token

- **Tenant setting:** `default_token_duration = days=90` (`PATCH /api/v3/admin/settings/`).
- Token API `hermes-automation` (identifier `hermes-automation`, intent `api`, user `akadmin`): expiry ±90 hari dari pembuatan.
- Token yang expiring **tidak bisa diperpanjang via PATCH** (`/authentik/core/api/tokens.py` baris 88–89: `For API tokens, expires cannot be overridden` — kode versi 2026.8.2 memaksa durasi default tenant).
- Nilai token tersimpan di `/opt/cloudsuite/.env` sebagai `AUTHENTIK_API_TOKEN` — TIDAK ditulis di dokumen ini, TIDAK di-commit.
- DILARANG membuat token no-expiry (`expiring=false`).

### Prosedur rotate token (revoke & recreate manual)

1. Buat token sementara: `POST /api/v3/core/tokens/` `{"identifier":"hermes-rotate-tmp","intent":"api","user":6}` → HTTP:201.
2. Ambil key-nya: `GET /api/v3/core/tokens/hermes-rotate-tmp/view_key/` (pakai **identifier**, bukan pk).
3. Verifikasi token sementara: `GET /api/v3/core/users/me/` → HTTP:200.
4. Hapus token lama: `DELETE /api/v3/core/tokens/hermes-automation/` → HTTP:204.
5. Buat token baru: `POST /api/v3/core/tokens/` `{"identifier":"hermes-automation","intent":"api","user":6}` → HTTP:201, `expires` ±90 hari.
6. Ambil key token baru via `view_key/`, update `AUTHENTIK_API_TOKEN` di `/opt/cloudsuite/.env` (`chmod 600`).
7. Hapus token sementara: `DELETE /api/v3/core/tokens/hermes-rotate-tmp/` → HTTP:204.
8. Verifikasi akhir: `GET /api/v3/core/users/me/` → HTTP:200 dan `GET /api/v3/core/tokens/hermes-automation/` menunjukkan `expires` baru.

## Command yang berhasil (changelog)

- `printf '<BOOTSTRAP_PASS>\n<BOOTSTRAP_PASS>\n' | docker exec -i cloudsuite-authentik-server ak changepassword akadmin` → `Password successfully changed for user akadmin`
- `PATCH /api/v3/admin/settings/ {"default_token_duration":"days=90"}` → HTTP:200
- `POST /api/v3/providers/oauth2/` (dengan `authorization_flow`, `invalidation_flow`, `signing_key`) → HTTP:201 (tanpa `invalidation_flow` → HTTP:400 `This field is required.`)
- `POST /api/v3/core/applications/ {"slug":"template-cloudsuite-services","provider":1}` → HTTP:201

## Provider Nextcloud (OIDC) -- Sprint 0.8 (2026-09-10)

- Name: Nextcloud, pk 2 (oauth2)
- Application: Nextcloud, slug nextcloud (Application pk 6748a3a5-0fb6-4c14-9b6d-d08758ec63d6)
- Client ID: o3AMAxUA1nW5XEZW6cczhmAqYoWNHXU9T3zODp8y (bukan secret -- boleh tampil)
- Client secret: tersimpan di Nextcloud OCC config (user_oidc provider CloudSuite) -- TIDAK ditulis di dokumen ini.
- Redirect URI (strict): https://drive.idchsuite.my.id/apps/user_oidc/code
- Discovery URI: https://auth.idchsuite.my.id/application/o/nextcloud/.well-known/openid-configuration -- HTTP:200
- Authorization flow: default-provider-authorization-explicit-consent (7f475261-68fa-45de-9226-de2be34014a5)
- Invalidation flow: default-provider-invalidation-flow (3353607b-6e34-406a-95f6-74ef80852a35)
- Signing key: bawaan authentik Self-signed Certificate (38b70fc0-69b8-44fa-b959-ad02ca4197da)
- Subject mode: hashed_user_id, Issuer mode: global, Scopes: openid/email/profile
- grant_types WAJIB diisi eksplisit via API (authorization_code, hybrid, implicit, client_credentials, password, device_code, refresh_token).
  API default grant_types kosong -> authorize gagal invalid_request The request is otherwise malformed + log Invalid grant_type for provider (Authentik 2026.8.2, diverifikasi dari source views/authorize.py baris 233).
- Nextcloud: image nextcloud:34.0.3-apache (stable terbaru 2026-09-10), container cloudsuite-nextcloud, URL https://drive.idchsuite.my.id, status.php installed:true, user_oidc 8.11.0 enabled.
- SSO test end-to-end (2026-09-10): Login Nextcloud -- redirect Authentik -- login akadmin -- consent Continue -- dashboard Nextcloud sebagai authentik Default Admin OK. User OIDC ter-provision: 307f28ffa44da592bb5ae1730fb58c19c0a26314c5d07c232614f42d79732293.
- OCC config (user_oidc 8.x): pakai php occ user_oidc:provider NAME dengan flag clientid, clientsecret, discoveryuri, unique-uid=1 (BUKAN config:app:set format lama -- itu membuat provider dengan clientId kosong).

## Catatan operasional

- Reset password akadmin: docker exec -i cloudsuite-authentik-server ak shell lalu set_password() + save() (terbukti PW-SET-OK). Alternatif: printf PASS newline PASS via pipe ke docker exec -i cloudsuite-authentik-server ak changepassword akadmin.
- AUTHENTIK_BOOTSTRAP_PASSWORD hanya berlaku saat setup awal (first-run). Setelah user ada di DB, reset via ak shell / ak changepassword, BUKAN via env tersebut.
- Buat provider OAuth2 via API: POST /api/v3/providers/oauth2/ WAJIB sertakan authorization_flow + invalidation_flow + signing_key + grant_types (tanpa invalidation_flow -> HTTP:400).
- Nextcloud ncadmin = break-glass only (akun darurat, JANGAN dipakai user biasa; login normal via SSO). Detail: docs/nextcloud-notes.md (disable via occ user:disable ncadmin, enable balik via occ user:enable ncadmin). Status 2026-09-10: AKTIF, keputusan disable menunggu persetujuan.
- Sprint 0.8b (2026-09-10): healthcheck nextcloud (curl -f http://localhost/status.php, interval 30s) -- docker compose ps nextcloud (healthy). Token hermes-automation expiry 2026-12-09 (format YYYY-MM-DD; laporan 0.8 memakai DD/MM 12/09/2026 = 9 Desember 2026).

## Sprint 0.9 Odoo 18 + SSO OIDC (2026-09-10)
- Odoo: image cloudsuite-odoo:18.0 (FROM odoo:18.0 + python-jose + odoo-addon-auth-oidc==18.0.1.1.0.2), container cloudsuite-odoo, URL https://erp.idchsuite.my.id, DB odoo OWNER cloudsuite.
- odoo.conf: 11 baris, list_db=False, dbfilter=^odoo\$ (fix database selector), owner 100:101 mode 600, live di /opt/cloudsuite/data/odoo/config/odoo.conf; repo hanya odoo.conf.example (password=CHANGE_ME).
- Authentik provider Odoo pk=3, client_id RAfdMHBRIePsrqb2LYBD6NsHYUEbZ8cQlXBJoqES, grant authorization_code+refresh_token, redirect strict https://erp.idchsuite.my.id/auth_oauth/signin.
- Authentik app slug=odoo PROVIDER pk=3.
- OAuth provider Odoo id=6 CloudSuite flow=id_token_code enabled, scope openid email profile, auth/token/jwks endpoint Authentik.
- SSO: /web/login langsung form (tanpa selector), tombol Login with CloudSuite -> redirect Authentik default-authentication-flow 200, tanpa error Odoo. Login manual akadmin menunggu admin.
- Selector: /web/database/selector manager disabled, tidak bocor list DB (authentik/cloudsuite/nextcloud count 0).
