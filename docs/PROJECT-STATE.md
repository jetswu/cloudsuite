# CloudSuite — Project State

> Snapshot status project. Update setiap sprint selesai.
> Terakhir update: 2026-09-10 (Sprint 0.9f).

## 1. Overview

CloudSuite adalah suite layanan bisnis self-hosted (Mail, Drive, ERP, Portal, IdP)
untuk kebutuhan internal. Di-build sebagai satu stack Docker Compose dengan SSO
terpusat (Authentik) dan reverse proxy Nginx. Tiap layanan mandiri tapi berbagi
infra yang sama: postgres, redis, dan jaringan `cloudsuite-net`.

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
- [ ] 0.10 — Stalwart + Bulwark + SSO
- [ ] 1 — Portal CloudSuite

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
| Stalwart + Bulwark (Mail) | mail.idchsuite.my.id | TODO | ⬜ |
| Portal CloudSuite | — | TODO | ⬜ |

## 5. Kontainer

7 container healthy:

- cloudsuite-postgres
- cloudsuite-redis
- cloudsuite-nginx
- cloudsuite-authentik-server
- cloudsuite-authentik-worker
- cloudsuite-nextcloud
- cloudsuite-odoo

## 6. Next Action

- Sprint 0.10 — Deploy Stalwart + Bulwark + SSO
- Sprint 1 — Portal CloudSuite

## 7. Referensi

- docs/DEPLOYMENT.md — panduan deploy
- docs/TROUBLESHOOTING.md — 22 learning bug
- docs/authentik-setup.md — baseline Authentik
- docs/nextcloud-notes.md — notes Nextcloud
- docs/odoo-notes.md — notes Odoo
