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
| DKIM masih `pending` | Trigger Task `DnsManagement` |
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
