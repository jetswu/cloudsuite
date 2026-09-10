# Nextcloud Notes -- CloudSuite (staging)

- **URL:** https://drive.idchsuite.my.id
- **Image:** nextcloud:34.0.3-apache (stable terbaru 2026-09-10, di-pin)
- **Container:** cloudsuite-nextcloud, DB nextcloud di cloudsuite-postgres, redis session
- **SSO:** OIDC via Authentik (provider pk 2, app slug nextcloud, user_oidc 8.11.0)

## ncadmin = akun break-glass

- ncadmin (NEXTCLOUD_ADMIN_USER di /opt/cloudsuite/.env) adalah akun DARURAT.
- JANGAN dipakai user biasa / operasional harian -- login normal via SSO Authentik.
- SSO user dibuat otomatis oleh user_oidc saat pertama login (provision on-the-fly).

## Disable login lokal (opsional)

Kalau SSO stabil dan ingin menutup login password lokal:

    docker exec -u www-data cloudsuite-nextcloud php occ user:disable ncadmin

Enable balik saat emergency:

    docker exec -u www-data cloudsuite-nextcloud php occ user:enable ncadmin

Status saat ini (2026-09-10): ncadmin AKTIF (belum di-disable -- keputusan disable menunggu persetujuan).

## Healthcheck

Service nextcloud punya healthcheck: curl -f http://localhost/status.php (interval 30s, start_period 60s).
Verifikasi: docker compose ps -- nextcloud harus (healthy).
