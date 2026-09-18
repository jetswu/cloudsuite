# CloudSuite — Project State

> Snapshot status project. Update setiap sprint selesai.
> Terakhir update: 2026-09-18 (Sprint 1.5a — Audit Log + Widget Stats).

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
- [x] 1.1 — Portal: polish UI + kartu layanan
- [x] 1.2 — Admin Console (CRUD user + group, superadmin only)
- [x] 1.2b — Admin Console full CRUD + group assignment + nested null fix
- [x] 1.2c — Admin add user + password handling (set_password terpisah)
- [x] 1.2d — Implicit consent flow OIDC (login tanpa consent screen)
- [x] 1.3 — DNS Wizard + Domain Onboarding (superadmin)
- [x] 1.4a — Provisioning Inti (queue Redis + worker + connectors Stalwart/Nextcloud/Odoo-deferred, E2E live)
- [x] 1.4b — De-provisioning + UI status/retry (guard anti re-create, destroy Stalwart via JMAP `x:Account/set`, E2E create→delete live)
- [x] 1.5a — Audit Log (append-only, API list/export CSV, UI /admin/audit) + Widget Stats dashboard (Mail/Drive/ERP, Redis cache, soft-degrade; ERP metrik adaptif base+mail)

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
| Stalwart + Bulwark (Mail) | mail.idchsuite.my.id, webmail.idchsuite.my.id | ✅ | ✅ |
| Portal CloudSuite | portal.idchsuite.my.id | ✅ (Authentik) | ✅ deploy + SSO + admin console full CRUD |

## 5. Kontainer

12 container healthy:

- cloudsuite-postgres
- cloudsuite-redis
- cloudsuite-nginx
- cloudsuite-authentik-server
- cloudsuite-authentik-worker
- cloudsuite-nextcloud
- cloudsuite-odoo
- cloudsuite-stalwart
- cloudsuite-jmapwebmail
- cloudsuite-portal-backend
- cloudsuite-portal-frontend
- cloudsuite-portal-worker

## 6. Next Action

- Sprint 1.5b — Auto-login + branding
- Sprint 1.5c — Reset password + pre-create Odoo
- Sprint 2.0 — Tenant admin UI (`/manage`, Phase 2)

## 7. Referensi

- docs/DEPLOYMENT.md — panduan deploy
- docs/TROUBLESHOOTING.md — 22 learning bug
- docs/authentik-setup.md — baseline Authentik
- docs/nextcloud-notes.md — notes Nextcloud
- docs/odoo-notes.md — notes Odoo
- docs/portal-notes.md — notes Portal (termasuk Admin Console + Provisioning 1.4a + De-provisioning 1.4b + Audit Log + Widget 1.5a)
