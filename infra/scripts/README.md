# infra/scripts/ — Script Ops CloudSuite

Script operasional live. Lokasi jalannya di VPS: `/opt/cloudsuite/infra/scripts/`
(dir ini = salinan repo untuk reproducibility — kalau VPS rusak, restore dari sini).

## Daftar Script

| Script | Fungsi | Cron |
|---|---|---|
| `check-services.sh` | Cek container 6 service (portal, authentik, nextcloud, odoo, stalwart, bulwark) jalan/tidak — output warning kalau ada yang down | belum terpasang (manual) |
| `check-storage.sh` | Cek disk usage `/` — warning kalau > 80% (alert Telegram/email = TODO) | belum terpasang (manual) |
| `reminder-mail-cert.sh` | Cek sisa hari cert LE `mail.pem` (nginx mail vhost, renewal MANUAL) — WARNING + exit 1 kalau < 30 hari | hermes VPS: `0 9 * * *` → log `/home/hermes/logs/reminder-mail-cert.log` |

## Cara Pakai

    # semua script bisa dijalankan langsung (butuh akses docker / path /opt/cloudsuite)
    /opt/cloudsuite/infra/scripts/check-services.sh
    /opt/cloudsuite/infra/scripts/check-storage.sh
    /opt/cloudsuite/infra/scripts/reminder-mail-cert.sh   # echo days_left, exit 1 kalau < 30

## Cron Terpasang (VPS, user hermes — `crontab -l`)

    0 9 * * * /opt/cloudsuite/infra/scripts/reminder-mail-cert.sh >> /home/hermes/logs/reminder-mail-cert.log 2>&1

## Catatan

- Cert port mail (993/465/995/587/143, di-serve Stalwart) = wildcard auto-renew
  via ACME — TIDAK perlu reminder. Hanya `mail.pem` (nginx) yang manual.
- Prosedur renew `mail.pem`: lihat PANDUAN-DEPLOY.md §10.9.
- Cron `crontab` hermes TIDAK ikut repo (per-user) — pasang manual saat deploy
  baru: lihat PANDUAN-DEPLOY.md.
