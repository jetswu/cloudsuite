# PRD CloudSuite — Final v1.4

**Produk:** CloudSuite
**Pemilik:** PT IDCloudHost
**Versi:** 1.4
**Tanggal:** 10 September 2026
**Status:** Final

## 1. Ringkasan Eksekutif

CloudSuite adalah platform PaaS Workspace dengan Single Sign-On (SSO)
yang menyatukan tiga layanan inti dalam satu portal:
- Mail — Stalwart + Bulwark
- Drive — Nextcloud
- ERP — Odoo

User cukup login sekali ke dashboard CloudSuite, lalu akses semua
layanan tanpa login ulang.

Model bisnis: Sales-led, multi-tenant, customer wajib punya domain sendiri.
Target segmen: Sekolah, perusahaan kecil-menengah, perorangan.
Target user: 100+ di tahun pertama.
Compliance: UU PDP + ISO 27001 (mengikuti IDCloudHost).

## 2. Tujuan & Metrik Sukses

Tujuan:
1. SSO untuk Mail, Drive, ERP
2. Dashboard portal sebagai pintu masuk tunggal
3. User & group management terpusat
4. Provisioning otomatis ke layanan
5. Audit log & kontrol keamanan dasar
6. Single Logout (bertahap + fallback)

Metrik:
- Login berhasil via SSO: 95%+
- Waktu provisioning user baru: < 5 menit
- Penurunan tiket support login: 50%
- Uptime portal & SSO: 99.5%
- User aktif: naik 20% dalam 3 bulan

## 3. Persona & Segmen

Persona: End User, Admin Organisasi, Super Admin Platform, IT Support, Auditor.

Segmen: Sekolah, Perusahaan kecil-menengah, Perorangan.

## 4. Scope

MVP (Phase 1):
- Login portal + SSO ke 3 layanan
- Dashboard dengan widget ringkas + redirect
- User & group management dasar
- Integrasi OIDC Authentik
- Provisioning semi-otomatis
- Audit log dasar
- Onboarding wizard + DNS verification
- SLO maksimal + fallback timeout 30 menit
- Branding hybrid
- Notifikasi in-app + email
- Bahasa: ID + EN

Phase 2: Auto provisioning, SCIM, MFA, SLO native, Billing, ISO 27701,
integrasi CRM IDCloudHost.

Phase 3: Marketplace, Multi-region, SOC 2.

Out of Scope: Bangun mail/Drive/ERP dari nol, mobile native, AI assistant,
DSpace.

## 5. Functional Requirements

### 5.1 Identity & Access Management
- Registrasi via invitation (sales-led)
- Login email/username
- Lupa password & reset via email
- MFA: TOTP, email OTP
- Role: Super Admin, Admin Tenant, Member, Viewer, Auditor
- Session: idle 30 menit, absolute 8 jam
- Audit login
- App Password untuk mail client

### 5.2 Single Logout
MVP: SLO maksimal + fallback timeout 30 menit.
Phase 2: SLO native semua layanan.

### 5.3 Dashboard Portal (Hybrid UI)
- App launcher (Mail, Drive, ERP) dengan redirect + SSO
- Widget: Mail (unread + 5 recent), Drive (storage + 5 file),
  ERP (2-4 metrik)
- Profil user, keamanan akun
- Notifikasi in-app
- Branding hybrid (logo tenant, warna CloudSuite)

### 5.4 Admin Console
- CRUD user + bulk CSV
- CRUD group + role
- Mapping group CloudSuite → layanan
- Provisioning job monitor
- Audit log + export
- Branding

### 5.5 Onboarding & Tenant
- Sales-led onboarding (via CRM IDCloudHost)
- DNS Wizard 4 langkah: MX → SPF → DKIM → DMARC
- Multi-domain per paket

### 5.6 Service Integration

Mail (Stalwart + Bulwark):
- SSO via OIDC
- Provisioning mailbox, quota, alias
- Domain mail: SPF, DKIM, DMARC
- DNS verification wizard
- App password
- Managed service oleh CloudSuite

Drive (Nextcloud):
- SSO via user_oidc
- Provisioning user + quota
- Mapping group

ERP (Odoo):
- SSO via OCA auth_oidc
- Provisioning user + company mapping
- Role mapping ke Odoo group
- Modul sesuai paket

### 5.7 Notifikasi
In-app + email untuk: undangan, reset password, domain verified,
provisioning, login device baru.

### 5.8 Audit & Compliance
- Log login, admin action, provisioning
- Immutable
- UU PDP: consent, export data, hapus akun, breach notification 72 jam
- Data retention: soft delete 30 hari, ERP transaksi 10 tahun

## 6. Non-Functional Requirements

| Aspek | Target |
|---|---|
| Security | OWASP Top 10, enkripsi, RBAC, MFA |
| Availability | 99.5% |
| Performance | Login < 2s, dashboard < 1s |
| Scalability | Multi-tenant, shared schema |
| Backup | RPO < 24 jam, RTO < 4 jam |
| Compliance | UU PDP + ISO 27001 |
| Data Residency | Server IDCloudHost Indonesia |
| Bahasa | ID + EN |
| Mobile | Responsive web |

## 7. Keputusan yang Dikunci

