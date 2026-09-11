# Dokumentasi Sprint 0.10a — Stalwart Mail Server

## Alur Deploy (paling aman)

1. **recovery mode** → bootstrap DataStore + apply NDJSON dasar
2. **stop recovery** → **start normal** (listener default 7 masuk otomatis)
3. **normal mode** → apply NDJSON tambahan (listener tambahan, task)
4. **restart** setelah apply listener baru (Stalwart tidak hot-reload listener)

## Struktur Direktori

```
/opt/cloudsuite/
├── data/stalwart/
│   ├── config.json          # 226B, DataStore only, 2000:2000 mode 600
│   └── lib/                 # data PostgreSQL-volume (jangan sentuh)
├── infra/docker/
│   ├── docker-compose.yml   # + backup .bak-{sprint}
│   └── stalwart-config.example.json
├── infra/nginx/conf.d/
│   └── mail.conf            # 443 ssl proxy
├── .env                     # 12 secrets
└── ...
```

## config.json

Hanya DataStore. JANGAN isi Bootstrap (acme, certificates, etc) — Stalwart tolak.

```json
{
  "data": {
    "@type": "Local",
    "path": "/var/lib/stalwart",
    "purge": { "@type": "Never" }
  },
  "registry": {
    "@type": "Local",
    "configKey": "STALWART_LICENSE"
  },
  "tracelogging": { "@type": "Server" }
}
```

## Object provisioning (apply NDJSON)

Apply via `stalwart-cli --url http://<container-ip>:8080 --user admin --password <pw> apply`.

| Object | Keterangan |
|--------|------------|
| `DnsServer` | Cloudflare via env `CLOUDFLARE_API_TOKEN` |
| `AcmeProvider` | DNS-01 Let's Encrypt |
| `Domain` | `idchsuite.my.id`, `dnsManagement.auto=true` |
| `Account` | admin@ + test@ (password via env hex-32) |
| `DkimSignature` | Dibuat otomatis saat server normal (background task) |
| `NetworkListener` | 7 default (25/465/993/995/443/8080/4190) + 2 tambahan (587/143) |

## Field Reference

| Object | Field | Tipe | Notes |
|--------|-------|------|-------|
| Account | `authUsername` | string | BUKAN `user` |
| Account | `authSecret` | `{"@type":"EnvironmentVariable","variableName":"..."}` | Jangan hardcode |
| All | `useTls`/`tlsImplicit` | bool | STARTTLS = `useTls:true, tlsImplicit:false` |
| Duration | integer | milidik | bukan detik! |
| Map | set | `{"member":true}` | bukan list |
| List | array | `{"0":{...},"1":{...}}` | key string |

## Health Check

| Endpoint | Code | Keterangan |
|----------|------|------------|
| `/healthz/live` | 200 | dipakai docker healthcheck |
| `/healthz/ready` | 200 | tersedia |
| `/health`, `/`, `/readiness` | 302 | redirect |
| `/healthz`, `/api/health` | 404 | not found |

## Listener Defaults vs Tambahan

| Port | Protokol | TLS | Status |
|------|----------|-----|--------|
| 25 | smtp | STARTTLS | default |
| 465 | smtp | implicit TLS | default |
| 587 | smtp | STARTTLS | **tambahan** |
| 143 | imap | STARTTLS | **tambahan** |
| 993 | imap | implicit TLS | default |
| 995 | pop3 | implicit TLS | default |
| 443 | http | implicit TLS | default (JMAP) |
| 8080 | http | none | default (admin) |

Listener tambahan perlu **restart container** setelah apply (Stalwart tidak hot-reload).

## DKIM

- Dual: RSA-SHA256 + Ed25519-SHA256
- Selector format: `<algo>-<YYYYMMDD>`
- Default stage: `pending` → publish via Task `DnsManagement`
- Publish auto ke Cloudflare (dnsManagement.auto=true)
- Zone file tersedia di `Domain.dnsZoneFile` (read-only)


## DKIM Signing — Root Cause & Fix

### Problem
DKIM public keys ter-publish ke DNS (via Cloudflare API), tapi outgoing email TIDAK punya header `DKIM-Signature`. SPF/DMARC pass (karena SPF align), tapi DKIM gagal karena tidak ada signature sama sekali.

### Root Cause
Dua issue:
1. **`dkimManagement` tidak di-set di Domain** — field ini required untuk mengaktifkan DKIM signing. Tanpa ini, server tidak sign outgoing email.
2. **DKIM signature stage = `pending`** — keys di-generate dengan stage `pending` dan tidak pernah di-transition ke `active`. Signing hanya aktif saat stage = `active`.

