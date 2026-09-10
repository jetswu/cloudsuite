# CloudSuite — Panduan Deployment

> Terakhir diperbarui: 2026-09-10 (Sprint 0.9e).
> Tujuan dokumen ini: deploy ulang CloudSuite dari VPS kosong tanpa mulai dari nol.

## 1. Overview

CloudSuite adalah platform layanan internal terpadu dalam satu VPS: satu identity
provider (Authentik) sebagai pintu SSO untuk semua aplikasi — Mail (Stalwart +
Bulwark), Drive (Nextcloud), ERP (Odoo 18), dan Portal custom — semua berjalan
sebagai container Docker dalam satu network (`cloudsuite-net`) di belakang satu
reverse proxy nginx dengan TLS dari Cloudflare Origin Certificate.

Layanan (per Sprint 0.9; Mail = TODO Sprint 0.10):

| Layanan | Teknologi | URL internal |
|---|---|---|
| IdP / SSO | Authentik 2026.8.2 (server + worker) | auth.idchsuite.my.id |
| Drive | Nextcloud 34.0.3-apache + user_oidc 8.11.0 | drive.idchsuite.my.id |
| ERP | Odoo 18 + OCA auth_oidc 18.0.1.1.0.2 | erp.idchsuite.my.id |
| Mail | Stalwart + Bulwark (TODO Sprint 0.10) | — |
| Portal | custom (TODO) | — |
| DB / cache | postgres:16-alpine, redis:7-alpine | internal saja |
| Proxy | nginx:alpine | :80/:443 |

Arsitektur: semua service di 1 VPS via Docker Compose, network eksternal tunggal
`cloudsuite-net` (dibuat manual sekali via `docker network create`).
Repo: `git@github.com:jetswu/cloudsuite.git` (branch `main`).

## 2. Prasyarat

- VPS: minimum 4 vCPU / 8 GB RAM (staging ringan); rekomendasi
  10 vCPU / 20 GB RAM (seperti staging saat ini). Staging aktual:
  Debian 13 (trixie), 10 vCPU, 20 GB RAM, Docker 29.8.0. OS boleh
  Ubuntu 24.04 — [perlu verifikasi] staging berjalan di Debian 13,
  langkah apt di bawah ditulis untuk Debian/Ubuntu.
- Domain di Cloudflare: `idchsuite.my.id` plus subdomain
  `auth`, `drive`, `erp` (dan `mail` saat Sprint 0.10).
- Cloudflare SSL mode: **Full (strict)** — origin menyajikan cert Origin CA,
  nginx `ssl_certificate /etc/nginx/certs/origin.pem`.
- Tools di laptop admin: `ssh`, `git`, akses Cloudflare dashboard
  (untuk download Origin Certificate), akses GitHub repo.

## 2.5 Setup Cloudflare

1. Tambahkan A record untuk tiap subdomain web → IP VPS, mode **Proxied**
   (awan oranye):
   - `portal.idchsuite.my.id` (Portal — Sprint 0.10)
   - `auth.idchsuite.my.id` (Authentik)
   - `drive.idchsuite.my.id` (Nextcloud)
   - `erp.idchsuite.my.id` (Odoo)
   - `api.idchsuite.my.id` (API)
   - `admin.idchsuite.my.id` (Admin)
2. Subdomain `mail.*` → mode **DNS only** (awan abu-abu), karena proxy
   Cloudflare hanya untuk HTTP(S); record mail (MX/SPF/DKIM/DMARC) tidak boleh
   lewat proxy.
3. SSL/TLS mode: **Full (strict)**.
4. **Always Use HTTPS**: ON.
5. **Automatic HTTPS Rewrites**: ON.
6. Download Origin Certificate (hostnames `*.idchsuite.my.id`, validity
   maksimum) → simpan sebagai `origin.pem` + `origin-key.pem` (lihat Bagian 8).

## 3. Setup VPS