### Produk & Bisnis
- Branding: Hybrid (logo tenant, warna CloudSuite)
- Kuota paket: Starter/Business/Enterprise (tabel di bawah)
- Compliance: UU PDP + ISO 27001, 27701 Phase 2
- Billing: Manual transfer dulu
- Tenant isolation: Shared schema + tenant_id
- Mail HA: Single node + backup + standby
- SLO: A1+ (maksimal + fallback timeout)
- Notifikasi: In-app + email
- Bahasa: ID + EN
- SLA: 99.5%
- Harga paket: Starter Rp 15-25rb/user/bln, Business Rp 35-50rb/user/bln
- RTO/RPO: 24 jam / 4 jam
- Multi-domain: Starter 1, Business 3, Enterprise unlimited
- Support SLA: Email 1x24 jam
- CRM: IDCloudHost existing
- Live chat: Intercom
- Data retention: soft delete 30 hari, ERP 10 tahun

### Tabel Kuota Paket
| | Starter | Business | Enterprise |
|---|---|---|---|
| Max user | 25 | 100 | Unlimited |
| Mailbox quota | 5 GB/user | 25 GB/user | 50 GB/user |
| Drive storage | 50 GB | 500 GB | 2 TB |
| Domain | 1 | 3 | Unlimited |
| App password | 3/user | 10/user | Unlimited |
| Audit log | 30 hari | 1 tahun | 2 tahun |

### Modul Odoo
- Starter: Contacts, Sales, Invoicing
- Business: + Purchase, Inventory, Accounting
- Enterprise: + pilih 3 dari HR, Project, CRM, Manufacturing, POS, Website

### Teknis
- IdP: Authentik 2026.8.2
- Reverse Proxy: Nginx
- Queue: Redis
- Mail client auth: App password dulu
- Mobile: Responsive web
- UX layanan: Hybrid (widget + redirect)
- Development: AI Hermes (DeepSeek V4 Pro) via SSH ke staging VPS

### Infrastruktur
- VPS: 1 VPS — 10 CPU, 20 GB RAM, 100 GB SSD
- Public IP: 103.117.56.35
- Private IP: 10.176.185.240
- Environment: Staging
- Domain: idchsuite.my.id (Cloudflare NS)
- SSL: Cloudflare Origin Certificate + Full (strict)
- Mail IP: Monitor reputasi rutin

### Subdomain
| Subdomain | Fungsi |
|---|---|
| idchsuite.my.id | Landing page → redirect portal |
| portal.idchsuite.my.id | Dashboard CloudSuite (Phase 2) |
| auth.idchsuite.my.id | Authentik |
| drive.idchsuite.my.id | Nextcloud |
| erp.idchsuite.my.id | Odoo |
| mail.idchsuite.my.id | Bulwark + Stalwart |
| api.idchsuite.my.id | API Portal |
| admin.idchsuite.my.id | Admin Console |

### Layanan (Final)
- Mail: Stalwart v0.16.21 + Bulwark
- Drive: Nextcloud 34.0.3-apache + user_oidc 8.11.0
- ERP: Odoo 18 + OCA auth_oidc 18.0.1.1.0.2
- Odoo template: cloudsuite_template (Internal, bukan admin)
- Odoo home action: Discuss id=109
- Stalwart TLS: ACME DNS-01 via Cloudflare
- Stalwart config: config.json = DataStore only + NDJSON apply

## 8. Roadmap

Phase 1 — Fondasi & SSO (Sprint 0.x) ✅
- 0.0-0.7: Infra + Authentik + security
- 0.8: Nextcloud + SSO
- 0.9: Odoo + SSO
- 0.9d-0.9f: Dokumentasi lengkap
- 0.10a: Stalwart + DNS
- 0.10b: Bulwark + SSO (next)

Phase 2 — Portal CloudSuite (Sprint 1.x)
- 1.0: Portal skeleton
- 1.1: Dashboard
- 1.2: Admin console
- 1.3: Onboarding wizard
- 1.4: Provisioning service
- 1.5: Widget Mail/Drive/ERP
- 1.6: Audit log + polish

Phase 3 — Production Ready (Sprint 2.x)
- Deploy prod, backup, monitoring, security, UAT

Phase 4 — Scale (Sprint 3.x)
- Auto provisioning, billing, MFA, SLO native, ISO 27701

## 9. Risiko & Mitigasi

| Risiko | Mitigasi |
|---|---|
| OIDC tiap app terbatas | oauth2-proxy / reverse proxy |
| SLO tidak konsisten | Fallback idle timeout |
| Provisioning gagal | Job queue, retry, idempotent |
| Email deliverability | SPF, DKIM, DMARC, PTR, warm-up |
| Data bocor antar tenant | tenant_id, RLS Phase 2 |
| DNS customer salah | DNS wizard + verify |
| Compliance UU PDP | Consent, DPA, data residency |

## 10. Referensi

- docs/DEPLOYMENT.md — panduan deploy
- docs/TROUBLESHOOTING.md — learning bug
- docs/authentik-setup.md — baseline Authentik
- docs/nextcloud-notes.md — notes Nextcloud
- docs/odoo-notes.md — notes Odoo
- docs/stalwart-notes.md — notes Stalwart
- docs/PROJECT-STATE.md — snapshot status
- Repo: github.com/jetswu/cloudsuite
