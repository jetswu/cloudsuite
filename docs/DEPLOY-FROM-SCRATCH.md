# Deploy CloudSuite dari VPS Kosong

File master untuk deploy ulang. High-level + referensi ke detail —
command lengkap ada di docs yang dirujuk.

**Estimasi: 4–6 jam** (1 orang, tanpa AI).

## Prerequisites
- VPS: 10 vCPU / 20 GB RAM / 100 GB disk (Ubuntu 24.04 LTS atau
  Debian 13; staging pakai Ubuntu 24.04)
- Domain di Cloudflare (NS pointing ke CF)
- Akun: Cloudflare, GitHub (akses repo `jetswu/cloudsuite`)
- Tools lokal: ssh, git, password manager

## Overview Arsitektur

```
Internet → Cloudflare (DNS/proxy) → VPS nginx (443, origin cert)
  ├── auth.<domain>    → authentik-server (IdP, OIDC)
  ├── drive.<domain>   → nextcloud (Drive)
  ├── erp.<domain>     → odoo (ERP)
  ├── webmail.<domain> → jmap-webmail → Stalwart (JMAP)
  └── mail.<domain>    → Stalwart (SMTP/IMAP 25/993/465, LE cert, DNS-only)
                        └── postgres + redis (shared, internal network)
```

Detail port/service: DEPLOYMENT.md §1 (Overview).

## Urutan Deploy (WAJIB urut)

| # | Langkah | Estimasi | Detail |
|---|---|---|---|
| 1 | Base VPS + Docker | 30 min | DEPLOYMENT.md §3 |
| 2 | User & SSH | 20 min | §4 |
| 3 | Git clone | 10 min | §5 |
| 4 | Folder structure | 5 min | §6 |
| 5 | Secrets (.env) | 20 min | §7 + SECRET-GENERATION.md |
| 6 | Cloudflare DNS + origin cert | 20 min | §2.5 + §8 |
| 7 | PostgreSQL + Redis | 15 min | §9 (pre-step) |
| 8 | Authentik | 45 min | §9 |
| 9 | Nextcloud | 30 min | §10 |
| 10 | Odoo | 45 min | §11 |
| 11 | Stalwart | 60 min | §12 |
| 12 | jmap-webmail | 30 min | §12b |
| 13 | Final verification | 30 min | DEPLOY-CHECKLIST.md |
| 14 | Backup setup | 30 min | §14 [TBC — prosedur backup masih TODO] |

## Per Langkah

### Langkah 1 — Base VPS + Docker
**Estimasi:** 30 menit
**Butuh:** akses root VPS
**Command + verifikasi:** lihat DEPLOYMENT.md §3
**Verify:**
    docker --version && docker compose version
    ufw status  # port 22/80/443/25/993/465 terbuka sesuai §3

### Langkah 2 — User & SSH
**Estimasi:** 20 menit
**Butuh:** Langkah 1
**Command + verifikasi:** lihat DEPLOYMENT.md §4
**Verify:**
    ssh <user>@<vps-ip> 'whoami'  # login tanpa password

### Langkah 3 — Git Clone
**Estimasi:** 10 menit
**Butuh:** Langkah 2, SSH key GitHub (TROUBLESHOOTING.md §12)
**Command:**
    git clone git@github.com:jetswu/cloudsuite.git ~/cloudsuite
**Verify:**
    ls ~/cloudsuite/infra/docker/docker-compose.yml

### Langkah 4 — Folder Structure
**Estimasi:** 5 menit
**Butuh:** Langkah 3
**Command + verifikasi:** lihat DEPLOYMENT.md §6 (live root
`/opt/cloudsuite`, mirror repo `~/cloudsuite`)
**Verify:**
    ls /opt/cloudsuite/infra /opt/cloudsuite/data

### Langkah 5 — Secrets (.env)
**Estimasi:** 20 menit
**Butuh:** Langkah 3–4
**Command:**
    cp ~/cloudsuite/.env.example /opt/cloudsuite/.env
    # isi Kategori 1 (offline) sekarang; var [LATE] nanti
    chmod 600 /opt/cloudsuite/.env