```bash
# Update + timezone Asia/Jakarta (wajib — semua container pakai TZ ini)
sudo apt update && sudo apt install -y curl git ufw fail2ban unattended-upgrades
sudo timedatectl set-timezone Asia/Jakarta

# Buat user admin (contoh: admin) + SSH key, matikan login password/root bila perlu
sudo adduser admin && sudo usermod -aG sudo admin

# UFW: buka SSH/HTTP/HTTPS + port mail (aturan persis staging [perlu verifikasi —
# status ufw staging belum sempat dibaca karena hermes tanpa sudo ufw])
sudo ufw allow 22/tcp
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
sudo ufw allow 25/tcp    # SMTP
sudo ufw allow 465/tcp   # SMTPS
sudo ufw allow 587/tcp   # SMTP submission
sudo ufw allow 993/tcp   # IMAPS
sudo ufw allow 995/tcp   # POP3S
sudo ufw enable

# Docker (repo resmi Docker)
curl -fsSL https://get.docker.com | sudo sh
sudo usermod -aG docker admin
docker network create cloudsuite-net
```

## 4. User Hermes (AI Agent)

User `hermes` adalah akun kerja AI agent: SSH key saja (tanpa password),
grup `docker` + `cloudsuite`, dan sudo NOPASSWD terbatas pada:

```
(ALL) NOPASSWD: /usr/bin/docker
(ALL) NOPASSWD: /usr/bin/docker compose *
(ALL) NOPASSWD: /usr/bin/apt update
(ALL) NOPASSWD: /usr/bin/apt install *
(ALL) NOPASSWD: /bin/systemctl restart cloudsuite-*
(ALL) NOPASSWD: /bin/systemctl status cloudsuite-*
```

(verifikasi: `sudo -l -U hermes`, output di atas dari staging 2026-09-10).
Buat dengan `adduser hermes`, pasang SSH public key ke
`~hermes/.ssh/authorized_keys`, tambahkan baris sudoers via
`sudo visudo -f /etc/sudoers.d/hermes`. Di sisi AI VPS, cukup entri
`~/.ssh/config`:

```
Host cloudsuite
    HostName <IP_VPS>
    User hermes
    IdentityFile ~/.ssh/hermes_cloudsuite
```

## 5. Git Repository

Di staging ada DUA checkout yang sinkron:

- `/home/hermes/cloudsuite` — clone kerja agent (SSH GitHub milik hermes).
- `/opt/cloudsuite` — copy deploy: file compose/config di bawah `infra/`
  mengacu path absolut `/opt/cloudsuite/...`, jadi deploy live BUKAN dari
  checkout git melainkan dari `/opt/cloudsuite` (copy manual per sprint).

```bash
# sebagai hermes
git clone git@github.com:jetswu/cloudsuite.git ~/cloudsuite
sudo mkdir -p /opt/cloudsuite && sudo chown -R admin:cloudsuite /opt/cloudsuite
# salin infra/ + file lain yang dibutuhkan ke /opt/cloudsuite per sprint
```

SSH key GitHub untuk hermes dibuat sekali (`ssh-keygen -t ed25519`),
public key didaftarkan sebagai deploy key / collaborator key di repo.

## 6. Struktur Folder

```
/opt/cloudsuite/
├── .env                      # secret live (600, hermes:hermes) + .env.bak-*
├── infra/
│   ├── docker/
│   │   ├── docker-compose.yml   # SATU-SATUNYA compose file
│   │   ├── Dockerfile.odoo      # image custom Odoo
│   │   └── odoo.conf.example    # contoh (password=CHANGE_ME)
│   └── nginx/
│       ├── conf.d/              # auth.conf drive.conf erp.conf default.conf
│       └── certs/               # origin.pem + origin-key.pem (600)
├── data/                     # volume: postgres redis authentik nextcloud odoo
├── logs/nginx/
├── docs/ connectors/ portal/ # placeholder / dokumen operasional
```

