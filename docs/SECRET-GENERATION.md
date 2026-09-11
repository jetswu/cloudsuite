# Secret Generation Guide

Panduan cara generate semua secret CloudSuite, urut berdasarkan
dependency (chicken-and-egg). Referensi template var: `.env.example`.

## Kategori Secret

### Kategori 1 — Generate Sebelum Deploy (offline)

Secret yang bisa di-generate kapan saja, tidak butuh service jalan.

| Var | Cara Generate | Format |
|---|---|---|
| POSTGRES_PASSWORD | `openssl rand -hex 32` | 64 hex |
| REDIS_PASSWORD | `openssl rand -hex 32` | 64 hex |
| AUTHENTIK_SECRET_KEY | `openssl rand -base64 60 \| tr -d '\n'` | 80 base64 |
| AUTHENTIK_ADMIN_PASSWORD | `openssl rand -hex 32` | 64 hex |
| ODOO_ADMIN_PASSWORD | `openssl rand -hex 32` | 64 hex |
| ODOO_MASTER_PASSWORD | `openssl rand -hex 32` | 64 hex |
| STALWART_ADMIN_PASSWORD | `openssl rand -hex 32` | 64 hex |
| STALWART_TEST_PASSWORD | `openssl rand -hex 32` | 64 hex |
| NEXTCLOUD_ADMIN_PASSWORD | `openssl rand -hex 32` | 64 hex |
| NEXTCLOUD_DB_PASSWORD / ODOO_DB_PASSWORD / STALWART_DB_PASSWORD | `openssl rand -hex 32` | 64 hex |

### Kategori 2 — Generate Setelah Service Jalan [LATE]

#### CLOUDFLARE_API_TOKEN
1. Buka https://dash.cloudflare.com/profile/api-tokens
2. Create Token → Custom Token
3. Permissions: Zone → DNS → Edit
4. Zone Resources: Include → Specific zone → `<domain>`
5. Continue to summary → Create Token
6. Copy token → simpan ke `.env`

Scope: minimal `Zone:DNS:Edit` untuk 1 domain. Dipakai Stalwart ACME DNS-01
dan manajemen DNS record programatik.

#### AUTHENTIK_API_TOKEN
1. Login ke `https://auth.<domain>` (akadmin)
2. Buka Admin Interface → Directory → Tokens
3. Create → Identifier: `hermes-automation`
4. User: akadmin
5. Intent: API
6. Expiring: 90 hari (default)
7. Save → view_key → copy ke `.env` `AUTHENTIK_API_TOKEN`

Catatan: expire 90 hari → calendar reminder untuk rotate.

#### AUTHENTIK_TEMPLATE_CLIENT_SECRET
1. Login Authentik → Applications → Providers
2. Buka provider `template-cloudsuite-services`
3. Tab Client Secret → lihat/copy
4. Simpan ke `.env` `AUTHENTIK_TEMPLATE_CLIENT_SECRET`

#### STALWART_API_KEY
Setelah Stalwart jalan + recovery admin di-set:

    # Login sebagai recovery admin, buat ApiKey
    IP=$(docker inspect cloudsuite-stalwart --format '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}')
    /tmp/stalwart-cli-x86_64-unknown-linux-gnu/stalwart-cli \
      --url http://$IP:8080 \
      --user admin --password <RECOVERY_PASS> \
      create ApiKey --field description=cloudsuite-admin
    # Copy key → simpan ke .env STALWART_API_KEY
    # Simpan juga ke /opt/cloudsuite/secrets/stalwart-apikey.txt (chmod 600)

⚠️ Setelah `Authentication.directoryId` di-set ke directory OIDC,
login `--user admin --password` (Basic) menghasilkan 401 — gunakan
recovery admin (`STALWART_RECOVERY_ADMIN`, lihat TROUBLESHOOTING.md
recovery mode). Setelah ApiKey ada, semua CLI pakai
`--api-key "$STALWART_API_KEY"`.

#### JMAPWEBMAIL_CLIENT_ID & JMAPWEBMAIL_CLIENT_SECRET
Setelah Authentik provider `stalwart-mail` dibuat (lihat
`authentik-setup.md` § Provider Stalwart Mail):

    source .env
    curl -s -H "Authorization: Bearer $AUTHENTIK_API_TOKEN" \
      "https://auth.<domain>/api/v3/providers/oauth2/6/" \
      | jq '{client_id, client_secret}'
    # Copy ke .env

Catatan: `6` = PK provider stalwart-mail (staging). Sesuaikan PK di
instansi lain. Client ID custom `stalwart-mail` (match Stalwart
Directory `requireAudience`).

### Kategori 3 — Generate Otomatis oleh Service
Beberapa secret di-generate otomatis oleh service saat first-run:
- Nextcloud admin password → set via `NEXTCLOUD_ADMIN_PASSWORD` env
- Odoo admin password → set via `ODOO_ADMIN_PASSWORD` env
- Stalwart DKIM keys → auto-generate (publish via `dnsZoneFile`)

## Urutan Pengisian .env (Chicken-and-Egg)

1. **Before deploy:** isi Kategori 1 (offline secrets)
2. **Deploy infrastructure:** postgres, redis, nginx, authentik
3. **After Authentik:** isi `CLOUDFLARE_API_TOKEN` (kapan saja),
   `AUTHENTIK_API_TOKEN`, `AUTHENTIK_TEMPLATE_CLIENT_SECRET`
4. **Deploy Nextcloud/Odoo/Stalwart**
5. **After Stalwart:** isi `STALWART_API_KEY`
6. **After Authentik provider:** isi `JMAPWEBMAIL_CLIENT_ID/SECRET`
7. **Deploy jmap-webmail**

## Simpan di Password Manager
Semua secret harus tersimpan di password manager (Bitwarden, 1Password,
atau sejenis). Catat juga:
- Tanggal generate
- Tanggal expire (untuk token)
- Prosedur rotate

## Aturan
1. JANGAN commit `.env` ke git (hanya `.env.example`)
2. JANGAN share secret di chat/email/laporan
3. Rotate token sebelum expired
4. Backup `.env` secara terpisah (encrypted)
