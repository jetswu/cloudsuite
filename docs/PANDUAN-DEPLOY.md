# Panduan Deploy CloudSuite — Dari VPS Kosong

> Satu file, self-contained: deploy CloudSuite dari nol TANPA membuka file lain.
> Semua command copy-paste ready. Target pembaca: manusia (admin).
> Standar dokumen: Sprint 0.10k (2026-09-11). Semua config diambil dari staging live.

**Estimasi:** 4-6 jam (1 orang, tanpa AI).
**Target akhir:** 9 container jalan — SSO Authentik untuk Webmail, Drive (Nextcloud), ERP (Odoo), Mail (Stalwart) dengan DKIM/SPF/DMARC aktif.

## Daftar Isi

- Bagian 0 — Persiapan
- Bagian 1 — Setup VPS
- Bagian 2 — Docker & Tools
- Bagian 3 — Git Clone & Folder
- Bagian 4 — Secrets (.env)
- Bagian 5 — Cloudflare DNS + Origin Cert
- Bagian 6 — Deploy PostgreSQL + Redis
- Bagian 7 — Deploy Authentik
- Bagian 8 — Deploy Nextcloud (Drive)
- Bagian 9 — Deploy Odoo (ERP)
- Bagian 10 — Deploy Stalwart (Mail)
- Bagian 11 — Deploy jmap-webmail (Webmail)
- Bagian 12 — Verifikasi Akhir
- Bagian 13 — Backup (Opsional)
- Lampiran A — Config Templates (Full Copy-Paste)
- Lampiran B — Troubleshooting Cepat
- Lampiran C — Checklist Ringkas

---

## Bagian 0 — Persiapan

### 0.1 Yang Dibutuhkan

| Kebutuhan | Detail |
|---|---|
| VPS | Minimum 4 vCPU / 8 GB RAM; rekomendasi 10 vCPU / 20 GB RAM / 100 GB disk (spec staging). OS: Debian 13 atau Ubuntu 24.04. |
| Domain | Satu domain di Cloudflare (contoh panduan ini: `idchsuite.my.id` — ganti dengan domain Anda). |
| Akun Cloudflare | Akses dashboard: DNS, SSL/TLS, Origin Certificate, API token. |
| Akun GitHub | Akses read ke repo `github.com/jetswu/cloudsuite`. |
| Tools di laptop admin | `ssh`, `git`, `dig` (dnsutils), browser, password manager. |

### 0.2 Checklist Pre-Deploy

- [ ] VPS provisioned, punya IP publik + akses root via SSH
- [ ] Domain aktif, nameserver mengarah ke Cloudflare
- [ ] SSH key laptop sudah bisa login ke VPS
- [ ] Akses repo GitHub `jetswu/cloudsuite`
- [ ] Password manager siap (akan menyimpan ±25 secret)

### 0.3 Diagram Arsitektur

```
                        Cloudflare (DNS + Proxy)
                              │
              ┌───────────────┼────────────────┐
              │ proxied       │ proxied        │ DNS-only (tanpa proxy)
      auth.idchsuite.my.id  drive/erp/webmail   mail.idchsuite.my.id
              │               │                        │ (SMTP/IMAP/POP direct)
              ▼               ▼                        ▼
        ┌─────────────────────────────────────────────────────┐
        │ VPS  (network docker: cloudsuite-net)               │
        │                                                     │
        │  nginx :80/:443 ──► authentik-server:9000 (auth)    │
        │        │         ─► nextcloud:80        (drive)     │
        │        │         ─► odoo:8069           (erp)       │
        │        │         ─► jmap-webmail:3000   (webmail)   │
        │        │         ─► stalwart:8080       (mail WebUI)│
        │        │                                             │
        │  stalwart :25/:465/:587/:143/:993/:995 (direct)     │
        │                                                     │
        │  postgres:16 ◄── authentik / nextcloud / odoo /     │
        │  redis:7     │    stalwart (DB)                     │
        │              └─ session nextcloud / cache authentik │
        └─────────────────────────────────────────────────────┘
```

### 0.4 Urutan Deploy (Overview)

| # | Langkah | Bagian | Estimasi |
|---|---|---|---|
| 1 | Setup VPS (user, SSH, firewall) | 1 | 30 mnt |
| 2 | Docker + tools + network | 2 | 15 mnt |
| 3 | Clone repo + struktur folder | 3 | 10 mnt |
| 4 | Generate + isi .env (Kategori 1) | 4 | 30 mnt |
| 5 | Cloudflare DNS + Origin Cert | 5 | 30 mnt |
| 6 | PostgreSQL + Redis + create DB | 6 | 15 mnt |
| 7 | Authentik (IdP) + provider SSO | 7 | 60 mnt |
| 8 | Nextcloud + user_oidc SSO | 8 | 30 mnt |
| 9 | Odoo + auth_oidc SSO | 9 | 60 mnt |
| 10 | Stalwart + DKIM + DNS records | 10 | 90 mnt |
| 11 | jmap-webmail + SSO | 11 | 30 mnt |
| 12 | Verifikasi akhir | 12 | 15 mnt |

> PENTING: urutan ini wajib — Authentik harus jalan dulu (provider SSO untuk
> semua app), postgres/redis sebelum semua app, Stalwart sebelum webmail.

### 0.5 Konvensi Penulisan

- `<IP>` = IP publik VPS Anda; `<domain>` = domain Anda (contoh: `idchsuite.my.id`)
- Semua command dijalankan **di VPS via SSH** kecuali ditandai "di laptop".
- Secret SELALU placeholder — jangan pernah tulis nilai asli di dokumen/chat.

---

## Bagian 1 — Setup VPS

### 1.1 Login + Update

**Prasyarat:** VPS aktif, SSH key terpasang (dari provider).

    ssh root@<IP>
    apt update && apt upgrade -y
    timedatectl set-timezone Asia/Jakarta
    hostnamectl set-hostname cloudsuite-prod

**Verify:**

    date          # harus WIB (UTC+7)
    hostname      # cloudsuite-prod

**Kalau gagal:** apt upgrade error → cek `df -h` (disk penuh?) dan `/etc/apt/sources.list`. Timezone salah tidak fatal, tapi log semua service jadi sulit dicocokkan.

### 1.2 Buat User Admin

**Prasyarat:** langkah 1.1 selesai.

    adduser admin
    usermod -aG sudo admin
    mkdir -p /home/admin/.ssh
    cp /root/.ssh/authorized_keys /home/admin/.ssh/
    chown -R admin:admin /home/admin/.ssh
    chmod 700 /home/admin/.ssh
    chmod 600 /home/admin/.ssh/authorized_keys

**Verify:** buka terminal BARU (jangan tutup sesi root):

    ssh admin@<IP>
    sudo whoami     # harus: root

**Kalau gagal:** `sudo whoami` minta password → user tidak masuk grup sudo → `usermod -aG sudo admin` ulang dari sesi root.

### 1.3 SSH Hardening

**Prasyarat:** langkah 1.2 terverifikasi (bisa login admin + sudo).

    nano /etc/ssh/sshd_config
    # Set baris berikut (uncomment + ubah):
    #   PermitRootLogin no
    #   PasswordAuthentication no
    #   PubkeyAuthentication yes
    systemctl restart sshd

**Verify:** buka terminal baru:

    ssh admin@<IP>        # harus masuk
    ssh root@<IP>         # harus DITOLAK (Permission denied)

**Kalau gagal:** kalau admin ikut terkunci → login via console VPS provider, balikin `PermitRootLogin yes`. JANGAN tutup sesi lama sebelum verifikasi sukses.

### 1.4 Firewall (UFW)

**Prasyarat:** langkah 1.3 sukses.

    apt install ufw -y
    ufw default deny incoming
    ufw default allow outgoing
    ufw allow 22/tcp
    ufw allow 80/tcp
    ufw allow 443/tcp
    ufw allow 25/tcp
    ufw allow 465/tcp
    ufw allow 587/tcp
    ufw allow 143/tcp
    ufw allow 993/tcp
    ufw allow 995/tcp
    ufw enable

**Verify:**

    ufw status verbose    # 9 rule ALLOW, default deny incoming

**Kalau gagal:** SSH terputus setelah `ufw enable` → port 22 lupa di-allow; masuk via console provider: `ufw allow 22/tcp`.

> Catatan staging: status UFW di staging belum terverifikasi (firewall aktif di
> level provider + Cloudflare). Rule di atas adalah rekomendasi minimal.

### 1.5 Fail2ban

**Prasyarat:** apt berfungsi.

    apt install fail2ban -y
    systemctl enable --now fail2ban
    systemctl status fail2ban --no-pager

**Verify:**

    fail2ban-client status sshd    # menampilkan jail sshd aktif

**Kalau gagal:** `fail2ban-client status sshd` "no such jail" → jail sshd default aktif saat ada log auth; tunggu atau cek `/var/log/fail2ban.log`.

> Konfigurasi jail custom staging: [TBC] — staging memakai default.

### 1.6 Unattended Upgrades

    apt install unattended-upgrades -y
    dpkg-reconfigure --priority=low unattended-upgrades

**Verify:**

    systemctl status unattended-upgrades --no-pager   # active (exited)

---

## Bagian 2 — Docker & Tools

### 2.1 Install Docker

    curl -fsSL https://get.docker.com -o get-docker.sh
    sh get-docker.sh
    apt install docker-compose-plugin -y
    usermod -aG docker admin

**Verify:**

    docker --version        # Docker version 29.x (staging: 29.8.0)
    docker compose version  # Compose v2

**Kalau gagal:** `permission denied` saat docker sebagai admin → logout + login ulang (grup docker baru aktif di sesi baru).

### 2.2 Install Tools Pendukung

    apt install -y git curl wget htop nano vim jq unzip tree dnsutils

**Verify:** `git --version && jq --version && dig -v | head -1`

### 2.3 Docker Logging Config

> Config staging: [TBC] — belum diverifikasi apakah staging memakai daemon.json
> custom. Rekomendasi aman (batasi log agar disk tidak penuh):

    nano /etc/docker/daemon.json
    # paste:
    {
      "log-driver": "json-file",
      "log-opts": { "max-size": "10m", "max-file": "3" }
    }
    systemctl restart docker

**Verify:** `docker info | grep -A2 "Logging Driver"`.

### 2.4 Docker Network

    docker network create cloudsuite-net

**Verify:**

    docker network ls | grep cloudsuite    # cloudsuite-net (bridge)