**Panduan:** SECRET-GENERATION.md (format, urutan chicken-and-egg)
**Verify:**
    grep -c CHANGE_ME /opt/cloudsuite/.env  # hanya var [LATE] yang tersisa

### Langkah 6 — Cloudflare DNS + Origin Cert
**Estimasi:** 20 menit
**Butuh:** akun CF, Langkah 5
**Command + verifikasi:** DEPLOYMENT.md §2.5 (DNS records, mail wajib
DNS-only) + §8 (origin cert → `infra/nginx/certs/origin.pem`)
**Verify:**
    dig +short auth.<domain>  # resolve ke IP VPS

### Langkah 7 — PostgreSQL + Redis
**Estimasi:** 15 menit
**Butuh:** Langkah 5 (.env terisi Kategori 1)
**Command:**
    cd /opt/cloudsuite/infra/docker && docker compose up -d postgres redis
**Verify:**
    docker ps --filter name=postgres --filter name=redis --format '{{.Names}} {{.Status}}'

### Langkah 8 — Authentik
**Estimasi:** 45 menit
**Butuh:** Langkah 6–7
**Command + verifikasi:** DEPLOYMENT.md §9
**Verify:**
    docker ps --filter name=authentik  # server + worker healthy
    curl -sI https://auth.<domain> | head -1
**Setelah jalan:** isi [LATE] `AUTHENTIK_API_TOKEN` +
`AUTHENTIK_TEMPLATE_CLIENT_SECRET` (SECRET-GENERATION.md Kategori 2)

### Langkah 9 — Nextcloud
**Estimasi:** 30 menit
**Butuh:** Langkah 8
**Command + verifikasi:** DEPLOYMENT.md §10
**Verify:**
    curl -sI https://drive.<domain> | head -1

### Langkah 10 — Odoo
**Estimasi:** 45 menit
**Butuh:** Langkah 8 (DB `odoo` dibuat SEBELUM start — §11)
**Command + verifikasi:** DEPLOYMENT.md §11
**Verify:**
    curl -sI https://erp.<domain>/web | head -1

### Langkah 11 — Stalwart
**Estimasi:** 60 menit
**Butuh:** Langkah 6 (DNS + CF token untuk ACME DNS-01), Langkah 7
**Command + verifikasi:** DEPLOYMENT.md §12 (config DataStore,
NDJSON apply, recovery admin)
**Verify:**
    docker ps --filter name=stalwart  # healthy
    openssl s_client -connect mail.<domain>:993 </dev/null 2>/dev/null | grep VERIFY
**Setelah jalan:** isi [LATE] `STALWART_API_KEY` (SECRET-GENERATION.md)

### Langkah 12 — jmap-webmail
**Estimasi:** 30 menit
**Butuh:** Langkah 11 + Authentik provider `stalwart-mail` dibuat
(authentik-setup.md)
**Command + verifikasi:** DEPLOYMENT.md §12b
**Verify:**
    curl -sI https://webmail.<domain> | head -1
    # SSO login test: buka URL, login via auth.<domain>, masuk inbox
**Setelah provider:** isi [LATE] `JMAPWEBMAIL_CLIENT_ID/SECRET`

### Langkah 13 — Final Verification
**Estimasi:** 30 menit
**Butuh:** semua langkah
**Command:** ikuti `docs/DEPLOY-CHECKLIST.md` baris demi baris

### Langkah 14 — Backup Setup
**Estimasi:** 30 menit
**Status:** [TBC] — prosedur backup masih TODO di DEPLOYMENT.md §14
(cakupan: DB postgres, volume `data/`, `.env`). Deploy dulu tanpa ini,
setup backup setelah stack stabil.

## Catatan Penting (baca sebelum mulai)
- Konvensi fatal (trailing slash, redirect regex, directoryId):
  LESSONS-LEARNED.md §1 — JANGAN dilanggar
- Kena masalah? TROUBLESHOOTING.md dulu, baru eksperimen
- JANGAN commit `.env` / secret apapun ke git (rule permanen)
