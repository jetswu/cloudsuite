# CloudSuite — Project State

> Snapshot status project. Update setiap sprint selesai.
> Terakhir update: 2026-09-14 (Sprint 1.0 — Portal deploy).

## 1. Overview

CloudSuite adalah suite layanan bisnis self-hosted (Mail, Drive, ERP, Portal, IdP)
untuk kebutuhan internal. Di-build sebagai satu stack Docker Compose dengan SSO
terpusat (Authentik) dan reverse proxy Nginx. Tiap layanan mandiri tapi berbagi
infra yang sama: postgres, redis, dan jaringan `cloudsuite-net`.

- **PRD:** docs/PRD.md (Final v1.4)
- **Repo:** git@github.com:jetswu/cloudsuite.git
- **Environment aktif:** staging (10 vCPU, 20 GB RAM, 100 GB disk)
- **Prod:** belum ada

## 2. Sprint Progress

- [x] 0.0 — Fondasi (postgres, redis, nginx)
- [x] 0.5 — Authentik IdP
- [x] 0.6 + 0.6b — Authentik baseline
- [x] 0.7 — Rotasi secret
- [x] 0.8 + 0.8b — Nextcloud + SSO
- [x] 0.9 + 0.9b + 0.9c — Odoo + SSO
- [x] 0.9d — Compile docs (DEPLOYMENT + TROUBLESHOOTING)
- [x] 0.9e — Perbaikan hasil review
- [x] 0.9f — Project state + odoo notes
- [x] 0.10a — Stalwart Mail Server + DNS
- [x] 0.10b — Bulwark webmail + SSO (⏳ pending DNS admin)
- [x] 1.0 — Portal CloudSuite (deploy; test SSO pending admin)

## 3. Keputusan yang Dikunci

- **IdP:** Authentik (MIT, proxy auth, unlimited)
- **Reverse Proxy:** Nginx
- **Queue:** Redis
- **Tenant isolation:** Shared schema + tenant_id
- **SLA:** 99.5%
- **Domain:** idchsuite.my.id (Cloudflare NS)
- **SSL:** Cloudflare Origin Certificate + Full (strict)
- **Bahasa:** ID + EN
- **Mobile:** responsive web (bukan native)

## 4. Layanan Aktif

| Layanan | URL | SSO | Status |
|---|---|---|---|
| Authentik (IdP) | auth.idchsuite.my.id | — | ✅ |
| Nextcloud (Drive) | drive.idchsuite.my.id | ✅ | ✅ |
| Odoo (ERP) | erp.idchsuite.my.id | ✅ | ✅ |
| Stalwart + Bulwark (Mail) | mail.idchsuite.my.id, webmail.idchsuite.my.id | ✅ | ✅
| Portal CloudSuite | portal.idchsuite.my.id | ✅ (Authentik) | ✅ deploy, ⏳ test SSO admin |

## 5. Kontainer

11 container healthy:

- cloudsuite-postgres
- cloudsuite-redis
- cloudsuite-nginx
- cloudsuite-authentik-server
- cloudsuite-authentik-worker
- cloudsuite-nextcloud
- cloudsuite-odoo
- cloudsuite-stalwart
- cloudsuite-portal-backend
- cloudsuite-portal-frontend

## 6. Next Action

- Test SSO Portal (admin) — https://portal.idchsuite.my.id
- Sprint 1.1 — Portal: polish UI + fitur kartu layanan

## 7. Referensi

- docs/DEPLOYMENT.md — panduan deploy
- docs/TROUBLESHOOTING.md — 22 learning bug
- docs/authentik-setup.md — baseline Authentik
- docs/nextcloud-notes.md — notes Nextcloud
- docs/odoo-notes.md — notes Odoo