### Fix
```bash
# 1. Set dkimManagement ke Automatic
cat > /tmp/dkim-fix.ndjson << EOF
{"@type":"update","object":"Domain","id":"b","value":{"dkimManagement":{"@type":"Automatic"}}}
EOF

# 2. Update stage ke active (2 keys)
cat > /tmp/dkim-stage-fix.ndjson << EOF
{"@type":"update","object":"DkimSignature","id":"jeapkzynksaa","value":{"stage":"active"}}
{"@type":"update","object":"DkimSignature","id":"jeaphbjpkrqa","value":{"stage":"active"}}
EOF

# 3. Apply
stalwart-cli apply --file /tmp/dkim-fix.ndjson
stalwart-cli apply --file /tmp/dkim-stage-fix.ndjson

# 4. Restart (perlu untuk dkimManagement take effect)
cd /opt/cloudsuite/infra/docker
docker compose --env-file /opt/cloudsuite/.env restart stalwart
```

### Verifikasi
- Internal: kirim test@ → admin@ via 465, cek `DKIM-Signature` header ada
- External: kirim admin@ → Gmail, cek `DKIM-Signature` header ada + `dkim=pass` di Gmail header

### Field Reference
| Object | Field | Nilai | Notes |
|--------|-------|-------|-------|
| Domain | `dkimManagement` | `{"@type":"Automatic"}` | Required untuk signing |
| DkimSignature | `stage` | `active` | `pending` = tidak sign |
| SenderAuth | `dkimSignDomain` | expression | Default: sign kalau local domain + authenticated |

### Behavior
- `dkimManagement.Automatic` → server auto-generate keys, auto-rotate, auto-publish DNS
- `dkimManagement.Manual` → manual key management
- `stage: pending` → key generated tapi belum aktif untuk signing
- `stage: active` → key aktif, signing jalan
- `SenderAuth.dkimSignDomain` → expression yang menentukan kapan sign (default: `is_local_domain(sender_domain) && !is_empty(authenticated_as)`)


## DNS Record (idchsuite.my.id)

```
; MX
idchsuite.my.id. IN MX 10 mail.idchsuite.my.id.
; SPF
idchsuite.my.id. IN TXT "v=spf1 mx -all"
mail.idchsuite.my.id. IN TXT "v=spf1 a -all"
; DKIM
v1-rsa-20260910._domainkey.idchsuite.my.id. IN TXT "v=DKIM1; k=rsa; h=sha256; p=<key>"
v1-ed25519-20260910._domainkey.idchsuite.my.id. IN TXT "v=DKIM1; k=ed25519; h=sha256; p=<key>"
; DMARC
_dmarc.idchsuite.my.id. IN TXT "v=DMARC1; p=reject; rua=mailto:postmaster@idchsuite.my.id"
; SRV
_submissions._tcp.idchsuite.my.id. IN SRV 0 1 465 mail.idchsuite.my.id.
_imaps._tcp.idchsuite.my.id. IN SRV 0 1 993 mail.idchsuite.my.id.
_pop3s._tcp.idchsuite.my.id. IN SRV 0 1 995 mail.idchsuite.my.id.
_jmap._tcp.idchsuite.my.id. IN SRV 0 1 443 mail.idchsuite.my.id.
; MTA-STS
_mta-sts.idchsuite.my.id. IN TXT "v=STSv1; id=<id>"
mta-sts.idchsuite.my.id. IN CNAME mail.idchsuite.my.id.
; TLSRPT
_smtp._tls.idchsuite.my.id. IN TXT "v=TLSRPTv1; rua=mailto:postmaster@idchsuite.my.id"
; CAA
idchsuite.my.id. IN CAA 0 issue "letsencrypt.org; accounturi=<acme-id>"
idchsuite.my.id. IN CAA 0 iodef "mailto:postmaster@idchsuite.my.id"
; Autoconfig
autoconfig.idchsuite.my.id. IN CNAME mail.idchsuite.my.id.
autodiscover.idchsuite.my.id. IN CNAME mail.idchsuite.my.id.
```

## Port yang Di-expose (docker-compose ports)

```
25:25     smtp
465:465   smtps (implicit TLS)
587:587   submission (STARTTLS)
143:143   imap (STARTTLS)
993:993   imaps (implicit TLS)
995:995   pop3s (implicit TLS)
```

## Cara Trigger UpdateDnsRecords

Task `DnsManagement` auto-scheduler dijalankan Stalwart. Jika perlu manual trigger:

```bash
stalwart-cli apply --file task-dns.ndjson
```

Task akan muncul di `query Task` lalu hilang setelah selesai (scheduler hapus task setelah eksekusi). Status: `pending` → `running` → `completed` (task dihapus).

## Cara Restart Container

