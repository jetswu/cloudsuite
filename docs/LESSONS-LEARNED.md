# CloudSuite — Lessons Learned

Ringkasan semua pembelajaran penting dari Phase 1 (Sprint 0.0–0.10j).
Referensi silang ke docs detail — file ini TIDAK mengulang detail, hanya
poin kunci + arah ke sumber.

## 1. Konvensi Wajib (JANGAN dilanggar)

### 1.1 Trailing Slash Convention
| Konteks | Format | Alasan |
|---|---|---|
| Stalwart `Directory.issuerUrl` | DENGAN `/` | match `iss` token Authentik |
| jmap-webmail `JMAPWEBMAIL_OIDC_ISSUER_URL` | TANPA `/` | discovery URL |
| Token `iss` claim (Authentik) | DENGAN `/` | bawaan Authentik |

**Reference:** TROUBLESHOOTING.md § "Bulwark SSO — Authentication Failed
(Issuer Mismatch)", stalwart-notes.md, jmap-webmail-notes.md

### 1.2 Redirect URI Regex
- Pakai `(/[a-z]{2})?/auth/callback` — locale optional
- JANGAN `[a-z]{2}/auth/callback` — jadinya locale wajib (redirect error)

**Reference:** TROUBLESHOOTING.md § "SSO Redirect URI Error", authentik-setup.md

### 1.3 Stalwart `Authentication.directoryId`
- `= null` (internal) → password login admin WORK, SSO GAGAL
- `= OIDC directory id` → SSO WORK, password login admin 401 MATI
- **Tradeoff wajib** — setelah SSO jalan, management pakai ApiKey
  (generate saat recovery, lihat SECRET-GENERATION.md)

**Reference:** TROUBLESHOOTING.md, stalwart-notes.md

### 1.4 Stalwart config.json
- File config cuma `DataStore` — BUKAN Bootstrap
- Sisanya via `stalwart-cli apply` NDJSON (ada di repo:
  `infra/stalwart-*.ndjson`)
- Durasi = integer milidetik, BUKAN string `"30s"`

**Reference:** TROUBLESHOOTING.md §23 (Bootstrap reject), §26 (duration
integer), infra/stalwart-*.ndjson

### 1.5 Authentik provider (Stalwart Mail)
- `client_id` (`stalwart-mail`) HARUS match Stalwart `requireAudience`
- `property_mappings` WAJIB eksplisit (scope openid/email/profile)
- `grant_types` WAJIB eksplisit: `authorization_code` + `refresh_token`

**Reference:** authentik-setup.md § Provider Stalwart Mail,
TROUBLESHOOTING.md §5 (grant_types kosong)

## 2. Password & Secret Management

### 2.1 Format
- Password CLI/shell: hex — `openssl rand -hex 32`
- Secret key app: base64 — `openssl rand -base64 60 | tr -d '\n'`
- JANGAN base64 untuk password (karakter `+ / =` memecah shell/compose)

**Reference:** SECRET-GENERATION.md Kategori 1

### 2.2 Chicken-and-Egg
- 5 var [LATE]: `AUTHENTIK_API_TOKEN`,
  `AUTHENTIK_TEMPLATE_CLIENT_SECRET`, `STALWART_API_KEY`,
  `JMAPWEBMAIL_CLIENT_ID/CLIENT_SECRET`
- Diisi bertahap setelah service terkait jalan
- Detail urutan: SECRET-GENERATION.md § Urutan Pengisian .env

### 2.3 Rotasi
- Token API: 90 hari (default Authentik; PATCH tidak bisa perpanjang —
  prosedur rotate 8 langkah di authentik-setup.md)
- Jangan no-expiry

## 3. Troubleshooting Pattern

### 3.1 Error "Authentication Failed" (webmail)
Cek berurutan:
1. CORS header reflect origin (nginx mail.conf `add_header`)
2. Redirect URI regex match
3. `OAUTH_SCOPES`/property_mappings ada
4. Trailing slash benar (lihat §1.1)
5. Stalwart `directoryId` di-set ke OIDC
6. Stalwart `Authentication` accept token

**Reference:** TROUBLESHOOTING.md § SSO webmail, jmap-webmail-notes.md

### 3.2 Log Debug
- Stalwart v0.16 log ke FILE di `/var/log/stalwart/*.log` (volume)
- `docker logs cloudsuite-stalwart` KOSONG — by design, jangan panik

**Reference:** stalwart-notes.md (table Docker logs kosong)

## 4. Konfigurasi Jaringan

### 4.1 Nginx
- Origin cert (Cloudflare) untuk vhost HTTPS web
- Let's Encrypt (via Stalwart ACME DNS-01) untuk port mail (993/465)
  — cert `mail.pem`/`mail-key.pem` di `infra/nginx/certs/` [TBC: renew
  procedure copy dari Stalwart internal store]
- CORS headers di `mail.conf` (`add_header Access-Control-*`) —
  BUKAN CF Transform Rule (tidak dipakai di staging)

### 4.2 Cloudflare
- A record: Proxied (web: auth/drive/erp/webmail/portal) atau
  DNS-only (mail — wajib untuk MX/SMTP direct)
- DNS records mail lengkap (SPF/DKIM/DMARC/MTA-STS/TLSRPT):
  stalwart-notes.md

## 5. Urutan Deploy (chicken-and-egg chart)

```
VPS base → user/SSH → git clone → folders
   → .env Kategori 1 (offline secrets)
   → Cloudflare DNS + origin cert
   → postgres + redis (compose)
   → authentik ──→ [LATE] AUTHENTIK_API_TOKEN, TEMPLATE_CLIENT_SECRET
   → nextcloud, odoo
   → stalwart ──→ [LATE] STALWART_API_KEY
   → provider stalwart-mail ──→ [LATE] JMAPWEBMAIL_CLIENT_ID/SECRET
   → jmap-webmail → final verify
```

Detail: DEPLOY-FROM-SCRATCH.md + SECRET-GENERATION.md

## 6. Referensi
- DEPLOYMENT.md — panduan langkah detail per service
- DEPLOY-FROM-SCRATCH.md — panduan master urutan deploy
- DEPLOY-CHECKLIST.md — checklist verifikasi akhir
- TROUBLESHOOTING.md — bug + fix per kasus
- SECRET-GENERATION.md — generate + rotasi secret
- Per-service notes: authentik-setup.md, nextcloud-notes.md,
  odoo-notes.md, stalwart-notes.md, jmap-webmail-notes.md