**Kalau gagal:** "network already exists" → tidak masalah, lanjut.

> PENTING: compose file memakai `networks: cloudsuite-net: external: true` —
> network TIDAK dibuat oleh compose, wajib manual (sekali saja).

---

## Bagian 3 — Git Clone & Folder

### 3.1 Clone Repo

**Prasyarat:** akun admin bisa `docker` dan `git`; SSH key GitHub terpasang (lihat 3.2).

    mkdir -p ~/cloudsuite
    cd ~/cloudsuite
    git clone git@github.com:jetswu/cloudsuite.git .

**Verify:**

    ls infra/ docs/     # infra/docker, infra/nginx, docs/*.md harus ada

**Kalau gagal:** `Permission denied (publickey)` → kerjakan 3.2 dulu.

### 3.2 Setup SSH Key GitHub (kalau belum)

    # di VPS
    ssh-keygen -t ed25519 -C "admin@cloudsuite-vps"
    cat ~/.ssh/id_ed25519.pub
    # → copy output, tambahkan di GitHub → Settings → SSH and GPG keys → New SSH key
    ssh -T git@github.com
    # → "Hi jetswu! You've successfully authenticated."

### 3.3 Folder Structure

**Prasyarat:** user admin punya sudo (folder /opt).

    sudo mkdir -p /opt/cloudsuite
    sudo chown -R admin:admin /opt/cloudsuite
    mkdir -p /opt/cloudsuite/{infra,data,logs,docs,secrets}
    mkdir -p /opt/cloudsuite/infra/{docker,nginx/conf.d,nginx/certs}
    mkdir -p /opt/cloudsuite/data/{postgres,redis,authentik,nextcloud,odoo,stalwart}
    mkdir -p /opt/cloudsuite/logs/nginx
    chmod 700 /opt/cloudsuite/secrets

**Verify:**

    tree /opt/cloudsuite -L 2

**Kalau gagal:** `tree` belum ada → `apt install tree` (langkah 2.2 sudah include).

### 3.4 Copy Config dari Repo ke /opt