```bash
cd /opt/cloudsuite/infra/docker
docker compose --env-file /opt/cloudsuite/.env restart stalwart
# tunggu healthy
docker inspect cloudsuite-stalwart --format "{{.State.Health.Status}}"
```

## Troubleshooting

| Problem | Solusi |
|---------|--------|
| `config.json` tidak load | Cek JSON valid, 2000:2000 mode 600, isi DataStore only |
| Listener tidak muncul setelah apply | Restart container (tidak hot-reload) |
| DKIM masih `pending` | Trigger Task `DnsManagement` atau manual set `stage: active` via stalwart-cli apply |
| DKIM keys ada tapi signing off | Cek `dkimManagement` di Domain (harus `Automatic`) + stage harus `active` + restart container |
| Task hilang dari query | Normal — scheduler hapus task setelah selesai |
| DB field error | Cek `describe <Object>` — banyak field tidak obvious (`authUsername` bukan `user`) |
| Secret di NDJSON | Selalu `{"@type":"EnvironmentVariable","variableName":"..."}` — jangan hardcode |
| Docker logs kosong | Stalwart v0.16 log ke file /var/log/stalwart/ (belum dikonfigurasi) |
| Netstat/ss tidak tersedia | Cek via `/proc/net/tcp` (hex, little-endian) atau `docker exec` |

## Backup & Recovery

- Backup docker-compose: `docker-compose.yml.bak-{sprint}`
- Data volume: `/opt/cloudsuite/data/stalwart/lib/` (PostgreSQL)
- Config: `config.json` (simpan di repo sebagai `.example.json`, tanpa secret)
- Recovery mode ulang: butuh fresh bootstrap dari awal (drop DB + volume)

## Sprint 0.10a Status