**PENTING — path compose file.** File compose HANYA ada di
`/opt/cloudsuite/infra/docker/docker-compose.yml`, BUKAN di root
`/opt/cloudsuite/`. Menjalankan `docker compose ps` dari root akan gagal
`no configuration file provided: not found`. Selalu salah satu:

```bash
cd /opt/cloudsuite/infra/docker && docker compose ps
# atau
docker compose -f /opt/cloudsuite/infra/docker/docker-compose.yml \
  --env-file /opt/cloudsuite/.env ps
```

Network `cloudsuite-net` bertipe `external: true` di compose — buat manual
sekali (`docker network create cloudsuite-net`) sebelum `up` pertama.

## 7. Environment File (.env)

- Generate secret acak: `python3 -c "import secrets; print(secrets.token_hex(32))"`
  (hex 64 char) untuk password; Authentik SECRET_KEY 60 char dari token asli
  (panjang aktual staging `TOKEN_LEN=60` — cara generate awal [perlu verifikasi]).
- Daftar key yang WAJIB ada (TANPA nilai — ambil dari staging), dikelompokkan
  per kategori:

  **Umum**
  - `DOMAIN` `PUBLIC_IP` `PRIVATE_IP` `TZ`

  **Database (PostgreSQL)**
  - `POSTGRES_DB` `POSTGRES_USER` `POSTGRES_PASSWORD`
  - `NEXTCLOUD_DB_NAME` `NEXTCLOUD_DB_USER` `NEXTCLOUD_DB_PASSWORD` `NEXTCLOUD_DB_HOST`
  - `ODOO_DB_NAME` `ODOO_DB_USER` `ODOO_DB_PASSWORD` `ODOO_DB_HOST`

  **Authentik**
  - `AUTHENTIK_SECRET_KEY` `AUTHENTIK_ADMIN_PASSWORD` `AUTHENTIK_ADMIN_EMAIL`
  - `AUTHENTIK_API_TOKEN` `AUTHENTIK_TEMPLATE_CLIENT_SECRET`

  **Nextcloud**
  - `NEXTCLOUD_ADMIN_USER` `NEXTCLOUD_ADMIN_PASSWORD`

  **Odoo**
  - `ODOO_ADMIN_PASSWORD` `ODOO_MASTER_PASSWORD`

  **Stalwart (mail — Sprint 0.10)**
  - `STALWART_ADMIN_PASSWORD`

  **Redis**
  - `REDIS_PASSWORD`
- Permission: `chmod 600 /opt/cloudsuite/.env`, owner `hermes:hermes`.
- Backup SEBELUM setiap perubahan: `cp .env .env.bak-<tanggal>` (pola yang
  dipakai staging: `.env.bak-0.9`, `.env.bak-20260910-HHMMSS`). JANGAN commit.

## 8. SSL Origin Certificate (Cloudflare)

1. Cloudflare dashboard → domain → SSL/TLS → Origin Server → Create Certificate
   (RSA, validitas maks, semua subdomain `*.idchsuite.my.id` bila didukung).
2. Simpan sebagai `/opt/cloudsuite/infra/nginx/certs/origin.pem` (cert) dan
   `origin-key.pem` (key), permission 600.
3. Semua `conf.d/*.conf` mengacu path container `/etc/nginx/certs/origin*.pem`
   (di-mount `:ro` dari folder certs). Satu pasangan cert dipakai semua vhost.
4. Cloudflare SSL mode wajib **Full (strict)** agar chain Origin CA valid.
## 9. Deploy Authentik

Jalankan dari folder compose (lihat Bagian 6 soal path):

    cd /opt/cloudsuite/infra/docker
    docker compose up -d postgres redis
    docker compose up -d authentik-server authentik-worker
    docker logs cloudsuite-authentik-server --tail 20

Pastikan log tanpa EAFNOSUPPORT.

