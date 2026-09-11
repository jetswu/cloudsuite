# Deploy Checklist — CloudSuite

Checklist verifikasi akhir deploy (dari VPS kosong). Jalankan berurutan.
Semua [ ] harus ✅ sebelum deploy dinyatakan selesai.

## A. DNS & Network

- [ ] A record web (auth/drive/erp/webmail/portal) → Proxied
      `dig +short auth.<domain>`
- [ ] A record mail → DNS-only
      `dig +short mail.<domain>` (harus IP VPS langsung, bukan CF)
- [ ] MX record → mail.<domain>
- [ ] DNS mail lengkap (SPF/DKIM/DMARC/MTA-STS/TLSRPT)
      — daftar record: stalwart-notes.md

## B. Containers

- [ ] 9 container Up + healthy:
      `docker ps --format '{{.Names}}\t{{.Status}}'`
      Ekspektasi: postgres, redis, authentik-server, authentik-worker,
      nextcloud, nginx, odoo, stalwart, jmapwebmail — semua `(healthy)`

## C. HTTP Endpoints

- [ ] `curl -sI https://auth.<domain>` → 200/3xx
- [ ] `curl -sI https://drive.<domain>` → 200/3xx
- [ ] `curl -sI https://erp.<domain>/web` → 200
- [ ] `curl -sI https://webmail.<domain>` → 200
- [ ] `curl -s https://mail.<domain>/.well-known/mta-sts.txt` → policy

## D. Mail TLS

- [ ] IMAPS (993):
      `openssl s_client -connect mail.<domain>:993 </dev/null 2>/dev/null | grep VERIFY`
      → `Verify return code: 0 (ok)`
- [ ] Submission SMTPS (465):
      `openssl s_client -connect mail.<domain>:465 </dev/null 2>/dev/null | grep VERIFY`
      → `Verify return code: 0 (ok)`
- [ ] Cert mail = Let's Encrypt (bukan self-signed)
      [TBC: verifikasi path cert 993/465 — staging internal, lihat LESSONS-LEARNED §4.1]

## E. SSO (login manual browser)

- [ ] Webmail: buka `https://webmail.<domain>` → redirect ke
      auth.<domain> → login → kembali ke inbox
- [ ] Nextcloud SSO: login via Authentik
- [ ] Odoo SSO: login via Authentik (bukan Portal user —
      TROUBLESHOOTING.md §8)

## F. Mail Function

- [ ] Kirim test email keluar (webmail compose)
- [ ] Terima test email dari luar
- [ ] Spam check: kirim ke Gmail → tidak masuk spam [TBC: tergantung
      reputasi IP baru; DKIM/SPF/DMARC harus pass — check via
      mail-tester.com]

## G. Management Access

- [ ] `STALWART_API_KEY` valid:
      `docker exec` / CLI `--api-key` → command `get account` jalan
      (contoh command: stalwart-notes.md § CLI)
- [ ] Authentik admin login (akadmin)
- [ ] Recovery admin Stalwart tersimpan di password manager
      (`STALWART_RECOVERY_ADMIN`)

## H. Security

- [ ] `.env` permission 600, owner benar
      `ls -l /opt/cloudsuite/.env`
- [ ] Tidak ada secret di repo:
      `cd ~/cloudsuite && git ls-files | grep -E '^\.env'`
      → hanya `.env.example`
- [ ] Secrets terpisah (/opt/cloudsuite/secrets/, chmod 600)
- [ ] Semua secret tercatat di password manager (tanggal generate +
      expire)

## I. Ops

- [ ] Calendar reminder rotate AUTHENTIK_API_TOKEN (90 hari)
- [ ] LE cert mail auto-renew via Stalwart ACME — cek:
      [TBC: command cek renewal Stalwart internal ACME]
- [ ] Backup: [TBC — prosedur TODO, DEPLOYMENT.md §14]

## Pass Criteria

Semua section A–H ✅ (I boleh [TBC] dengan catatan) → deploy SELESAI.
Ada ❌ → cek TROUBLESHOOTING.md, buka LESSONS-LEARNED.md §3 pola debug.