- ✅ Stalwart v0.16.21 healthy
- ✅ ACME DNS-01 (Let's Encrypt wildcard)
- ✅ 9 listener (7 default + 587 + 143)
- ✅ DKIM dual (RSA + Ed25519) published ke Cloudflare
- ✅ DNS records: MX, SPF, DKIM, DMARC, SRV, MTA-STS, TLSRPT, CAA, autoconfig
- ✅ Mail flow: internal test OK (test@ → admin@)
- ✅ Mail flow: external relay OK (admin@ → Gmail)

## Sprint 0.10a-bis Status

- ✅ DKIM signing aktif (dkimManagement=Automatic, stage=active)
- ✅ DKIM-Signature header muncul di outgoing email
- ✅ External test ke Gmail: DKIM-Signature present

## Password admin via CLI (Sprint 0.10b)

Set password admin permanen di recovery mode TIDAK bisa lewat `AccountPassword`
(singleton, hanya untuk CHANGE password yang sudah ada). Cara yang benar: tulis
`credentials` langsung di Account via `apply` NDJSON.

- `credentials` adalah `list<Credential>` — di NDJSON pakai object map dengan key
  integer string (`"0"`, `"1"`, ...), BUKAN array.
- `secret` menerima plaintext (verifikasi pakai compare literal bila tanpa prefix
  hash `$`/`_`/`{`). Password random hex aman.
- `credentialId` dan `createdAt` server-set — JANGAN tulis.
- `roles` = tagged enum, format `{"@type":"Admin"}` (BUKAN `{"admin":true}`).

```ndjson
{"@type":"upsert","object":"Account","matchOn":["name"],"value":{"adm":{"@type":"User","name":"admin","domainId":"<domain-id>","description":"System Administrator","roles":{"@type":"Admin"},"credentials":{"0":{"@type":"Password","secret":"<password>"}}}}}}
```

Verifikasi login: `stalwart-cli --user admin@<domain> --password <pass> query Account` → exit 0.
Catatan: secret tersimpan plaintext di path direct-write; rotate via WebUI setelah
akses pulih agar ter-hash argon2id.


## Recovery mode procedure (Sprint 0.10b)

Prosedur lengkap memulihkan akses admin Stalwart saat password hilang / DB
di-wipe. Berlaku untuk chicken-and-egg: tanpa recovery admin, semua API return
401 dan tidak ada credential valid untuk mengakses.

### CLI yang benar

Binary CLI ada di **host** (`/tmp/stalwart-cli-x86_64-unknown-linux-gnu/stalwart-cli`,
versi 1.0.12), BUKAN di dalam container. Akses via IP container (bukan localhost):

```bash
IP=$(docker inspect cloudsuite-stalwart --format '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}')
CLI=/tmp/stalwart-cli-x86_64-unknown-linux-gnu/stalwart-cli
$CLI --url http://$IP:8080 --user <user> --password <pass> <command>
```

Subcommand untuk apply NDJSON = `apply` (BUKAN `import`). Plan file NDJSON
(1 objek JSON per baris), via `--file` atau `--stdin`. Tambah `--json` untuk
output NDJSON per operasi, `--dry-run` untuk validasi tanpa eksekusi.

### Langkah recovery

1. **Backup DB** (WAJIB sebelum apa pun):
   ```bash
   docker exec cloudsuite-postgres pg_dump -U cloudsuite -d stalwart > /tmp/stalwart_backup_$(date +%Y%m%d_%H%M%S).sql
   ```

2. **Masuk recovery mode** — set env lalu restart:
   ```
   STALWART_RECOVERY_ADMIN=admin:<pass>
   ```
   Catatan: `STALWART_ADMIN_PASSWORD` normal = SHA-256 hash, TIDAK bisa login
   web admin.

3. **Provision Domain + Account** (jika hilang) — `apply` NDJSON upsert.

4. **Set admin password** — lihat seksi "Password admin via CLI" di atas
   (credentials object map integer key, secret plaintext,
   roles `{"@type":"Admin"}`).

5. **Restore config OIDC** — upsert Directory (issuerUrl trailing slash),
   Authentication (`directoryId` = ID Directory), SystemSettings
   (`defaultHostname`, `defaultDomainId`).

6. **Keluar recovery mode** — hapus `STALWART_RECOVERY_ADMIN`, restart, pastikan
   normal mode.

7. **Verifikasi** — CLI login exit 0 + WebUI `curl ... /admin` → 302→200.

### Gotchas

- `directoryId` di Authentication WAJIB di-set ke Directory ID; null = SSO gagal
  silent.
- `credentialId` / `createdAt` server-set — jangan tulis di plan.
- `roles` tagged enum: `{"@type":"Admin"}`, bukan `{"admin":true}`.
- `issuerUrl` WAJIB trailing slash (otoritatif `.well-known`).

## Sprint 0.10e — SSO Webmail Final + Follow-up Keamanan (2026-09-12)

### Status SSO
- SSO webmail (jmap-webmail → Authentik → Stalwart) **BERHASIL end-to-end**.
  Bukti: login admin@idchsuite.my.id via Authentik → inbox tampil, console 0 error.
- Root cause 401 terakhir: `Authentication.directoryId` belum di-set ke Directory OIDC
  (Authentik). Setelah di-set, SSO jalan.
- Catatan: ada delay propagasi config — percobaan pertama pasca-set gagal, sukses
  setelah ±15 menit (kemungkinan cache JWKS/directory; container restart 23:41 WIB
  juga bertepatan).

### Konsekuensi directoryId=OIDC (PENTING)
- Basic auth admin (CLI `-u` dan Webadmin UI) **401 PUTUS** — semua login password
  diarahkan ke Directory OIDC.
- Jalur manajemen yang tersisa:
  1. **ApiKey CLI** (`stalwart-cli --api-key`) — utama
  2. Recovery mode (`STALWART_RECOVERY_ADMIN`) — terakhir
- CLI `query Log` via ApiKey mengembalikan 0 bytes (tidak didukung; hanya via Basic).

### Lokasi Secrets (permanen)
- `/opt/cloudsuite/secrets/` (mode 700, owner hermes — root chown tidak tersedia
  tanpa sudo; deviasi dari rencana, tercatat)
  - `stalwart-apikey.txt` (600) — ApiKey manajemen Stalwart; juga di `.env`
    sebagai `STALWART_API_KEY`
  - `cloudflare-cf.ini` (600) — CF API token utk certbot DNS-01; token terverifikasi
    valid via API verify (`success: true`)
- Semua file plaintext sensitif di `/tmp` sudah di-`shred -u -z` (apikey, cf.ini,
  password admin/recovery/Authentik, ndjson apply, jmap-proxy.log berisi Bearer
  ApiKey live, admin-setup.json, odoo_conf.txt, research_output.txt, dll).

### Investigasi Restart Container (belum terpecahkan)
- `cloudsuite-stalwart` restart pada 2026-09-11T16:41:05Z (23:41 WIB), ExitCode=0
  (graceful), OOMKilled=false, RestartCount=6, policy unless-stopped.
- docker events buffer tidak menyimpan event historis → penyebab (manual/compose)
  **belum terverifikasi**. Tidak bukti OOM (memori normal 95MB/1GiB).
- Restart ini bertepatan dengan SSO mulai jalan — kemungkinan reload config.

### Follow-up Tersisa
- Port 993/465 mungkin masih pakai origin.pem (belum diverifikasi).
- LE cert renewal ~2026-11-10 (mail.pem berlaku s/d 2026-12-10).
- Credential rotation (Authentik token, CF token, JMAPWEBMAIL secret) — belum.
- CORS warning Authentik discovery (cosmetic).