- **IPv4 override (PENTING):** kernel staging IPv6 mati total; Authentik
  default bind `[::]:9000/9443/9300` sehingga muncul `error 97 EAFNOSUPPORT`
  (`server.rs:33`) dan restart-loop sekitar 45 detik. Fix: kedua service
  (server DAN worker) wajib punya env
  `AUTHENTIK_LISTEN__HTTP: 0.0.0.0:9000`,
  `AUTHENTIK_LISTEN__HTTPS: 0.0.0.0:9443`,
  `AUTHENTIK_LISTEN__METRICS: 0.0.0.0:9300`. Jangan hapus baris ini.
- Redis: service lain konek dengan password dari env `REDIS_PASSWORD`
  (pola env-based auth — lihat troubleshooting #2).
- Nginx: `conf.d/auth.conf` — port 80 redirect 301 ke https plus endpoint
  `/health` lokal; 443 proxy ke `http://authentik-server:9000` dengan
  lazy-resolve (`resolver 127.0.0.11`, `set $auth_upstream ...`) agar
  `nginx -t` lolos sebelum container ada.
- Setup admin: login pertama pakai `AUTHENTIK_BOOTSTRAP_EMAIL` dan
  `AUTHENTIK_BOOTSTRAP_PASSWORD`. Reset berikutnya BUKAN via env itu lagi
  (hanya berlaku first-run) melainkan via perintah `ak changepassword`
  di dalam container server (contoh persis ada di docs/authentik-setup.md).
- Baseline terdokumentasi di `docs/authentik-setup.md`: brand, groups,
  token API `hermes-automation` (intent `api`, expiry sekitar 90 hari —
  DILARANG no-expiry; rotate manual dengan revoke lalu recreate,
  `default_token_duration=days=90`), template aplikasi
  `template-cloudsuite-services`.
- Verifikasi: `curl https://auth.idchsuite.my.id/-/health/ready/` menghasilkan 200.

## 10. Deploy Nextcloud (Drive)

Jalankan dari folder compose:

    cd /opt/cloudsuite/infra/docker
    docker compose up -d nextcloud

- DB: database `nextcloud` di `cloudsuite-postgres` (kredensial
  `NEXTCLOUD_DB_*`), redis untuk session; env penting:
  `NEXTCLOUD_TRUSTED_DOMAINS=drive.idchsuite.my.id`,
  `TRUSTED_PROXIES=cloudsuite-nginx`, `OVERWRITEPROTOCOL=https`,
  `OVERWRITECLIURL=https://drive.idchsuite.my.id`.
- Nginx: `conf.d/drive.conf` — proxy ke `http://nextcloud:80`,
  `client_max_body_size 10G`, timeout 3600s, header WebSocket.
- Healthcheck compose: `curl -f http://localhost/status.php`
  (interval 30s, start_period 60s).
- SSO: app `user_oidc 8.11.0`; konfigurasi via OCC format 8.x — perintah
  `php occ user_oidc:provider` dengan flag clientid, clientsecret,
  discoveryuri, unique-uid (BUKAN format lama `config:app:set` — itu
  menghasilkan provider dengan clientId kosong). Jalankan sebagai
  `docker exec -u www-data ...`.
- Provider OIDC di Authentik (pk=2, app slug `nextcloud`, client ID boleh
  tampil di docs, secret HANYA di OCC dan .env): redirect strict
  `https://drive.idchsuite.my.id/apps/user_oidc/code`, discovery
  `https://auth.idchsuite.my.id/application/o/nextcloud/.well-known/openid-configuration`.
  `grant_types` WAJIB diisi eksplisit via API (authorization_code, hybrid,
  implicit, client_credentials, password, device_code, refresh_token) —
  default API kosong sehingga authorize gagal `invalid_request` plus log
  `Invalid grant_type` (terverifikasi dari source `views/authorize.py:233`).
- Break-glass: user `ncadmin` (`NEXTCLOUD_ADMIN_USER`) adalah akun DARURAT —
  jangan dipakai harian; login normal via SSO. Opsional disable via
  `occ user:disable ncadmin` (status staging 2026-09-10: AKTIF; keputusan
  disable menunggu persetujuan). Detail: docs/nextcloud-notes.md.
- Verifikasi: `curl https://drive.idchsuite.my.id/status.php` memuat
  `"installed":true`; test SSO manual: Login lalu redirect Authentik lalu
  consent Continue lalu dashboard sebagai user OIDC ter-provision.

## 11. Deploy Odoo (ERP)

Jalankan dari folder compose:

    cd /opt/cloudsuite/infra/docker
    docker compose up -d --build odoo

- DB: buat database `odoo` OWNER user Odoo di postgres SEBELUM start
  (staging: DB `odoo` OWNER `cloudsuite`) — image ini tidak membuat DB
  sendiri; `dbfilter=^odoo$` mengunci tepat satu DB itu.
- `Dockerfile.odoo`: `FROM odoo:18.0`, sebagai root jalankan
  `pip install --no-cache-dir --break-system-packages python-jose odoo-addon-auth-oidc==18.0.1.1.0.2`,
  lalu kembali `USER odoo` (`--break-system-packages` karena PEP 668).
- `odoo.conf` live (11 baris) di `/opt/cloudsuite/data/odoo/config/odoo.conf`:
  owner `100:101` mode 600 (UID/GID user odoo di image), `list_db=False`,
  `dbfilter=^odoo$`, `db_host=postgres`, `admin_passwd` berisi secret.
  Repo HANYA menyimpan `odoo.conf.example` (`password=CHANGE_ME`).
- Command compose: `odoo --proxy-mode --without-demo=all`; env
  `HOST=postgres PORT=5432 USER/PASSWORD` dari `ODOO_DB_*` plus `TZ`; limit
  `mem 2.5g cpus 2`; healthcheck `curl -f http://localhost:8069/web/login`
  (start_period 180s); expose 8069/8072 internal.
- Nginx: `conf.d/erp.conf` — proxy ke `http://odoo:8069`,
  `client_max_body_size 200M`, timeout 720s, header WebSocket.
- **Template user internal (PENTING):** Odoo 18 membuat user OAuth baru dari
  `base.template_portal_user_id` (BUKAN `auth_signup.default_template_user_id`
  yang bernilai False dan tidak dibaca kode). Default template = Portal
  (id=5 `portaltemplate`, share=True) sehingga user SSO baru jadi Portal lalu
  redirect nyangkut di `/web/login_successful`. Fix staging: buat user
  `cloudsuite_template` (active=True, email `template@cloudsuite.local`)
  dengan grup HANYA Internal User plus Technical Features, lalu set
  `base.template_portal_user_id` dari 5 ke 8. **JANGAN pakai admin sebagai
  template** (copy dari admin sama dengan privilege escalation).
- OAuth provider di Odoo (id=6 `CloudSuite`, flow `id_token_code`, enabled,
  scope `openid email profile`, endpoint auth/token/jwks Authentik) plus
  provider dan app di Authentik (pk=3, app slug `odoo`, redirect strict
  `https://erp.idchsuite.my.id/auth_oauth/signin`).
- Home action default: Discuss `id=109` (`mail.action_discuss`) — set
  `action_id=109` per user plus `ir.default res.users action_id=109` agar
  user baru langsung dapat (staging: user id=9 hasil SSO terverifikasi
  share=False, grup Internal, action_id=109).
- Prosedur test user baru: hapus record Odoo (`unlink` — aman, tidak hapus
  user Authentik), re-login via `Login with CloudSuite`.
- Verifikasi: `curl https://erp.idchsuite.my.id/web/login` menghasilkan 200;
  login SSO manual masuk Discuss tanpa stuck `/web/login_successful`.

## 12. Deploy Stalwart + Bulwark (Mail)

TODO — Sprint 0.10.

## 13. Verifikasi Akhir

    docker compose -f /opt/cloudsuite/infra/docker/docker-compose.yml --env-file /opt/cloudsuite/.env ps

Ekspektasi: 7 container Up — postgres/redis/authentik-server/authentik-worker/
nextcloud/odoo/nginx semua `(healthy)` (healthcheck nginx ditambah Sprint 0.9e,
lihat TROUBLESHOOTING #22).

    curl -s -o /dev/null -w "auth:%{http_code}\n" https://auth.idchsuite.my.id/-/health/ready/
    curl -s -o /dev/null -w "erp:%{http_code}\n" https://erp.idchsuite.my.id/web/login
    curl -s https://drive.idchsuite.my.id/status.php | grep -o '"installed":true'

Ditambah SSO test manual tiap aplikasi (login Authentik lalu redirect balik
lalu dashboard).

## 14. Ops Rutin

- Rotate token API tiap sekitar 90 hari (`default_token_duration=days=90`;
  prosedur lengkap 8 langkah di `docs/authentik-setup.md` — ringkasnya: buat
  token sementara, verifikasi, hapus token lama, buat token baru, ambil key,
  update .env, hapus token sementara, verifikasi akhir). Token expiring tidak
  bisa diperpanjang via PATCH (kode Authentik 2026.8.2 memaksa durasi default
  tenant).
- Backup: TODO (belum ada prosedur — cakupan: DB postgres, volume `data/`, `.env`).
- Monitor storage VPS plus ukuran volume `data/` berkala.
- Update image: pin versi di compose (jangan `latest`); test di staging dulu.

## 15. Rollback / Recovery

- **Restore `.env` dari backup:** bila `.env` rusak/ketimpa, pulihkan dari
  snapshot terakhir lalu muat ulang:

      cp /opt/cloudsuite/.env.bak-<tanggal> /opt/cloudsuite/.env
      cd /opt/cloudsuite/infra/docker
      docker compose --env-file /opt/cloudsuite/.env up -d

  (Pola backup staging: `.env.bak-0.9`, `.env.bak-20260910-HHMMSS`.)

- **Restart service bermasalah:** cek dulu dengan `docker compose ps`, lalu
  restart service spesifik (jangan restart semua bila hanya satu yang gagal):

      cd /opt/cloudsuite/infra/docker
      docker compose restart <service>     # mis. redis / odoo / nextcloud
      # atau rebuild satu service:
      docker compose up -d --build <service>

  Bila gagal terus, baca log: `docker compose logs --tail=100 <service>`.

- **Rebuild penuh dari git:** bila repo staging berubah dan live perlu
  disinkronkan ulang (pull dulu di repo, lalu turunkan & naikkan ulang stack):

      cd /home/hermes/cloudsuite
      git pull origin main
      cd /opt/cloudsuite/infra/docker
      docker compose down
      docker compose pull
      docker compose --env-file /opt/cloudsuite/.env up -d --build

  Catatan: `down` menghapus container (volume data tetap), `pull` ambil image
  terbaru, `up -d --build` bangun ulang image lokal (Odoo via Dockerfile) dan
  naikkan stack. Lalu verifikasi dengan `docker compose ps` + curl health
  (Bagian 13).

## 16. Aturan Hermes

- Kerja di `/opt/cloudsuite` (live) plus `/home/hermes/cloudsuite` (repo);
  sinkronkan manual per sprint.
- JANGAN tampilkan nilai secret/token/password di laporan — tulis nama key
  plus lokasi `.env` plus status verifikasi (HTTP code) saja.
- LAPOR dulu bila iteration melebihi 100/150; STOP plus LAPOR bila error;
  bila ragu, tanya.
- Backup `.env` sebelum ubah; bersihkan `/tmp` lokal dan staging tiap selesai;
  JANGAN commit `.env`.

## 17. Kontak dan Referensi

- Repo: `git@github.com:jetswu/cloudsuite.git`
- Dokumen terkait: `docs/authentik-setup.md` (baseline plus changelog SSO),
  `docs/nextcloud-notes.md` (break-glass ncadmin),
  `docs/TROUBLESHOOTING.md` (22 pembelajaran bug).
- Upstream: dokumentasi Authentik, Nextcloud `user_oidc`, Odoo 18 dan OCA
  `auth-oidc`, Cloudflare Origin CA.