**Prasyarat:** 3.1 + 3.3 sukses.

    cp -r ~/cloudsuite/infra/docker/* /opt/cloudsuite/infra/docker/
    cp -r ~/cloudsuite/infra/nginx/* /opt/cloudsuite/infra/nginx/

**Verify:**

    ls /opt/cloudsuite/infra/docker/    # docker-compose.yml, Dockerfile.odoo, odoo.conf.example, stalwart-*.ndjson
    ls /opt/cloudsuite/infra/nginx/conf.d/   # auth.conf drive.conf erp.conf default.conf mail.conf webmail.conf

> Semua config full ada di Lampiran A — kalau tidak mau clone repo, bisa
> copy-paste manual dari sana.

---

## Bagian 4 — Secrets (.env)

### 4.1 Copy .env Template

    cp ~/cloudsuite/.env.example /opt/cloudsuite/.env
    chmod 600 /opt/cloudsuite/.env

**Verify:** `ls -l /opt/cloudsuite/.env` → `-rw-------`

### 4.2 Generate Secrets Kategori 1 (Offline)

Semua bisa dijalankan sebelum service apapun jalan. Simpan hasilnya di password manager SEBELUM ditaruh di .env.

    # Password (semua hex 64 char):
    openssl rand -hex 32
    # Jalankan BERULANG untuk tiap var, hasil masing-masing BEDA:
    #   POSTGRES_PASSWORD, REDIS_PASSWORD, AUTHENTIK_ADMIN_PASSWORD,
    #   NEXTCLOUD_ADMIN_PASSWORD, NEXTCLOUD_DB_PASSWORD,
    #   ODOO_ADMIN_PASSWORD, ODOO_MASTER_PASSWORD, ODOO_DB_PASSWORD,
    #   STALWART_ADMIN_PASSWORD, STALWART_TEST_PASSWORD, STALWART_DB_PASSWORD

    # Authentik secret key (base64 80 char — KHUSUS ini jangan hex):
    openssl rand -base64 60 | tr -d '\n'

Aturan format (dari pembelajaran staging):
- Password (dipakai CLI/shell): **hex** `openssl rand -hex 32`
- Secret key aplikasi (Authentik): **base64** `openssl rand -base64 60 | tr -d '\n'`
- JANGAN pakai base64 untuk password (karakter khusus bikin masalah shell/env).

### 4.3 Isi .env Kategori 1

    nano /opt/cloudsuite/.env

Template .env lengkap ada di Lampiran A.2. Ganti semua `CHANGE_ME`:

| Grup | Var yang diisi sekarang |
|---|---|
| Umum | `DOMAIN` `PUBLIC_IP` `PRIVATE_IP` `TZ` |
| Postgres | `POSTGRES_DB` `POSTGRES_USER` `POSTGRES_PASSWORD` |
| Redis | `REDIS_PASSWORD` |
| Authentik | `AUTHENTIK_SECRET_KEY` `AUTHENTIK_ADMIN_PASSWORD` `AUTHENTIK_ADMIN_EMAIL` |
| Nextcloud | `NEXTCLOUD_ADMIN_USER` `NEXTCLOUD_ADMIN_PASSWORD` `NEXTCLOUD_DB_*` |
| Odoo | `ODOO_ADMIN_PASSWORD` `ODOO_MASTER_PASSWORD` `ODOO_DB_*` |
| Stalwart | `STALWART_ADMIN_*` `STALWART_DB_*` `STALWART_DOMAIN` `STALWART_MAIL_HOSTNAME` `STALWART_TEST_PASSWORD` |

Nilai bawaan yang JANGAN diubah: `POSTGRES_DB=cloudsuite`, `POSTGRES_USER=cloudsuite`, `NEXTCLOUD_DB_NAME=nextcloud`, `ODOO_DB_NAME=odoo`, `STALWART_DB_NAME=stalwart`, semua `*_DB_USER=cloudsuite`, semua `*_DB_HOST=postgres`, `TZ=Asia/Jakarta`.

JANGAN isi dulu 5 var bertanda `[LATE]` — butuh service jalan dulu (Bagian 7, 10, 11): `AUTHENTIK_API_TOKEN`, `AUTHENTIK_TEMPLATE_CLIENT_SECRET`, `STALWART_API_KEY`, `JMAPWEBMAIL_CLIENT_ID`, `JMAPWEBMAIL_CLIENT_SECRET`. Plus `CLOUDFLARE_API_TOKEN` (dibuat di dashboard, bisa kapan saja).

**Verify:** `grep -c CHANGE_ME /opt/cloudsuite/.env` → hanya sisa var [LATE] + `CLOUDFLARE_API_TOKEN` yang masih CHANGE_ME.

### 4.4 Simpan di Password Manager

Simpan SEMUA yang diisi di 4.3 + catat:
- tanggal generate
- untuk token: tanggal expire (Authentik token 90 hari — pasang reminder rotate)
- `STALWART_RECOVERY_ADMIN` nanti (Bagian 10) — ini kunci darurat terakhir mail server

> JANGAN commit `.env` ke git — repo hanya menyimpan `.env.example`.
> Backup manual tiap sebelum ubah: `cp /opt/cloudsuite/.env /opt/cloudsuite/.env.bak-$(date +%Y%m%d-%H%M%S)`

---

## Bagian 5 — Cloudflare DNS + Origin Cert

### 5.1 Login Cloudflare

Buka `https://dash.cloudflare.com` → pilih domain → pastikan nameserver domain sudah mengarah ke Cloudflare (cek di registrar domain).

### 5.2 Setup A Records

Menu **DNS → Records → Add record**. Tabel lengkap (ganti `<IP>` dengan IP VPS):

| Type | Name | Content | Proxy status |
|---|---|---|---|
| A | `auth` | `<IP>` | **Proxied** (oranye) |
| A | `drive` | `<IP>` | **Proxied** |
| A | `erp` | `<IP>` | **Proxied** |
| A | `webmail` | `<IP>` | **Proxied** |
| A | `mail` | `<IP>` | **DNS only** (abu-abu) ⚠️ |

⚠️ `mail` WAJib DNS only — proxy Cloudflare hanya untuk HTTP(S), SMTP/IMAP tidak boleh lewat proxy. Kalau mail di-proxied, port 25/465/587/993 tidak akan sampai ke VPS.

> Opsional (belum dipakai staging): A record `portal`, `api`, `admin`, root `@` — kalau mau disiapkan sejak awal, samakan dengan `auth` (Proxied).

### 5.3 SSL/TLS Mode

Menu **SSL/TLS → Overview**:

- Mode: **Full (strict)** — WAJIB. Origin menyajikan Cloudflare Origin Certificate (5.4). Mode "Flexible" akan error redirect loop.

Lalu menu **SSL/TLS → Edge Certificates**:
- **Always Use HTTPS**: ON
- **Automatic HTTPS Rewrites**: ON

### 5.4 Origin Certificate

1. Menu **SSL/TLS → Origin Server → Create Certificate**.
2. Pilih: Private key type **RSA (2048)**, Hostnames: `*.idchsuite.my.id` + `idchsuite.my.id`, validity maksimum.
3. Copy "Origin Certificate" → paste di VPS:

       nano /opt/cloudsuite/infra/nginx/certs/origin.pem    # paste cert
       nano /opt/cloudsuite/infra/nginx/certs/origin-key.pem # paste key
       chmod 600 /opt/cloudsuite/infra/nginx/certs/origin-key.pem

**Verify:** `openssl x509 -in /opt/cloudsuite/infra/nginx/certs/origin.pem -noout -subject -enddate`

**Kalau gagal:** nginx nanti error "key values mismatch" → cert dan key tidak satu pasangan; ulangi create + paste.

### 5.5 CORS (catatan, tanpa aksi)

jmap-webmail butuh CORS header di hostname mail — implementasi staging memakai **nginx** (sudah builtin di `mail.conf`, lihat Lampiran A.3). TIDAK perlu Cloudflare Transform Rule.

### 5.6 Verify DNS

    dig +short auth.<domain>    # IP Cloudflare (proxy)
    dig +short webmail.<domain> # IP Cloudflare (proxy)
    dig +short mail.<domain>    # HARUS IP VPS langsung (DNS only)

**Kalau gagal:** mail.* mengembalikan IP Cloudflare → proxy belum dimatikan (klik awan di DNS record sampai abu-abu).

---

## Bagian 6 — Deploy PostgreSQL + Redis

### 6.1 Start

**Prasyarat:** Bagian 2-4 selesai (.env Kategori 1 terisi), network `cloudsuite-net` ada.

    cd /opt/cloudsuite/infra/docker
    docker compose --env-file /opt/cloudsuite/.env up -d postgres redis

**Verify:**

    docker compose ps    # postgres + redis: Up (healthy)
    docker exec cloudsuite-postgres psql -U cloudsuite -c "SELECT version();"
    docker exec cloudsuite-redis sh -c 'redis-cli -a "$REDIS_PASSWORD" ping'   # PONG

**Kalau gagal:** redis log `WRONGPASS` → `REDIS_PASSWORD` di .env beda dengan yang dipakai saat container pertama start; kalau fresh install pastikan .env benar SEBELUM `up` pertama.

### 6.2 Create Databases

**Prasyarat:** 6.1 healthy. Database dibuat SEKALI sebelum app start (image app tidak membuat DB sendiri, kecuali nextcloud yang auto-create via entrypoint).

    docker exec cloudsuite-postgres psql -U cloudsuite -c "CREATE DATABASE authentik OWNER cloudsuite;"
    docker exec cloudsuite-postgres psql -U cloudsuite -c "CREATE DATABASE odoo OWNER cloudsuite;"
    docker exec cloudsuite-postgres psql -U cloudsuite -c "CREATE DATABASE stalwart OWNER cloudsuite;"

> Database `nextcloud` dibuat otomatis oleh container Nextcloud saat first run — tidak perlu manual.

**Verify:**

    docker exec cloudsuite-postgres psql -U cloudsuite -c "\l" | grep -E "authentik|odoo|stalwart|nextcloud"

---

## Bagian 7 — Deploy Authentik

### 7.1 Start Container

**Prasyarat:** Bagian 6 selesai.

    cd /opt/cloudsuite/infra/docker
    docker compose --env-file /opt/cloudsuite/.env up -d authentik-server authentik-worker
    docker logs cloudsuite-authentik-server --tail 20

**Verify:**

    docker compose ps     # authentik-server + authentik-worker: Up
    docker compose --env-file /opt/cloudsuite/.env up -d nginx
    curl -s -o /dev/null -w "%{http_code}\n" https://auth.<domain>/-/health/ready/   # 200

**Kalau gagal:**
- Log error 97 `EAFNOSUPPORT` / restart loop → cek IPv4 override sudah ada di compose (7.2).
- 502 dari nginx → tunggu ±1 menit (Authentik startup lama), lalu cek `docker logs`.

### 7.2 IPv4 Override (PENTING)

VPS tanpa IPv6: Authentik default bind `[::]:9000/9443/9300` → crash loop `EAFNOSUPPORT`. Compose di repo SUDAH memuat fix ini di KEDUA service (server + worker) — jangan hapus:

```yaml
AUTHENTIK_LISTEN__HTTP: 0.0.0.0:9000
AUTHENTIK_LISTEN__HTTPS: 0.0.0.0:9443
AUTHENTIK_LISTEN__METRICS: 0.0.0.0:9300
```

**Verify:** `grep AUTHENTIK_LISTEN /opt/cloudsuite/infra/docker/docker-compose.yml` → 6 baris (3 per service).

### 7.3 Nginx auth.conf

Sudah dicopy di 3.4 (`infra/nginx/conf.d/auth.conf` — full di Lampiran A.3). Nginx start bareng di 7.1. Test ulang:

    docker exec cloudsuite-nginx nginx -t        # syntax OK
    curl -s -o /dev/null -w "%{http_code}\n" http://auth.<domain>/health   # 200 (via CF)

### 7.4 Setup Admin

Login PERTAMA pakai bootstrap (env `AUTHENTIK_ADMIN_EMAIL` + `AUTHENTIK_ADMIN_PASSWORD`, user `akadmin`) di `https://auth.<domain>`.

Langsung ganti password (bootstrap env hanya berlaku first-run):

    printf 'PASSWORD_BARU\nPASSWORD_BARU\n' | docker exec -i cloudsuite-authentik-server ak changepassword akadmin
    # → "Password successfully changed for user akadmin"

**Verify:** logout, login ulang dengan password baru.

**Kalau gagal:** lupa password bootstrap dan first-run sudah lewat → `docker exec -it cloudsuite-authentik-server ak shell` lalu:

```python
from authentik.core.models import User
u = User.objects.get(username="akadmin")
u.set_password("PASSWORD_BARU"); u.save()
```

### 7.5 Baseline Setup (via UI Authentik)

Login admin interface (admin interface icon kiri bawah). Buat:

1. **Branding:** Flows & Branding → Brands → edit default: brand name `CloudSuite`, default domain `auth.<domain>`.
2. **Groups** (Directory → Groups, create):
   - `cloudsuite-superadmin`
   - `cloudsuite-tenant-admin`
   - `cloudsuite-users`
3. **User test** (Directory → Users): buat user (mis. `admin@<domain>`) + masukkan grup `cloudsuite-users`. Password dicatat di password manager. User ini nanti dipakai login SSO webmail/mail/erp.
4. **Setting durasi token:** Admin interface → System → Settings (`/api/v3/admin/settings/` via swagger, atau UI Settings) → `default_token_duration = days=90`. [TBC — di staging di-set via API PATCH; letak UI persisnya bisa beda antar versi]
5. **API Token** `hermes-automation` (Directory → Tokens → Create): identifier `hermes-automation`, user akadmin, intent **API**, expiring (90 hari). Klik **View key** → copy → ini `AUTHENTIK_API_TOKEN` [LATE].
6. **Provider template** `template-cloudsuite-services` (Applications → Providers → Create → OAuth2): authorization flow `default-provider-authorization-explicit-consent`, invalidation flow `default-provider-invalidation-flow`, signing key `authentik Self-signed Certificate`, client type confidential. Lalu Create Application slug `template-cloudsuite-services` + assign provider. Client secret-nya → `AUTHENTIK_TEMPLATE_CLIENT_SECRET` [LATE].

**Verify:** `https://auth.<domain>/application/o/template-cloudsuite-services/.well-known/openid-configuration` → 200 JSON berisi issuer/endpoints.

**Kalau gagal:** membuat provider via API → tanpa `invalidation_flow` → HTTP 400 "This field is required" — sertakan authorization_flow + invalidation_flow + signing_key.

### 7.6 Isi [LATE] Secrets Authentik

    nano /opt/cloudsuite/.env
    # AUTHENTIK_API_TOKEN=<dari 7.5 langkah 5>
    # AUTHENTIK_TEMPLATE_CLIENT_SECRET=<dari 7.5 langkah 6>

**Verify:** `grep -c CHANGE_ME /opt/cloudsuite/.env` berkurang 2.

---

## Bagian 8 — Deploy Nextcloud (Drive)

### 8.1 Deploy Container

    cd /opt/cloudsuite/infra/docker
    docker compose --env-file /opt/cloudsuite/.env up -d nextcloud
    # first run lama (install + DB) — tunggu 1-2 menit

**Verify:**

    docker compose ps nextcloud    # Up (healthy)
    curl -s https://drive.<domain>/status.php | grep -o '"installed":true'

**Kalau gagal:** cek `docker logs cloudsuite-nextcloud --tail 50`; error DB → pastikan 6.2/postgres healthy dan `NEXTCLOUD_DB_*` benar.

> Akun `admin`/`ncadmin` (env `NEXTCLOUD_ADMIN_USER`) = akun BREAK-GLASS darurat. Login normal via SSO. Jangan dipakai harian.

### 8.2 Nginx drive.conf

Sudah tercopy (3.4, Lampiran A.3). Verifikasi upload besar nanti: `client_max_body_size 10G` sudah builtin.

### 8.3 Install user_oidc

    docker exec -u www-data cloudsuite-nextcloud php occ app:install user_oidc
    # kalau sudah ada: php occ app:enable user_oidc

**Verify:** `docker exec -u www-data cloudsuite-nextcloud php occ app:list | grep user_oidc` → enabled, versi 8.x.

### 8.4 Authentik Provider untuk Nextcloud

Di Authentik admin → Applications → Providers → Create → OAuth2 (atau via template 7.5 langkah 6, klik duplicate):

- Name: `Nextcloud`, client type confidential
- Redirect URI (strict, bukan regex): `https://drive.<domain>/apps/user_oidc/code`
- Signing key: `authentik Self-signed Certificate`
- Sub mode: `hashed_user_id`, issuer mode: global
- Scopes/mappings: openid, email, profile
- **grant_types WAJIB diisi eksplisit** (via tab Advanced / API): `authorization_code, hybrid, implicit, client_credentials, password, device_code, refresh_token` — default kosong → login gagal `invalid_request` + log "Invalid grant_type"

Lalu Create Application (name Nextcloud, slug `nextcloud`) + assign provider ini.

Catat: **Client ID** + **Client Secret** (View secret).

**Verify:** `https://auth.<domain>/application/o/nextcloud/.well-known/openid-configuration` → 200.

### 8.5 Konfigurasi OIDC di Nextcloud

⚠️ Format 8.x — JANGAN pakai `config:app:set` (membuat provider dengan clientId kosong):

    docker exec -u www-data cloudsuite-nextcloud php occ user_oidc:provider CloudSuite \
      --clientid=<CLIENT_ID> \
      --clientsecret=<CLIENT_SECRET> \
      --discoveryuri=https://auth.<domain>/application/o/nextcloud/.well-known/openid-configuration \
      --unique-uid=1

**Verify:** `docker exec -u www-data cloudsuite-nextcloud php occ user_oidc:provider` → list provider CloudSuite dengan clientId terisi.

### 8.6 Verify SSO

Browser: buka `https://drive.<domain>` → klik login → redirect ke Authentik → login user dari 7.5 langkah 3 → consent Continue → dashboard Nextcloud. User OIDC auto-provision.

**Kalau gagal:**
- `invalid_request` saat authorize → grant_types belum diisi (8.4).
- Redirect balik error → redirect URI tidak persis sama (harus strict match).

---

## Bagian 9 — Deploy Odoo (ERP)

### 9.1 Dockerfile.odoo

Sudah di `/opt/cloudsuite/infra/docker/Dockerfile.odoo` (dari repo, full di Lampiran A.4): `FROM odoo:18.0` + `pip install --break-system-packages python-jose odoo-addon-auth-oidc==18.0.1.1.0.2` (OCA auth_oidc — native Odoo tidak dukung authorization-code flow murni).

### 9.2 Siapkan odoo.conf

    cp /opt/cloudsuite/infra/docker/odoo.conf.example /opt/cloudsuite/data/odoo/config/odoo.conf
    nano /opt/cloudsuite/data/odoo/config/odoo.conf
    # isi: admin_passwd = <ODOO_MASTER_PASSWORD>, db_password = <ODOO_DB_PASSWORD>
    # (nilai dari .env)
    chown 100:101 /opt/cloudsuite/data/odoo/config/odoo.conf
    chmod 600 /opt/cloudsuite/data/odoo/config/odoo.conf

> `dbfilter = ^odoo$` + `list_db = False` = database selector TIDAK muncul (anti brute list DB).

**Verify:** `ls -l /opt/cloudsuite/data/odoo/config/odoo.conf` → owner 100:101, mode 600.

### 9.3 Deploy

    cd /opt/cloudsuite/infra/docker
    docker compose --env-file /opt/cloudsuite/.env up -d --build odoo
    # build first time ±5 menit; start_period healthcheck 180s

**Verify:**

    docker compose ps odoo    # Up (healthy)
    curl -s -o /dev/null -w "%{http_code}\n" https://erp.<domain>/web/login   # 200

**Kalau gagal:**
- `NoSectionError` / permission denied → odoo.conf owner harus `100:101` mode 600 (9.2).
- Selector database muncul → `dbfilter`/`list_db` belum benar.
- `externally-managed-environment` saat build → pastikan pakai Dockerfile dari repo (ada `--break-system-packages`).

### 9.4 Setup DB Odoo (first run)

Buka `https://erp.<domain>/web/login`. Kalau DB `odoo` masih kosong, Odoo meminta master password → isi `ODOO_MASTER_PASSWORD`. Login admin pertama: password admin = `ODOO_ADMIN_PASSWORD`, lalu SEGERA set email admin ke `admin@<domain>` (dipakai break-glass). [TBC — alur first-run persis: di staging DB di-init via `--without-demo=all` + dbfilter; ikuti wizard di UI]

### 9.5 Template User Internal (PENTING)

Tanpa ini, user SSO baru jadi **Portal** (bukan internal) dan stuck di `/web/login_successful`:

1. Login Odoo sebagai admin (9.4).
2. Settings → Users → create user baru: name `cloudsuite_template`, login `template@cloudsuite.local`, active.
3. Edit user: hapus semua grup KECUALI **Internal User** + **Technical Features**. Pastikan "portal"/share = OFF.
4. Aktifkan developer mode (Settings → General Settings → Developer Tools → Activate).
5. Settings → Technical → System Parameters → cari `base.template_portal_user_id` → ganti value dari `5` ke **id user template** barusan (cek di URL/atau kolom ID).

> JANGAN pakai admin sebagai template (copy admin = privilege escalation).

**Verify:** System parameter `base.template_portal_user_id` = ID user cloudsuite_template (di staging: 8).

### 9.6 OAuth Provider Authentik (untuk Odoo)

Di Authentik (seperti 8.4):

- Name `Odoo`, client confidential
- Redirect URI strict: `https://erp.<domain>/auth_oauth/signin`
- grant_types: **`authorization_code`, `refresh_token`** (Odoo cuma butuh 2 ini)
- Signing key self-signed, sub mode hashed_user_id
- Create Application slug `odoo` + assign.

Catat Client ID + Secret.

### 9.7 Konfigurasi Provider di Odoo

1. Odoo → Settings → General Settings → Integrasi/OAuth Providers → tambah:
   - Name: `CloudSuite`, Client ID = dari 9.6, Secret = dari 9.6
   - Flow: **id_token_code** (authorization code + id_token)
   - Scope: `openid email profile`
   - Authorization endpoint: `https://auth.<domain>/application/o/odoo/authorize`
   - Token endpoint: `https://auth.<domain>/application/o/odoo/token`
   - UserInfo endpoint: `https://auth.<domain>/application/o/odoo/userinfo`
   - JWKS endpoint: `https://auth.<domain>/application/o/odoo/.well-known/jwks.json` [TBC — format URL jwks persis; cek di well-known provider]
   - Enabled ✓
2. (Opsional, UX) Home action default → Technical → User Defaults: model `res.users`, field `action_id` = action Discuss (`mail.action_discuss`).

**Verify:** `https://erp.<domain>/web/login` → ada tombol **Login with CloudSuite**.

### 9.8 Verify SSO

Login via "Login with CloudSuite" → Authentik → user baru auto-create dengan grup **Internal User** + Technical Features (dari template 9.5) → masuk halaman Discuss (bukan stuck `/web/login_successful`).

**Kalau gagal:**
- Stuck `/web/login_successful` → template user (9.5) belum benar / belum re-login.
- Selector DB muncul → cek 9.2.

---

## Bagian 10 — Deploy Stalwart (Mail)

### 10.1 Siapkan config.json

    nano /opt/cloudsuite/data/stalwart/config.json
    # paste (hanya DataStore — JANGAN isi apapun selain ini):
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
    chown -R 2000:2000 /opt/cloudsuite/data/stalwart
    chmod 600 /opt/cloudsuite/data/stalwart/config.json

> config.json HANYA DataStore. Isi `bootstrap`/`acme`/`certificates` di config.json → Stalwart REJECT saat start (semua konfigurasi lain via API/NDJSON, tersimpan di DB).

**Verify:** `cat /opt/cloudsuite/data/stalwart/config.json | jq .` → valid JSON, 3 key.

### 10.2 Deploy Container

    cd /opt/cloudsuite/infra/docker
    docker compose --env-file /opt/cloudsuite/.env up -d stalwart
    docker inspect cloudsuite-stalwart --format "{{.State.Health.Status}}"   # tunggu healthy

**Verify:** `curl -s http://$(docker inspect cloudsuite-stalwart --format '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}'):8080/healthz/live` → 200 `{"detail":"OK"}`

**Kalau gagal:** config.json invalid → container restart loop; cek `docker logs cloudsuite-stalwart`.

### 10.3 Nginx mail.conf

Sudah tercopy (3.4). Catatan penting `mail.conf` (Lampiran A.3): pakai cert `mail.pem`/`mail-key.pem` (bukan origin.pem) — hasil ACME Stalwart (10.5). Copy dulu origin cert sebagai placeholder supaya nginx jalan, replace setelah LE terbit:

    cp /opt/cloudsuite/infra/nginx/certs/origin.pem /opt/cloudsuite/infra/nginx/certs/mail.pem
    cp /opt/cloudsuite/infra/nginx/certs/origin-key.pem /opt/cloudsuite/infra/nginx/certs/mail-key.pem
    docker compose --env-file /opt/cloudsuite/.env up -d nginx

### 10.4 Provisioning Dasar via Recovery Mode (Bootstrap)

Stalwart fresh tidak punya admin — provisioning pertama via **recovery mode**. CLI stalwart tidak ada di image — download di host:

    cd /tmp
    curl -LO https://github.com/stalwartlabs/stalwart-cli/releases/latest/download/stalwart-cli-x86_64-unknown-linux-gnu.zip
    unzip stalwart-cli-x86_64-unknown-linux-gnu.zip
    CLI=/tmp/stalwart-cli-x86_64-unknown-linux-gnu/stalwart-cli

Aktifkan recovery admin (edit compose atau export):

    # Cara staging: tambahkan env di service stalwart (compose):
    #   STALWART_RECOVERY_ADMIN=admin:<STALWART_ADMIN_PASSWORD>
    nano /opt/cloudsuite/infra/docker/docker-compose.yml   # tambah baris env di atas (ganti password)
    cd /opt/cloudsuite/infra/docker
    docker compose --env-file /opt/cloudsuite/.env up -d stalwart

    IP=$(docker inspect cloudsuite-stalwart --format '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}')

**Verify:** `$CLI --url http://$IP:8080 --user admin --password <STALWART_ADMIN_PASSWORD> query Account` → exit 0 (belum ada data, tapi auth OK).

### 10.5 Apply NDJSON Dasar

Buat file NDJSON provisioning dasar. Format: 1 objek JSON per baris; secret via EnvironmentVariable reference; subcommand = `apply` (BUKAN `import`):

    nano /tmp/stalwart-bootstrap.ndjson

Contoh minimal (Domain + Account admin) — field reference:

```ndjson
{"@type":"upsert","object":"Domain","matchOn":["name"],"value":{"idchsuite.my.id":{"@type":"Domain","name":"idchsuite.my.id","description":"CloudSuite primary domain","dkimManagement":{"@type":"Automatic"}}}}
```

```ndjson
{"@type":"upsert","object":"Account","matchOn":["name"],"value":{"admin":{"@type":"User","name":"admin","domainId":"<domain-id>","description":"System Administrator","roles":{"@type":"Admin"},"credentials":{"0":{"@type":"Password","secret":"<STALWART_ADMIN_PASSWORD>"}}}}}
```

⚠️ Ganti `idchsuite.my.id` dengan domain Anda; `<domain-id>` diambil dari `query Domain` output setelah apply Domain. `roles` format tagged enum `{"@type":"Admin"}` — BUKAN `{"admin":true}`. `credentials` = object map key integer string, BUKAN array.

Apply:

    $CLI --url http://$IP:8080 --user admin --password <STALWART_ADMIN_PASSWORD> apply --dry-run --file /tmp/stalwart-bootstrap.ndjson
    $CLI --url http://$IP:8080 --user admin --password <STALWART_ADMIN_PASSWORD> apply --file /tmp/stalwart-bootstrap.ndjson

> NDJSON DnsServer (Cloudflare) + AcmeProvider (Let's Encrypt DNS-01) juga perlu
> di-apply untuk terbitnya cert LE wildcard + auto-publish DNS — format field
> persisnya: [TBC — cek `describe DnsServer` dan `describe AcmeProvider` via CLI,
> atau dokumentasi Stalwart "remote management"]. `CLOUDFLARE_API_TOKEN` sudah
> ter-passing ke container via compose env.

**Verify:**

    $CLI --url http://$IP:8080 --user admin --password <PW> query Domain
    $CLI --url http://$IP:8080 --user admin --password <PW> query Account

### 10.6 Listener Tambahan (587 + 143)

Default listener: 25, 465, 993, 995, 443, 8080, 4190. Tambahan STARTTLS 587/143 via NDJSON (file ada di repo `infra/docker/stalwart-listeners.ndjson`):

    $CLI --url http://$IP:8080 --user admin --password <PW> apply --file /opt/cloudsuite/infra/docker/stalwart-listeners.ndjson

    # Stalwart TIDAK hot-reload listener → restart:
    cd /opt/cloudsuite/infra/docker
    docker compose --env-file /opt/cloudsuite/.env restart stalwart

**Verify:** `docker exec cloudsuite-stalwart ss -tlnp 2>/dev/null || cat /proc/net/tcp | head` [TBC — image minim tool; verifikasi via koneksi eksternal `openssl s_client -starttls smtp -connect mail.<domain>:587`]

### 10.7 DNS Records + DKIM

Setelah Domain dibuat dengan `dkimManagement: Automatic`, Stalwart generate DKIM keys (RSA + Ed25519) dan publish DNS via Task `DnsManagement` (butuh DnsServer + `CLOUDFLARE_API_TOKEN` dari 10.5):

    $CLI --url http://$IP:8080 --user admin --password <PW> apply --file /opt/cloudsuite/infra/docker/stalwart-task-dns.ndjson

Task akan: generate/update MX, SPF, DKIM, DMARC, SRV, MTA-STS, TLSRPT, CAA, autoconfig → ke Cloudflare otomatis. Task hilang dari query setelah selesai (normal).

Records yang di-publish (cek di dashboard Cloudflare DNS):

```
idchsuite.my.id.           MX 10 mail.idchsuite.my.id.
idchsuite.my.id.           TXT "v=spf1 mx -all"
mail.idchsuite.my.id.      TXT "v=spf1 a -all"
v1-rsa-<tanggal>._domainkey    TXT "v=DKIM1; k=rsa; ..."
v1-ed25519-<tanggal>._domainkey TXT "v=DKIM1; k=ed25519; ..."
_dmarc                    TXT "v=DMARC1; p=reject; rua=mailto:postmaster@..."
_submissions._tcp         SRV 0 1 465 mail...
_imaps._tcp               SRV 0 1 993 mail...
_pop3s._tcp               SRV 0 1 995 mail...
_jmap._tcp                SRV 0 1 443 mail...
_mta-sts                  TXT "v=STSv1; id=..."
mta-sts                   CNAME mail...
_smtp._tls                TXT "v=TLSRPTv1; ..."
autoconfig/autodiscover   CNAME mail...
```

**Verify:**

    dig +short MX <domain>
    dig +short TXT _dmarc.<domain>
    # Kirim test mail: dari akun mail → Gmail, lihat "show original" → DKIM-Signature ada + dkim=pass

**Kalau gagal:** DKIM `pending` → trigger ulang Task DnsManagement atau cek `dkimManagement` di Domain harus `Automatic`, lalu restart stalwart.

### 10.8 Directory OIDC + ApiKey

> ⚠️ Urutan penting: ApiKey dibuat SEBELUM set `directoryId` (setelah itu login password mati).

1. Buat **ApiKey** manajemen:

       $CLI --url http://$IP:8080 --user admin --password <PW> create ApiKey --field description=cloudsuite-admin
       # copy key → /opt/cloudsuite/.env (STALWART_API_KEY) + /opt/cloudsuite/secrets/stalwart-apikey.txt (chmod 600)

2. Buat **Directory OIDC** (upsert via CLI, escape hati-hati — atau via webadmin `https://mail.<domain>/admin` sebelum directoryId di-set):

       # issuerUrl DENGAN trailing slash (match iss token Authentik)
       $CLI --url http://$IP:8080 --user admin --password <PW> --json update Directory ... [TBC — format NDJSON Directory staging: issuerUrl https://auth.<domain>/application/o/stalwart-mail/, audience stalwart-mail]

3. Set **Authentication.directoryId** = ID directory OIDC dari langkah 2 (`query Directory` untuk ambil id):

       $CLI --url http://$IP:8080 --user admin --password <PW> update Authentication --field directoryId=<directory-id>

> Setelah ini: login admin password (webadmin + CLI Basic) = **401 by design**.
> Semua manajemen selanjutnya pakai `--api-key "$STALWART_API_KEY"`.
> Break-glass terakhir: `STALWART_RECOVERY_ADMIN` di compose (10.4).

4. Hapus recovery admin dari compose (security) — simpan nilainya di password manager.

**Verify:**

    source /opt/cloudsuite/.env
    $CLI --url http://$IP:8080 --api-key "$STALWART_API_KEY" query Account   # exit 0
    $CLI --url http://$IP:8080 --user admin --password apapun query Account  # 401 (expected)

### 10.9 Verify Mail

    # SMTP submission STARTTLS
    openssl s_client -starttls smtp -connect mail.<domain>:587 -brief
    # IMAPS
    openssl s_client -connect mail.<domain>:993 -brief
    # Kirim/receive test via webmail (Bagian 11) atau telnet 465

---

## Bagian 11 — Deploy jmap-webmail (Webmail)

### 11.1 Authentik Provider stalwart-mail

Di Authentik (pola sama 8.4):

- Name `Stalwart Mail`, **Client ID custom: `stalwart-mail`** (WAJIB match `requireAudience` di Directory Stalwart)
- Client type confidential → secret → ini `JMAPWEBMAIL_CLIENT_SECRET` [LATE]
- Redirect URIs **regex**: `https://webmail\.<domain>(/[a-z]{2})?/auth/callback` (locale OPSIONAL — jangan wajibkan `[a-z]{2}`)
- sub_mode: `user_email` (login pakai email); include claims in id_token: ON
- grant_types: `authorization_code`, `refresh_token`
- Signing key self-signed; mappings openid/email/profile
- Create Application slug `stalwart-mail`, launch URL `https://webmail.<domain>`

### 11.2 Container

Isi dulu .env:

    nano /opt/cloudsuite/.env
    # JMAPWEBMAIL_CLIENT_ID=stalwart-mail
    # JMAPWEBMAIL_CLIENT_SECRET=<dari 11.1>
    # JMAPWEBMAIL_OIDC_ISSUER_URL=https://auth.<domain>/application/o/stalwart-mail   ← TANPA trailing slash!
    # JMAPWEBMAIL_JMAP_URL=https://mail.<domain>

⚠️ Konvensi slash (sumber bug paling sering):
- `JMAPWEBMAIL_OIDC_ISSUER_URL` (.env): **TANPA** `/`
- issuerUrl Directory Stalwart + iss token: **DENGAN** `/`

    cd /opt/cloudsuite/infra/docker
    docker compose --env-file /opt/cloudsuite/.env up -d jmap-webmail

**Verify:** `docker compose ps jmap-webmail` → Up (healthy)

### 11.3 Nginx webmail.conf

Sudah tercopy (3.4, Lampiran A.3). CORS untuk mail hostname sudah builtin di `mail.conf` (Access-Control-Allow-Origin webmail + OPTIONS 204).

### 11.4 Verify SSO

Browser `https://webmail.<domain>` → redirect Authentik → login pakai **EMAIL** (mis. `admin@<domain>`, bukan username) → balik ke webmail → inbox tampil, console browser 0 error.

**Kalau gagal:**
- Authentik log "Invalid redirect URI" → regex redirect belum match (11.1)
- 401 setelah redirect → `Authentication.directoryId` Stalwart belum di-set (10.8 langkah 3) — ada delay propagasi ±15 menit; kalau masih gagal restart stalwart.
- CORS error di console → pastikan `mail.conf` CORS aktif (`docker exec cloudsuite-nginx nginx -t` + reload).

---

## Bagian 12 — Verifikasi Akhir

### 12.1 9 Container

    cd /opt/cloudsuite/infra/docker
    docker compose --env-file /opt/cloudsuite/.env ps

Ekspektasi 9 container `(healthy)`: postgres, redis, authentik-server, authentik-worker, nextcloud, odoo, stalwart, nginx, jmap-webmail.

### 12.2 HTTP Endpoints

    curl -s -o /dev/null -w "auth: %{http_code}\n"    https://auth.<domain>/-/health/ready/
    curl -s -o /dev/null -w "drive: %{http_code}\n"   https://drive.<domain>/status.php
    curl -s -o /dev/null -w "erp: %{http_code}\n"     https://erp.<domain>/web/login
    curl -s -o /dev/null -w "webmail: %{http_code}\n" https://webmail.<domain>/
    # auth=200, drive=200, erp=200, webmail=200

### 12.3 Mail TLS + DNS

    dig +short MX <domain>                       # mail.<domain>
    dig +short TXT _dmarc.<domain>               # v=DMARC1...
    openssl s_client -connect mail.<domain>:993 -brief     # TLS ok
    openssl s_client -starttls smtp -connect mail.<domain>:587 -brief

### 12.4 SSO Manual Test (browser)

- [ ] webmail → login email → inbox
- [ ] drive → login → dashboard (user OIDC auto-provision)
- [ ] erp → "Login with CloudSuite" → Discuss (bukan login_successful)

### 12.5 Mail Send/Receive

- [ ] internal: kirim dari user mail ke admin@
- [ ] external: kirim ke Gmail → cek header: `DKIM-Signature` ada, `dkim=pass`, `spf=pass`, `dmarc=pass`
- [ ] balas dari Gmail → masuk inbox webmail

---

## Bagian 13 — Backup (Opsional)

Status: [TBC] — prosedur backup lengkap belum ada (TODO staging).

Minimal manual (pg_dump):

    docker exec cloudsuite-postgres pg_dump -U cloudsuite -d authentik > /root/backup/authentik_$(date +%F).sql
    docker exec cloudsuite-postgres pg_dump -U cloudsuite -d nextcloud > /root/backup/nextcloud_$(date +%F).sql
    docker exec cloudsuite-postgres pg_dump -U cloudsuite -d odoo > /root/backup/odoo_$(date +%F).sql
    docker exec cloudsuite-postgres pg_dump -U cloudsuite -d stalwart > /root/backup/stalwart_$(date +%F).sql

Plus: `/opt/cloudsuite/.env` (encrypted, di luar VPS), volume `/opt/cloudsuite/data/nextcloud` (file drive), `/opt/cloudsuite/data/odoo/filestore`.

> Restore .env: `cp /opt/cloudsuite/.env.bak-<tanggal> /opt/cloudsuite/.env` lalu `docker compose --env-file /opt/cloudsuite/.env up -d`.

## Ops Rutin (pengingat)

- Rotate token API Authentik tiap ±90 hari (expire otomatis; token expiring TIDAK bisa diperpanjang — buat baru, hapus lama).
- LE cert mail renewal otomatis via ACME Stalwart (cek ±30 hari sebelum expiry).
- Monitor disk: `df -h` + `du -sh /opt/cloudsuite/data/*`.
- Update image: pin versi di compose (jangan `latest`), test di staging dulu.

---

## Lampiran A — Config Templates (Full Copy-Paste)

Semua config di bawah identik dengan repo `jetswu/cloudsuite` (committed).
Kalau clone repo (Bagian 3), file-file ini sudah tersedia — lampiran ini untuk
copy-paste manual / referensi cepat.

### A.1 docker-compose.yml

> PENTING: file ini berada di `/opt/cloudsuite/infra/docker/docker-compose.yml`
> (BUKAN root `/opt/cloudsuite/`). Path volume memakai absolut `/opt/cloudsuite/...`.
> Network `cloudsuite-net` external — buat manual (Bagian 2.4).

```yaml
services:
  postgres:
    image: postgres:16-alpine
    container_name: cloudsuite-postgres
    restart: unless-stopped
    environment:
      POSTGRES_DB: ${POSTGRES_DB}
      POSTGRES_USER: ${POSTGRES_USER}
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD}
      TZ: ${TZ}
    volumes:
      - /opt/cloudsuite/data/postgres:/var/lib/postgresql/data
    networks:
      - cloudsuite-net
    expose:
      - "5432"
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U $${POSTGRES_USER} -d $${POSTGRES_DB}"]
      interval: 10s
      timeout: 5s
      retries: 5
      start_period: 20s
    mem_limit: 1g
    cpus: 1.5

  redis:
    image: redis:7-alpine
    container_name: cloudsuite-redis
    restart: unless-stopped
    command: >
      sh -c 'exec redis-server
      --requirepass "$$REDIS_PASSWORD"
      --appendonly yes
      --maxmemory 512mb
      --maxmemory-policy allkeys-lru'
    environment:
      REDIS_PASSWORD: ${REDIS_PASSWORD}
    volumes:
      - /opt/cloudsuite/data/redis:/data
    networks:
      - cloudsuite-net
    expose:
      - "6379"
    healthcheck:
      test: ["CMD-SHELL", "redis-cli -a $${REDIS_PASSWORD} ping | grep -q PONG"]
      interval: 10s
      timeout: 5s
      retries: 5
      start_period: 10s
    mem_limit: 512m
    cpus: 0.5

  authentik-server:
    image: ghcr.io/goauthentik/server:2026.8.2
    container_name: cloudsuite-authentik-server
    restart: unless-stopped
    command: server
    environment:
      AUTHENTIK_SECRET_KEY: ${AUTHENTIK_SECRET_KEY}
      AUTHENTIK_POSTGRESQL__HOST: postgres
      AUTHENTIK_POSTGRESQL__USER: ${POSTGRES_USER}
      AUTHENTIK_POSTGRESQL__NAME: authentik
      AUTHENTIK_POSTGRESQL__PASSWORD: ${POSTGRES_PASSWORD}
      AUTHENTIK_REDIS__HOST: redis
      AUTHENTIK_REDIS__PASSWORD: ${REDIS_PASSWORD}
      AUTHENTIK_ERROR_REPORTING__ENABLED: "false"
      AUTHENTIK_DISABLE_UPDATE_CHECK: "true"
      AUTHENTIK_LISTEN__HTTP: 0.0.0.0:9000
      AUTHENTIK_LISTEN__HTTPS: 0.0.0.0:9443
      AUTHENTIK_LISTEN__METRICS: 0.0.0.0:9300
      AUTHENTIK_BOOTSTRAP_PASSWORD: ${AUTHENTIK_ADMIN_PASSWORD}
      AUTHENTIK_BOOTSTRAP_EMAIL: ${AUTHENTIK_ADMIN_EMAIL}
    volumes:
      - /opt/cloudsuite/data/authentik/media:/media
      - /opt/cloudsuite/data/authentik/certs:/certs
      - /opt/cloudsuite/data/authentik/templates:/templates
    networks:
      - cloudsuite-net
    expose:
      - "9000"
      - "9443"
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_healthy
    mem_limit: 2g
    cpus: 1.5

  authentik-worker:
    image: ghcr.io/goauthentik/server:2026.8.2
    container_name: cloudsuite-authentik-worker
    restart: unless-stopped
    command: worker
    user: root
    environment:
      AUTHENTIK_SECRET_KEY: ${AUTHENTIK_SECRET_KEY}
      AUTHENTIK_POSTGRESQL__HOST: postgres
      AUTHENTIK_POSTGRESQL__USER: ${POSTGRES_USER}
      AUTHENTIK_POSTGRESQL__NAME: authentik
      AUTHENTIK_POSTGRESQL__PASSWORD: ${POSTGRES_PASSWORD}
      AUTHENTIK_REDIS__HOST: redis
      AUTHENTIK_REDIS__PASSWORD: ${REDIS_PASSWORD}
      AUTHENTIK_ERROR_REPORTING__ENABLED: "false"
      AUTHENTIK_DISABLE_UPDATE_CHECK: "true"
      AUTHENTIK_LISTEN__HTTP: 0.0.0.0:9000
      AUTHENTIK_LISTEN__HTTPS: 0.0.0.0:9443
      AUTHENTIK_LISTEN__METRICS: 0.0.0.0:9300
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
      - /opt/cloudsuite/data/authentik/media:/media
      - /opt/cloudsuite/data/authentik/certs:/certs
      - /opt/cloudsuite/data/authentik/templates:/templates
    networks:
      - cloudsuite-net
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_healthy
    mem_limit: 1g
    cpus: 1.0

  nextcloud:
    image: nextcloud:34.0.3-apache
    container_name: cloudsuite-nextcloud
    restart: unless-stopped
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_healthy
    environment:
      POSTGRES_HOST: postgres
      POSTGRES_DB: ${NEXTCLOUD_DB_NAME}
      POSTGRES_USER: ${NEXTCLOUD_DB_USER}
      POSTGRES_PASSWORD: ${NEXTCLOUD_DB_PASSWORD}
      NEXTCLOUD_ADMIN_USER: ${NEXTCLOUD_ADMIN_USER}
      NEXTCLOUD_ADMIN_PASSWORD: ${NEXTCLOUD_ADMIN_PASSWORD}
      NEXTCLOUD_TRUSTED_DOMAINS: drive.idchsuite.my.id
      TRUSTED_PROXIES: cloudsuite-nginx
      OVERWRITEPROTOCOL: https
      OVERWRITECLIURL: https://drive.idchsuite.my.id
      REDIS_HOST: redis
      REDIS_HOST_PASSWORD: ${REDIS_PASSWORD}
      TZ: ${TZ}
    volumes:
      - /opt/cloudsuite/data/nextcloud:/var/www/html
    networks:
      - cloudsuite-net
    expose:
      - "80"
    mem_limit: 1.5g
    cpus: 1
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost/status.php"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 60s

  odoo:
    build:
      context: /opt/cloudsuite/infra/docker
      dockerfile: Dockerfile.odoo
    image: cloudsuite-odoo:18.0
    container_name: cloudsuite-odoo
    restart: unless-stopped
    depends_on:
      postgres:
        condition: service_healthy
    environment:
      HOST: postgres
      PORT: "5432"
      USER: ${ODOO_DB_USER}
      PASSWORD: ${ODOO_DB_PASSWORD}
      TZ: ${TZ}
    command: odoo --proxy-mode --without-demo=all
    volumes:
      - /opt/cloudsuite/data/odoo/filestore:/var/lib/odoo
      - /opt/cloudsuite/data/odoo/config:/etc/odoo
    networks:
      - cloudsuite-net
    expose:
      - "8069"
      - "8072"
    mem_limit: 2.5g
    cpus: 2
    healthcheck:
      test: ["CMD-SHELL", "curl -f http://localhost:8069/web/login || exit 1"]
      interval: 30s
      timeout: 10s
      retries: 5
      start_period: 180s

  stalwart:
    image: stalwartlabs/stalwart:v0.16.21
    container_name: cloudsuite-stalwart
    restart: unless-stopped
    depends_on:
      postgres:
        condition: service_healthy
    environment:
      TZ: ${TZ}
      STALWART_DB_PASSWORD: ${STALWART_DB_PASSWORD}
      CLOUDFLARE_API_TOKEN: ${CLOUDFLARE_API_TOKEN}
      STALWART_PUBLIC_URL: https://mail.idchsuite.my.id
      STALWART_HOSTNAME: mail.idchsuite.my.id
    volumes:
      - /opt/cloudsuite/data/stalwart:/etc/stalwart
      - /opt/cloudsuite/data/stalwart/lib:/var/lib/stalwart
    ports:
      - "25:25"
      - "465:465"
      - "587:587"
      - "143:143"
      - "993:993"
      - "995:995"
    expose:
      - "8080"
    networks:
      - cloudsuite-net
    mem_limit: 1g
    cpus: 1

  nginx:
    image: nginx:alpine
    container_name: cloudsuite-nginx
    restart: unless-stopped
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - /opt/cloudsuite/infra/nginx/conf.d:/etc/nginx/conf.d:ro
      - /opt/cloudsuite/infra/nginx/certs:/etc/nginx/certs:ro
      - /opt/cloudsuite/logs/nginx:/var/log/nginx
    networks:
      - cloudsuite-net
    mem_limit: 256m
    cpus: 0.5
    healthcheck:
      test: ["CMD-SHELL", "wget --spider -q http://localhost/health || exit 1"]
      interval: 30s
      timeout: 5s
      retries: 3
      start_period: 10s

  jmap-webmail:
    image: ghcr.io/root-fr/jmap-webmail:1.7.1
    container_name: cloudsuite-jmapwebmail
    restart: unless-stopped
    depends_on:
      stalwart:
        condition: service_healthy
    environment:
      - JMAP_SERVER_URL=${JMAPWEBMAIL_JMAP_URL}
      - OAUTH_ENABLED=true
      - OAUTH_ONLY=true
      - OAUTH_CLIENT_ID=${JMAPWEBMAIL_CLIENT_ID}
      - OAUTH_CLIENT_SECRET=${JMAPWEBMAIL_CLIENT_SECRET}
      - OAUTH_ISSUER_URL=${JMAPWEBMAIL_OIDC_ISSUER_URL}
      - TZ=Asia/Jakarta
    networks:
      - cloudsuite-net
    expose:
      - "3000"
    mem_limit: 512m
    cpus: 0.5
    healthcheck:
      test: ["CMD", "wget", "--spider", "-q", "http://localhost:3000/api/health"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 60s

networks:
  cloudsuite-net:
    external: true
```


### A.2 .env.example

> Copy ke `/opt/cloudsuite/.env`, chmod 600. Isi sesuai Bagian 4.

```bash
# ============================================================
# CloudSuite .env Template
# Copy ke .env, ganti semua CHANGE_ME
# Panduan generate secret: docs/SECRET-GENERATION.md
# Chicken-and-egg: var [LATE] baru bisa diisi SETELAH service jalan
# ============================================================

# === Domain & Network ===
DOMAIN=CHANGE_ME
PUBLIC_IP=CHANGE_ME
PRIVATE_IP=CHANGE_ME
TZ=Asia/Jakarta

# === Database (PostgreSQL) ===
POSTGRES_DB=cloudsuite
POSTGRES_USER=cloudsuite
POSTGRES_PASSWORD=CHANGE_ME_HEX64  # openssl rand -hex 32

# === Redis ===
REDIS_PASSWORD=CHANGE_ME_HEX64  # openssl rand -hex 32

# === Authentik (IdP) ===
AUTHENTIK_SECRET_KEY=CHANGE_ME_BASE64  # openssl rand -base64 60 | tr -d '\n'
AUTHENTIK_ADMIN_PASSWORD=CHANGE_ME_HEX64  # openssl rand -hex 32
AUTHENTIK_ADMIN_EMAIL=admin@CHANGE_ME
AUTHENTIK_API_TOKEN=CHANGE_ME_LATE  # [LATE] isi setelah Authentik jalan — lihat SECRET-GENERATION.md
AUTHENTIK_TEMPLATE_CLIENT_SECRET=CHANGE_ME_LATE  # [LATE] dari provider template-cloudsuite-services

# === Nextcloud (Drive) ===
NEXTCLOUD_ADMIN_USER=admin
NEXTCLOUD_ADMIN_PASSWORD=CHANGE_ME_HEX64  # openssl rand -hex 32
NEXTCLOUD_DB_NAME=nextcloud
NEXTCLOUD_DB_USER=cloudsuite
NEXTCLOUD_DB_PASSWORD=CHANGE_ME_HEX64  # openssl rand -hex 32
NEXTCLOUD_DB_HOST=postgres

# === Odoo (ERP) ===
ODOO_ADMIN_PASSWORD=CHANGE_ME_HEX64  # openssl rand -hex 32
ODOO_MASTER_PASSWORD=CHANGE_ME_HEX64  # openssl rand -hex 32
ODOO_DB_NAME=odoo
ODOO_DB_USER=cloudsuite
ODOO_DB_PASSWORD=CHANGE_ME_HEX64  # openssl rand -hex 32
ODOO_DB_HOST=postgres

# === Stalwart (Mail) ===
STALWART_ADMIN_USER=admin
STALWART_ADMIN_PASSWORD=CHANGE_ME_HEX64  # openssl rand -hex 32
STALWART_DB_NAME=stalwart
STALWART_DB_USER=cloudsuite
STALWART_DB_PASSWORD=CHANGE_ME_HEX64  # openssl rand -hex 32
STALWART_DB_HOST=postgres
STALWART_DOMAIN=CHANGE_ME  # root domain, mis. example.com
STALWART_MAIL_HOSTNAME=mail.CHANGE_ME
STALWART_TEST_PASSWORD=CHANGE_ME_HEX64  # openssl rand -hex 32
STALWART_API_KEY=CHANGE_ME_LATE  # [LATE] generate via stalwart-cli — lihat SECRET-GENERATION.md

# === jmap-webmail (Webmail) ===
JMAPWEBMAIL_CLIENT_ID=CHANGE_ME_LATE  # [LATE] dari Authentik provider stalwart-mail (= "stalwart-mail")
JMAPWEBMAIL_CLIENT_SECRET=CHANGE_ME_LATE  # [LATE] dari Authentik provider stalwart-mail
JMAPWEBMAIL_OIDC_ISSUER_URL=https://auth.CHANGE_ME/application/o/stalwart-mail  # TANPA trailing slash!
JMAPWEBMAIL_JMAP_URL=https://mail.CHANGE_ME

# === Cloudflare ===
CLOUDFLARE_API_TOKEN=CHANGE_ME  # scope: Zone:DNS:Edit untuk 1 zone — lihat SECRET-GENERATION.md
```


### A.3 Nginx Configs

> Lokasi: `/opt/cloudsuite/infra/nginx/conf.d/`. Semua vhost 443 pakai
> `/etc/nginx/certs/origin.pem` KECUALI mail.conf (`mail.pem` — cert LE hasil
> ACME Stalwart, Bagian 10.3). Lazy-resolve upstream (`resolver 127.0.0.11` +
> `set $...`) supaya `nginx -t` lolos sebelum container app ada.

#### A.3.1 auth.conf

```nginx
server {
    listen 80;
    server_name auth.idchsuite.my.id;

    location / {
        return 301 https://$host$request_uri;
    }

    location /health {
        access_log off;
        return 200 "OK\n";
        add_header Content-Type text/plain;
    }
}

server {
    listen 443 ssl;
    server_name auth.idchsuite.my.id;

    ssl_certificate /etc/nginx/certs/origin.pem;
    ssl_certificate_key /etc/nginx/certs/origin-key.pem;

    client_max_body_size 100M;

    resolver 127.0.0.11 valid=30s;
    set $auth_upstream http://authentik-server:9000;

    location / {
        proxy_pass $auth_upstream;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # WebSocket support
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";

        proxy_connect_timeout 60s;
        proxy_send_timeout 60s;
        proxy_read_timeout 60s;
    }
}
```

#### A.3.2 drive.conf

```nginx
server {
    listen 80;
    server_name drive.idchsuite.my.id;

    location / {
        return 301 https://$host$request_uri;
    }

    location /health {
        access_log off;
        return 200 "OK\n";
        add_header Content-Type text/plain;
    }
}

server {
    listen 443 ssl;
    server_name drive.idchsuite.my.id;

    ssl_certificate /etc/nginx/certs/origin.pem;
    ssl_certificate_key /etc/nginx/certs/origin-key.pem;

    client_max_body_size 10G;

    resolver 127.0.0.11 valid=30s;
    set $drive_upstream http://nextcloud:80;

    location / {
        proxy_pass $drive_upstream;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header X-Forwarded-Host $host;

        # WebSocket support
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";

        proxy_connect_timeout 60s;
        proxy_send_timeout 3600s;
        proxy_read_timeout 3600s;
    }
}
```

#### A.3.3 erp.conf

```nginx
# Odoo ERP (CloudSuite staging) -- Sprint 0.9
# Upstream lazy-resolve agar nginx -t lolos sebelum container odoo ada.

server {
    listen 443 ssl;
    server_name erp.idchsuite.my.id;

    ssl_certificate /etc/nginx/certs/origin.pem;
    ssl_certificate_key /etc/nginx/certs/origin-key.pem;

    client_max_body_size 200M;

    resolver 127.0.0.11 valid=30s;
    set $erp_upstream http://odoo:8069;

    location / {
        proxy_pass $erp_upstream;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header X-Forwarded-Host $host;

        # WebSocket / longpolling support
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";

        proxy_connect_timeout 60s;
        proxy_send_timeout 720s;
        proxy_read_timeout 720s;
    }
}
```

#### A.3.4 mail.conf

```nginx
# Stalwart Mail (CloudSuite staging) -- Sprint 0.10a
# WebUI + API di belakang Nginx; SMTP/IMAP/POP direct ke container.
# Upstream lazy-resolve agar nginx -t lolos sebelum container stalwart ada.

server {
    listen 443 ssl;
    server_name mail.idchsuite.my.id;

    ssl_certificate /etc/nginx/certs/mail.pem;
    ssl_certificate_key /etc/nginx/certs/mail-key.pem;

    client_max_body_size 100M;

    resolver 127.0.0.11 valid=30s;
    set $mail_upstream http://stalwart:8080;

    location / {
        # CORS untuk jmap-webmail (webmail.idchsuite.my.id) -- Sprint 0.10d
        add_header 'Access-Control-Allow-Origin' 'https://webmail.idchsuite.my.id' always;
        add_header 'Access-Control-Allow-Methods' 'GET, POST, OPTIONS' always;
        add_header 'Access-Control-Allow-Headers' 'Authorization, Content-Type' always;
        add_header 'Access-Control-Max-Age' 1728000 always;

        if ($request_method = 'OPTIONS') {
            return 204;
        }

        proxy_pass $mail_upstream;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header X-Forwarded-Host $host;

        # WebSocket support
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";

        proxy_connect_timeout 60s;
        proxy_send_timeout 3600s;
        proxy_read_timeout 3600s;
    }
}
```

#### A.3.5 webmail.conf

```nginx
# jmap-webmail (CloudSuite staging) -- Sprint 0.10d CLEAN SLATE
# Reverse proxy ke jmap-webmail (root-fr/jmap-webmail) di port 3000.
# Upstream lazy-resolve agar nginx -t lolos sebelum container jmap-webmail ada.

server {
    listen 443 ssl;
    server_name webmail.idchsuite.my.id;

    ssl_certificate /etc/nginx/certs/origin.pem;
    ssl_certificate_key /etc/nginx/certs/origin-key.pem;

    client_max_body_size 100M;

    resolver 127.0.0.11 valid=30s;
    set $jmapwebmail_upstream http://jmap-webmail:3000;

    location / {
        proxy_pass $jmapwebmail_upstream;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header X-Forwarded-Host $host;

        # WebSocket support
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";

        proxy_connect_timeout 60s;
        proxy_send_timeout 300s;
        proxy_read_timeout 300s;
    }
}
```

#### A.3.6 default.conf

```nginx
server {
    listen 80;
    server_name _;

    location /health {
        access_log off;
        return 200 "OK\n";
        add_header Content-Type text/plain;
    }

    location / {
        return 503 "CloudSuite — service belum siap.\n";
        add_header Content-Type text/plain;
    }
}
```


### A.4 Dockerfile.odoo

> Lokasi: `/opt/cloudsuite/infra/docker/Dockerfile.odoo`.

```dockerfile
# Odoo ERP custom image (CloudSuite staging) -- Sprint 0.9 revisi
# Base 18.0 (sesuai compose) + OCA auth_oidc untuk OIDC authorization-code flow.
FROM odoo:18.0

USER root
RUN pip install --no-cache-dir --break-system-packages \
        python-jose \
        odoo-addon-auth-oidc==18.0.1.1.0.2
USER odoo
```


### A.5 odoo.conf.example

> Copy ke `/opt/cloudsuite/data/odoo/config/odoo.conf`, isi password, lalu
> `chown 100:101` + `chmod 600` (Bagian 9.2).

```ini
[options]
addons_path = /mnt/extra-addons
data_dir = /var/lib/odoo
admin_passwd = CHANGE_ME
db_host = postgres
db_port = 5432
db_user = cloudsuite
db_password = CHANGE_ME
list_db = False
log_level = info
dbfilter = ^odoo$
```


### A.6 Stalwart config.json (example)

> Lokasi live: `/opt/cloudsuite/data/stalwart/config.json` (owner 2000:2000,
> mode 600). HANYA DataStore — Stalwart menolak config lain di file ini.

```json
{
  "@type": "PostgreSql",
  "host": "postgres",
  "port": 5432,
  "database": "stalwart",
  "authUsername": "cloudsuite",
  "authSecret": {
    "@type": "EnvironmentVariable",
    "variableName": "STALWART_DB_PASSWORD"
  }
}
```


### A.7 Stalwart NDJSON (listeners + task DNS)

> Lokasi: `/opt/cloudsuite/infra/docker/stalwart-listeners.ndjson` dan
> `stalwart-task-dns.ndjson`. Apply via stalwart-cli (Bagian 10.6/10.7).
> Restart container setelah apply listener (tidak hot-reload).

#### A.7.1 stalwart-listeners.ndjson

```json
{"@type":"upsert","object":"NetworkListener","matchOn":["name"],"value":{"submission":{"name":"submission","bind":{"[::]:587":true},"protocol":"smtp","useTls":true,"tlsImplicit":false}}}
{"@type":"upsert","object":"NetworkListener","matchOn":["name"],"value":{"imap":{"name":"imap","bind":{"[::]:143":true},"protocol":"imap","useTls":true,"tlsImplicit":false}}}
```

#### A.7.2 stalwart-task-dns.ndjson

```json
{"@type":"create","object":"Task","value":{"task-dns":{"@type":"DnsManagement","domainId":"b","updateRecords":{"dkim":true,"tlsa":true,"spf":true,"mx":true,"dmarc":true,"srv":true,"mtaSts":true,"tlsRpt":true,"caa":true,"autoConfig":true,"autoConfigLegacy":true,"autoDiscover":true},"onSuccessRenewCertificate":false}}}
```

---

## Lampiran B — Troubleshooting Cepat

Gejala → penyebab → fix. Format: GEJALA / PENYEBAB / FIX.

### B.1 Webmail — "Authentication Failed" setelah login Authentik

- Penyebab paling sering: `Authentication.directoryId` di Stalwart belum di-set ke Directory OIDC (null = SSO gagal silent).
- Fix: Bagian 10.8 langkah 3. Ada delay propagasi ±15 menit; kalau masih gagal, `docker compose restart stalwart`.
- Penyebab lain: issuerUrl Directory TANPA slash (harus DENGAN slash — match `iss` token).

### B.2 Redis — WRONGPASS / NOAUTH

- Redis password di compose dibaca dari env `REDIS_PASSWORD` saat FIRST start.
- Fix fresh install: benarkan `.env` sebelum `up` pertama. Fix existing: cek `.env` vs `docker exec cloudsuite-redis env | grep REDIS`.

### B.3 Authentik — restart loop, log `error 97 EAFNOSUPPORT`

- VPS tanpa IPv6; Authentik bind default `[::]`.
- Fix: env `AUTHENTIK_LISTEN__HTTP: 0.0.0.0:9000` (+HTTPS 9443 +METRICS 9300) di server DAN worker (sudah builtin di compose repo — Bagian 7.2).

### B.4 Odoo — Database selector muncul di /web/login

- `dbfilter`/`list_db` belum benar di odoo.conf.
- Fix: `dbfilter = ^odoo$`, `list_db = False`, restart odoo. DB `odoo` harus sudah ada (Bagian 6.2).

### B.5 Stalwart — login admin 401 (webadmin/CLI Basic)

- SETELAH `directoryId=OIDC` ini NORMAL BY DESIGN (semua password login diarahkan OIDC).
- Fix: pakai `--api-key "$STALWART_API_KEY"`. Kalau ApiKey hilang: recovery mode `STALWART_RECOVERY_ADMIN` (Bagian 10.4).

### B.6 CORS error browser (webmail fetch auth.*.well-known)

- jmap-webmail fetch discovery client-side (browser) → cross-origin.
- Fix: header CORS di `mail.conf` (sudah builtin) — cek `Access-Control-Allow-Origin https://webmail.<domain>` + preflight OPTIONS 204. Bukan Cloudflare Transform Rule.

### B.7 SSH — Permission denied (publickey)

- Key tidak terpasang / salah user.
- Fix: `ssh -v` untuk debug; cek `~/.ssh/authorized_keys` + permission 700/.ssh 600/keys; untuk GitHub: `ssh -T git@github.com`.

### B.8 Nextcloud SSO — provider terbuat tapi clientId kosong

- Pakai format OCC lama `config:app:set`.
- Fix: `php occ user_oidc:provider CloudSuite --clientid=... --clientsecret=... --discoveryuri=... --unique-uid=1` (Bagian 8.5).

### B.9 Authentik authorize — `invalid_request` / log "Invalid grant_type"

- `grant_types` provider kosong (default API/UI lama).
- Fix: isi eksplisit (Nextcloud: semua 7; Odoo: authorization_code+refresh_token; stalwart-mail: authorization_code+refresh_token) — Bagian 8.4/9.6/11.1.

### B.10 Odoo SSO — stuck di `/web/login_successful`

- User OAuth baru dibuat dari template Portal (default Odoo) → bukan internal.
- Fix: template user internal (Bagian 9.5) + hapus user Odoo rusak + re-login.

### B.11 Docker compose — `no configuration file provided`

- Menjalankan compose dari folder yang salah.
- Fix: `cd /opt/cloudsuite/infra/docker` dulu, atau pakai `-f /opt/cloudsuite/infra/docker/docker-compose.yml --env-file /opt/cloudsuite/.env`.

### B.12 Odoo build — `externally-managed-environment` (PEP 668)

- pip global diblokir image Debian.
- Fix: Dockerfile repo sudah pakai `--break-system-packages` — jangan edit keluar.

### B.13 Mail port mati dari luar (25/587/993 timeout)

- `mail.*` masih Proxied di Cloudflare.
- Fix: set DNS only (Bagian 5.2) + cek UFW rule.

### B.14 Stalwart — config.json membuat container gagal start

- Isi config.json selain DataStore (bootstrap/acme/certificates) → reject.
- Fix: config.json hanya 3 blok (Bagian 10.1); semua konfigurasi lain via CLI/NDJSON ke DB.

### B.15 DKIM tidak sign outgoing mail

- `dkimManagement` bukan Automatic / stage key masih `pending`.
- Fix: Domain `dkimManagement: {"@type":"Automatic"}`, key stage `active`, restart stalwart; verifikasi header DKIM-Signature di test mail.

### B.16 Git — hermes/admin key beda akun

- Repo clone pakai user lain dari pemilik key.
- Fix: clone sebagai user yang punya key GitHub di `~/.ssh/`, atau daftarkan key baru.

---

## Lampiran C — Checklist Ringkas

Print / copy bagian ini. Centang tiap langkah selesai + verified.

### Setup
- [ ] 1. VPS: update + timezone + hostname
- [ ] 2. User admin + sudo + SSH key
- [ ] 3. SSH hardening (root login OFF, password OFF) — verifikasi terminal baru!
- [ ] 4. UFW 9 port (22/80/443/25/465/587/143/993/995)
- [ ] 5. Docker + compose plugin + `cloudsuite-net`
- [ ] 6. Clone repo + struktur /opt/cloudsuite + copy config

### Secrets & DNS
- [ ] 7. .env Kategori 1 terisi + chmod 600 + disimpan di password manager
- [ ] 8. Cloudflare: 5 A record (mail = DNS only!), SSL Full (strict)
- [ ] 9. Origin cert terpasang (origin.pem + origin-key.pem, key 600)

### Deploy
- [ ] 10. postgres + redis healthy
- [ ] 11. DB authentik/odoo/stalwart dibuat
- [ ] 12. Authentik up + admin password diganti + brand/groups/token/template
- [ ] 13. Nextcloud up + installed:true + user_oidc + provider + SSO test
- [ ] 14. Odoo up + odoo.conf 100:101/600 + template user internal + provider + SSO test
- [ ] 15. Stalwart up + config.json DataStore-only + bootstrap NDJSON + listener 587/143
- [ ] 16. DNS records published (MX/SPF/DKIM/DMARC/SRV/MTA-STS) + DKIM test pass
- [ ] 17. ApiKey dibuat + directoryId=OIDC + recovery admin disimpan & dihapus dari compose
- [ ] 18. jmap-webmail up + provider stalwart-mail + SSO test (login EMAIL)

### Verifikasi Akhir
- [ ] 19. `docker compose ps` → 9 container healthy
- [ ] 20. curl auth/drive/erp/webmail → semua 200
- [ ] 21. TLS mail 993/587 OK
- [ ] 22. Mail internal + external (Gmail) DKIM pass
- [ ] 23. Tidak ada secret di git: `cd ~/cloudsuite && git ls-files | grep -E '^\.env$'` → kosong
- [ ] 24. `.env` 600: `ls -l /opt/cloudsuite/.env`

### Dokumen Terkait (referensi lanjutan, TIDAK wajib untuk deploy)

Dokumen detail di repo `docs/`: `DEPLOYMENT.md` (deploy per-service),
`LESSONS-LEARNED.md` (tribal knowledge), `TROUBLESHOOTING.md` (24 kasus detail),
`SECRET-GENERATION.md`, `authentik-setup.md`, `stalwart-notes.md`,
`jmap-webmail-notes.md`, `nextcloud-notes.md`, `odoo-notes.md`.

---

*Panduan ini dibuat Sprint 0.10k (2026-09-11). Sumber: staging CloudSuite live — semua config diverifikasi dari repo `jetswu/cloudsuite` commit terbaru. Item [TBC] = belum terverifikasi di staging.*
