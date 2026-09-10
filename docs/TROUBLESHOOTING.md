# CloudSuite — Troubleshooting

> Terakhir diperbarui: 2026-09-10 (Sprint 0.9e).
> Setiap item: gejala, root cause, fix, pencegahan. Semua terverifikasi dari
> staging kecuali yang bertanda [perlu verifikasi].

## 1. Authentik — IPv6 Crash Loop

- Gejala: `cloudsuite-authentik-server` restart-loop sekitar 45 detik.
  `docker logs` memuat `error 97 EAFNOSUPPORT` di `server.rs:33`.
- Root cause: kernel staging IPv6 mati total; Authentik default bind
  `[::]:9000/9443/9300` sehingga bind gagal.
- Fix: override di KEDUA service (server dan worker):

      AUTHENTIK_LISTEN__HTTP: 0.0.0.0:9000
      AUTHENTIK_LISTEN__HTTPS: 0.0.0.0:9443
      AUTHENTIK_LISTEN__METRICS: 0.0.0.0:9300

  Lalu `docker compose up -d authentik-server authentik-worker`.
- Pencegahan: jangan hapus tiga baris ini saat edit compose; cek log
  `docker logs cloudsuite-authentik-server --tail 20` tiap deploy.

## 2. Redis — WRONGPASS

- Gejala: service yang konek ke redis gagal dengan `WRONGPASS invalid
  username-password pair`.
- Root cause: password redis di `/opt/cloudsuite/.env` (`REDIS_PASSWORD`)
  berisi karakter khusus (mis. `+`, `/`, `=`). Bila command redis-server
  di compose di-inject langsung (`--requirepass $REDIS_PASSWORD` tanpa
  quoting), shell memecah/mengubah password saat di-interpolasi sehingga
  nilai yang benar-benar dipakai redis berbeda dari yang dipakai client.
  Jadi akar masalahnya **quoting**, bukan sekadar "password beda".
- Fix (pola yang sudah benar di staging):
  generate password hex tanpa karakter khusus, lalu pakai `sh -c` +
  environment variable:

      # generate password hex 64 char (aman untuk shell/redis)
      openssl rand -hex 32

      # di docker-compose.yml service redis:
      command: >
        sh -c 'exec redis-server --requirepass "$$REDIS_PASSWORD" ...'
      environment:
        REDIS_PASSWORD: ${REDIS_PASSWORD}

  `$$REDIS_PASSWORD` (double dollar) menunda interpolasi ke shell dalam
  container, bukan ke shell host saat `docker compose` baca file, sehingga
  nilai persis dari env yang dipakai. Lalu restart redis + client:

      cd /opt/cloudsuite/infra/docker
      docker compose up -d redis
      docker compose up -d authentik-server authentik-worker nextcloud

- Pencegahan: selalu generate password redis pakai hex
  (`openssl rand -hex 32`); jangan inject password langsung ke `command`
  tanpa quoting `sh -c`; satu sumber `REDIS_PASSWORD`; healthcheck redis
  `redis-cli -a "$$REDIS_PASSWORD" ping` ikut pola yang sama.

## 3. Authentik — Token 403 setelah Rotate SECRET_KEY

- Gejala: setelah `AUTHENTIK_SECRET_KEY` diganti, semua API token lama
  menghasilkan 403 / `Token invalid/expired`.
- Root cause: secret dipakai menandatangani token; rotate menginvalidasi
  semua token lama.
- Fix: buat ulang token `hermes-automation` via prosedur rotate 8 langkah
  di docs/authentik-setup.md (token sementara, hapus lama, buat baru,
  update .env, hapus sementara, verifikasi `users/me` 200).
- Pencegahan: rotate SECRET_KEY hanya saat darurat; siapkan jendela
  recreate token segera setelahnya.

## 4. Authentik — ak set_password Tidak Ada

- Gejala: `docker exec ... ak set_password ...` gagal — subcommand tidak ada.
- Root cause: CLI Authentik tidak punya `set_password`; yang ada
  `changepassword` dan `shell`.
- Fix (salah satu):

      printf '<PASS>\n<PASS>\n' | docker exec -i cloudsuite-authentik-server ak changepassword akadmin
      docker exec -i cloudsuite-authentik-server ak shell  # lalu set_password() + save()

- Pencegahan: catat bahwa `AUTHENTIK_BOOTSTRAP_PASSWORD` hanya first-run;
  reset selalu via dua cara di atas.

## 5. Authentik — OIDC Provider grant_types Kosong

- Gejala: authorize gagal `invalid_request The request is otherwise
  malformed` plus log `Invalid grant_type for provider`.
- Root cause: API default `grant_types` kosong (Authentik 2026.8.2,
  terverifikasi dari source `views/authorize.py` baris 233).
- Fix: isi eksplisit via API saat buat provider:

      authorization_code, hybrid, implicit, client_credentials, password, device_code, refresh_token

  Untuk Odoo cukup `authorization_code` plus `refresh_token`.
- Pencegahan: selalu sertakan `authorization_flow`, `invalidation_flow`,
  `signing_key`, dan `grant_types` pada `POST /api/v3/providers/oauth2/`
  (tanpa `invalidation_flow` menghasilkan HTTP 400).

## 6. Nextcloud — OCC Format Lama

- Gejala: provider OIDC terbuat tapi `clientId` kosong; login SSO gagal.
- Root cause: memakai format lama `config:app:set` yang tidak mengisi
  field provider user_oidc 8.x.
- Fix: pakai format OCC 8.x sebagai user www-data:

      docker exec -u www-data cloudsuite-nextcloud php occ user_oidc:provider CloudSuite \
        --clientid=<ID> --clientsecret=<SECRET> \
        --discoveryuri=https://auth.idchsuite.my.id/application/o/nextcloud/.well-known/openid-configuration \
        --unique-uid=1

- Pencegahan: jangan pakai `config:app:set` untuk user_oidc 8.x.

## 7. Odoo — Database Selector Muncul

- Gejala: `/web/login` menampilkan pemilih database / manager
  (`/web/database/selector`).
- Root cause: `list_db` True atau tanpa `dbfilter` sehingga semua DB
  (authentik, cloudsuite, nextcloud, odoo) terekspos.
- Fix di odoo.conf live (`/opt/cloudsuite/data/odoo/config/odoo.conf`):

      list_db = False
      dbfilter = ^odoo$

  Lalu restart odoo. Verifikasi selector count 0 untuk DB non-odoo.
- Pencegahan: selalu set kedua opsi ini; repo hanya simpan
  `odoo.conf.example`.

## 8. Odoo — User SSO Jadi Portal (Redirect /web/login_successful)

- Gejala: user SSO baru nyangkut di `/web/login_successful` walau
  `action_id=109`.
- Root cause: Odoo 18 membuat user OAuth dari `base.template_portal_user_id`
  (default id=5 `portaltemplate`, share=True) sehingga
  `is_user_internal()` False dan `home.py _login_redirect` mengarah ke
  `/web/login_successful`. Opsi lama `auth_signup.default_template_user_id`
  bernilai False dan tidak dibaca kode (`auth_signup/models/res_users.py:138`,
  via `auth_oauth ... res_users.py:112-114` dan `_create_user_from_template`).
- Fix staging: buat user `cloudsuite_template` id=8 (active=True, email
  `template@cloudsuite.local`, grup HANYA Internal User + Technical Features,
  share=False), set `base.template_portal_user_id` 5 ke 8, hapus user SSO
  lama (`unlink`), re-login. Hasil: user id=9 share=False grup Internal
  action_id=109 masuk Discuss tanpa stuck.
- Pencegahan: JANGAN pakai admin sebagai template (privilege escalation);
  template minimal internal saja.

## 9. Odoo — auth_oidc Native Tidak Ada

- Gejala: menu OAuth/OIDC tidak ada di Odoo vanilla.
- Root cause: Odoo 18 community tidak menyertakan provider OIDC generik.
- Fix: install modul OCA via Dockerfile.odoo:

      pip install --no-cache-dir --break-system-packages python-jose odoo-addon-auth-oidc==18.0.1.1.0.2

- Pencegahan: pin versi modul di Dockerfile; rebuild via `--build`.

## 10. Odoo — PEP 668 externally-managed-environment

- Gejala: `pip install` gagal dengan error `externally-managed-environment`.
- Root cause: Debian 13 menerapkan PEP 668 — pip sistem dikunci.
- Fix: tambah flag `--break-system-packages` (sudah di Dockerfile.odoo).
- Pencegahan: jangan buat venv di image; flag ini wajib tiap pip install.

## 11. Odoo — NoSectionError dan Permission denied (owner 100:101)

- Gejala: Odoo gagal baca config: `NoSectionError` / `Permission denied`.
- Root cause: `odoo.conf` live bukan milik UID/GID user odoo di image
  (100:101) atau mode terlalu terbuka/salah.
- Fix:

      chown 100:101 /opt/cloudsuite/data/odoo/config/odoo.conf
      chmod 600 /opt/cloudsuite/data/odoo/config/odoo.conf

- Pencegahan: selalu set owner/mode ini tiap tulis ulang odoo.conf.

## 12. Git — SSH Permission denied (publickey)

- Gejala: `git clone/push` via SSH gagal `Permission denied (publickey)`.
- Root cause: key GitHub belum terdaftar untuk user hermes, atau remote
  masih HTTPS, atau permission key salah.
- Fix: key GitHub untuk hermes ada di `~/.ssh/github` (bukan `id_ed25519`).
  Buat sekali dengan `ssh-keygen -t ed25519 -f ~/.ssh/github` (tanpa
  passphrase bila untuk automation), daftarkan public key (`~/.ssh/github.pub`)
  ke repo, pastikan `chmod 600 ~/.ssh/github`. Sertakan `~/.ssh/config`:

      Host github.com
          HostName github.com
          User git
          IdentityFile ~/.ssh/github
          IdentitiesOnly yes

  Lalu pastikan remote SSH:

      git remote set-url origin git@github.com:jetswu/cloudsuite.git

- Pencegahan: test sekali setelah setup `ssh -T git@github.com` (harus
  membalas `Hi <user>! You've successfully authenticated...`); jangan
  campur identitas dengan key lain — `IdentitiesOnly yes` memaksa pakai
  `~/.ssh/github` saja.

## 13. Hermes — Iteration Budget Exhausted

- Gejala: sesi berhenti di tengah kerja karena iterasi habis, tool output
  banner `exit 0, 1 lines` berulang tanpa hasil.
- Root cause: output SSH ber-banner (`HermestAlan Ubuntu 24.04.4`) memakan
  konteks; retry tanpa backoff.
- Fix: patuhi aturan iteration di atas 100/150 — LAPOR PROGRESS dulu sebelum
  lanjut; pecah task besar jadi sub-task; pakai `LogLevel=ERROR` plus `tail`.
- Pencegahan: batching read-only calls; satu batch satu tujuan.

## 14. Hermes — Halusinasi (sebut layanan tidak ada)

- Gejala: laporan menyebut file/container/service yang tidak ada.
- Root cause: klaim tanpa verifikasi `ls` / `docker ps` / `git log`.
- Fix: aturan anti-halusinasi — sebut hanya yang sudah diverifikasi,
  kalau belum tulis `belum diverifikasi`; kalau tidak yakin tulis
  `tidak yakin`; tiap klaim wajib bukti path/output/screenshot.
- Pencegahan: JANGAN tambah scope/fitur tanpa izin; tanya dulu.

## 15. Hermes — Over-Engineering (base64 untuk copy file)

- Gejala: copy file kecil via base64 encode/decode berlapis-lapis.
- Root cause: kebiasaan defensif tanpa alasan (file kecil, koneksi sehat).
- Fix: untuk file kecil pakai heredoc/`scp` langsung; base64 hanya bila
  binary atau koneksi terbukti korup.
- Pencegahan: prinsip user — `jangan over engineering, jangan halu`.

## 16. Laporan Hermes — Jangan Tampilkan Secret

- Gejala: nilai token/password muncul di laporan chat (bisa bocor).
- Root cause: output mentah di-copy-paste tanpa redaksi.
- Fix: tampilkan hanya nama token, lokasi .env, tanggal/expiry, status
  verifikasi HTTP code, plus kalimat `Nilai TIDAK ditampilkan — admin
  ambil dari .env`. Token yang terlanjur muncul = wajib di-rotate.
- Pencegahan: `grep -i 'password\|secret\|token'` tiap draft docs; pastikan
  yang muncul hanya placeholder `<SECRET>` / `<IP_VPS>`.

## 17. Cloudflare — Mail Subdomain (DNS only)

- Gejala: [perlu verifikasi — ditulis menjelang Sprint 0.10] record mail
  (MX/SPF/DKIM/DMARC) berisiko rusak bila melewati proxy Cloudflare.
- Root cause: proxy (awan oranye) hanya untuk HTTP(S); record mail harus
  DNS-only (awan abu-abu).
- Fix: set semua record terkait mail (`mail`, MX, TXT SPF/DKIM/DMARC)
  ke DNS-only saat Sprint 0.10.
- Pencegahan: SSL mode Full (strict) tetap untuk subdomain web.

## 18. Sudo — sudo -u hermes gagal untuk git

- Gejala: `sudo -u hermes git ...` dari user admin gagal (SSH agent /
  HOME ikut admin).
- Root cause: environment SSH (`SSH_AUTH_SOCK`, `HOME/.ssh`) milik admin,
  bukan hermes.
- Fix: jalankan git sebagai hermes langsung via SSH (`ssh hermes@...`),
  bukan `sudo -u`. Pola kerja: agent SSH ke staging sebagai hermes.
- Pencegahan: jangan campur sesi admin dan hermes untuk operasi git.

## 19. General — Copy File /opt dan Repo

- Gejala: config live dan repo drift (berbeda isi tanpa catatan).
- Root cause: dua lokasi (`/opt/cloudsuite` live, `~/cloudsuite` repo)
  disinkron manual tanpa pola tetap.
- Fix: sinkron manual per sprint; perubahan DB-only skip commit kecuali
  docs; backup `.env` tiap ubah (`cp .env .env.bak-<tanggal>`).
- Pencegahan: commit message prefix jelas (`fix(erp):`, `docs(erp):`,
  `docs:`, `chore(drive):`).

## 20. General — Environment Variables di Compose

- Gejala: container gagal start dengan env kosong / `variable is not set`.
- Root cause: compose dijalankan tanpa `--env-file`, atau key hilang di .env.
- Fix: selalu sertakan `--env-file /opt/cloudsuite/.env` bila tidak `cd`
  ke folder compose; cek daftar key Bagian 7 DEPLOYMENT.md via
  `grep -E '^[A-Z_]+=' /opt/cloudsuite/.env | cut -d= -f1` (hanya nama key).
- Pencegahan: validasi key sebelum `up -d`.

## 21. Docker Compose — no configuration file provided

- Gejala: `docker compose ps` dari `/opt/cloudsuite` menghasilkan
  `no configuration file provided: not found`.
- Root cause: file compose ada di subfolder `infra/docker/`, bukan root
  `/opt/cloudsuite/`.
- Fix: `cd` ke folder yang benar, atau pakai flag `-f`:

      cd /opt/cloudsuite/infra/docker && docker compose ps
      docker compose -f /opt/cloudsuite/infra/docker/docker-compose.yml --env-file /opt/cloudsuite/.env ps

- Pencegahan: ingat Bagian 6 DEPLOYMENT.md — compose HANYA di
  `infra/docker/docker-compose.yml`.

## 22. Nginx — Healthcheck

- Gejala: `docker compose ps` menampilkan nginx `Up` tanpa label
  `(healthy)`.
- Root cause: healthcheck belum ditambah ke service nginx di compose.
- Fix: tambahkan blok berikut ke service nginx di compose (sudah diterapkan
  staging saat Sprint 0.9e):

      healthcheck:
        test: ["CMD-SHELL", "wget --spider -q http://localhost/health || exit 1"]
        interval: 30s
        timeout: 5s
        retries: 3
        start_period: 10s

  Endpoint `/health` sudah ada di `default.conf` (`return 200 "OK"`).
  Terapkan lalu verifikasi:

      cd /opt/cloudsuite/infra/docker
      docker compose up -d nginx
      docker compose ps nginx   # → harus menampilkan (healthy)

- Pencegahan: jaga endpoint `/health` tetap ada; healthcheck nginx tidak
  butuh dependensi service lain (loopback sendiri).


---

## 23. config.json isi Bootstrap -> Stalwart reject (Sprint 0.10a)

- Gejala: config.json berisi "bootstrap": true atau field ACME/certificates ->
  container restart loop atau gagal start.
- Penyebab: config.json harusnya DataStore only (data, registry, tracelogging).
  Field Bootstrap (acme, certificates, system) bukan di config.json.
- Solusi: reset config.json ke DataStore only:
  {"data":{"@type":"Local","path":"/var/lib/stalwart","purge":{"@type":"Never"}},
   "registry":{"@type":"Local","configKey":"STALWART_LICENSE"},
   "tracelogging":{"@type":"Server"}}
- Pencegahan: jangan campur Bootstrap + DataStore di satu file.

## 24. Port 587/143 default OFF (Sprint 0.10a)

- Gejala: compose map 587:587 dan 143:143 tapi tidak listening.
- Penyebab: Stalwart v0.16.21 default listener hanya bind 25, 465, 993, 995
  (465=smtps implicit, 993=imaps implicit). 587 (submission/STARTTLS) dan
  143 (imap/STARTTLS) tidak auto-created -- harus apply manual.
- Solusi: apply NDJSON NetworkListener untuk 587 + 143, lalu restart container
  (Stalwart tidak hot-reload listener baru).
- Pencegahan: cek /proc/net/tcp setelah restart untuk konfirmasi binding.

## 25. DKIM stage pending perlu trigger publish (Sprint 0.10a)

- Gejala: DKIM keys sudah generate tapi stage pending -- tidak ada record di DNS.
- Penyebab: DKIM auto-generate saat server normal (background task), tapi
  publish ke DNS butuh Task DnsManagement.
- Solusi: apply Task DnsManagement (lihat docs/stalwart-notes.md).
  Task hilang dari query setelah selesai (scheduler auto-delete).
  Cek via Cloudflare API atau dig untuk konfirmasi publish.
- Pencegahan: set dnsManagement.auto=true di Domain agar auto-publish.

## 26. Duration field harus integer milidetik (Sprint 0.10a)

- Gejala: apply NDJSON gagal dengan error validasi pada field durasi.
- Penyebab: Stalwart serialisasi durasi sebagai integer milidetik, bukan detik.
  timeout:30000 = 30 detik, bukan 30.
- Pencegahan: selalu pakai milidetik: 30000 (30s), 300000 (5m).

## 27. authUsername bukan user (Sprint 0.10a)

- Gejala: apply NDJSON Account gagal -- "unknown field: user".
- Penyebab: field di Account object bernama authUsername, bukan user (legacy
  name dari Stalwart versi lama).
- Pencegahan: describe <Object> sebelum apply untuk cek field names.

## 28. Stalwart admin API tidak bind host 127.0.0.1 di normal mode (Sprint 0.10a)

- Gejala: curl http://127.0.0.1:8080 dari host gagal (connection refused).
- Penyebab: default HTTP listener bind [::]:8080 di dalam container, tapi
  admin API hanya bisa diakses via container IP (172.18.0.9:8080).
- Solusi: gunakan container IP atau docker exec curl localhost:8080.
- Pencegahan: catat container IP di dokumentasi deployment.

### 29. DKIM-Signature tidak muncul di outgoing email

**Gejala:** Email dikirim dari Stalwart, tapi header `DKIM-Signature` tidak ada. SPF/DMARC pass (karena SPF align), tapi DKIM tidak ada signature.

**Root Cause:**
1. `dkimManagement` tidak di-set di Domain config -> signing disabled
2. DKIM signature stage = `pending` -> signing hanya aktif saat stage = `active`

**Fix:**
```bash
# Cek config
stalwart-cli query Domain --fields name,dkimManagement --json
stalwart-cli query DkimSignature --fields selector,stage --json

# Fix 1: Set dkimManagement
echo '{"@type":"update","object":"Domain","id":"b","value":{"dkimManagement":{"@type":"Automatic"}}}' | stalwart-cli apply

# Fix 2: Set stage ke active
echo '{"@type":"update","object":"DkimSignature","id":"<id>","value":{"stage":"active"}}' | stalwart-cli apply

# Restart
cd /opt/cloudsuite/infra/docker && docker compose --env-file /opt/cloudsuite/.env restart stalwart
```

**Verifikasi:** `DKIM-Signature` header muncul di email yang dikirim.

---


## Bulwark SSO (Sprint 0.10b)

### Bulwark returns blank page or redirect loop
- **Cause**: OAUTH_ISSUER_URL trailing slash mismatch
- **Fix**: OAUTH_ISSUER_URL harus ADA trailing slash: `https://auth.idchsuite.my.id/application/o/stalwart-mail/`
- **Verify**: `docker exec cloudsuite-bulwark env | grep OAUTH_ISSUER_URL`

### SESSION_SECRET empty in container
- **Cause**: docker-compose uses `${BULWART_SESSION_SECRET}` but .env has `BULWARK_SESSION_SECRET` (typo: no K)
- **Fix**: Pastikan variabel di compose MATCH dengan .env (BULWARK dengan K)
- **Verify**: `docker exec cloudsuite-bulwark env | grep SESSION_SECRET`

### Authentik API 403 on POST (create provider)
- **Cause**: `Authorization: Token xxx` header format tidak work di Authentik 2026.x
- **Fix**: Pakai `Authorization: Bearer xxx` format
- **Note**: GET tetap work dengan Token format, POST/PUT/PATCH butuh Bearer

### Stalwart OIDC Directory schema error
- **Cause**: PRD lama pakai schema endpoint/fields/cache — tidak ada di v0.16.21
- **Fix**: Pakai schema aktual: issuerUrl, claimUsername, claimName, claimGroups, requireAudience
- **Note**: requireScopes field type `set<string>` — validasi format NDJSON

### CORS error: No Access-Control-Allow-Origin
- **Cause**: Stalwart tidak mengirim CORS headers untuk cross-origin requests
- **Fix**: `stalwart-cli update Http --json "{\"usePermissiveCors\":true}"`
- **Verify**: `stalwart-cli get Http | grep cors`

### Authentik OIDC well-known returns HTML
- **Cause**: Provider belum dibuat atau issuer_mode salah
- **Fix**: Pastikan OAuth2 provider sudah ada + issuer_mode = per_provider
- **Verify**: `curl -sk https://auth.idchsuite.my.id/application/o/stalwart-mail/.well-known/openid-configuration`

### Nginx webmail 502 Bad Gateway
- **Cause**: Bulwark container belum running atau port salah
- **Fix**: `docker ps --filter name=bulwark` — pastikan healthy, port 3000

## SSO webmail.idchsuite.my.id

### "SSO is enabled but identity provider could not be reached"
- **Gejala**: Buka https://webmail.idchsuite.my.id/en/login → error SSO
- **Root cause**: OAUTH_ISSUER_URL trailing slash → Bulwark append `/.well-known/...` → double slash → HTTP 404
- **Log error**: `[OAuth] Discovery failed for .../stalwart-mail//.well-known/openid-configuration returned HTTP 404`
- **Fix**: Hapus trailing slash dari OAUTH_ISSUER_URL di .env DAN di docker-compose.yml
  - .env: `BULWARK_OAUTH_ISSUER_URL=https://auth.idchsuite.my.id/application/o/stalwart-mail` (TANPA /)
  - compose: `OAUTH_ISSUER_URL=https://auth.idchsuite.my.id/application/o/stalwart-mail` (TANPA /)
- **Verify**: `docker exec cloudsuite-bulwark env | grep OAUTH_ISSUER` → harus TANPA trailing slash
- **Verify logs**: `docker logs cloudsuite-bulwark --tail 30 | grep -i discovery` → harus tidak ada error
- **Verify discovery**: `docker exec cloudsuite-bulwark wget -qO- .../.well-known/openid-configuration` → return JSON

## SSO Redirect URI Error + Invalid grant_type

### Gejala
1. Klik "Sign in with SSO" di webmail → Authentik error "redirect_uri"
2. Setelah redirect_uri fix → error "Invalid grant_type for provider"

### Root Cause
1. **redirect_uri mismatch**: Bulwark (Next.js i18n) kirim callback `/en/auth/callback` tapi Authentik registered `/api/auth/callback` (strict mode)
2. **grant_types kosong**: Authentik OAuth2 provider default `grant_types: []` — tidak support `authorization_code`

### Fix
1. Update redirect_uris ke regex mode:
```bash
curl -X PATCH -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  "$AUTHENTIK_URL/api/v3/providers/oauth2/<pk>/" \
  -d '{"redirect_uris": [{"matching_mode": "regex", "url": "https://webmail\\\\.idchsuite\\\\.my\\\\.id/[a-z]{2}/auth/callback", "redirect_uri_type": "authorization"}]}'
```
2. Add grant types:
```bash
curl -X PATCH -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  "$AUTHENTIK_URL/api/v3/providers/oauth2/<pk>/" \
  -d '{"grant_types": ["authorization_code", "refresh_token"]}'
```

### Verify
- `curl -sk -o /dev/null -w "%{http_code}" "$AUTHENTIK_URL/application/o/authorize/?response_type=code&client_id=stalwart-mail&redirect_uri=https%3A%2F%2Fwebmail.idchsuite.my.id%2Fen%2Fauth%2Fcallback&scope=openid+email+profile&state=test"` → 302 (bukan 400)

### Pencegahan
- Selalu pakai regex matching_mode untuk OAuth2 redirect_uris yang punya locale prefix
- Selalu set grant_types: authorization_code + refresh_token

## Bulwark SSO — Authentication Failed (Issuer Mismatch)

### Gejala
Setelah SSO callback ke `/en/auth/callback?code=...`, Bulwark tampil "Authentication Failed".

### Root Cause
Stalwart OIDC Directory `issuerUrl` tanpa trailing slash (`/stalwart-mail`), tapi Authentik JWT `iss` claim pakai trailing slash (`/stalwart-mail/`). Stalwart lakukan exact string match → token ditolak.

### Fix
Update issuerUrl di Stalwart Directory:
```bash
stalwart-cli update Directory <id> --json "{\"issuerUrl\":\"https://auth.idchsuite.my.id/application/o/stalwart-mail/\"}"
```
Atau via NDJSON upsert (matchOn: description).

### Pencegahan
Cek issuer actual dari Authentik sebelum setup Stalwart OIDC Directory:
```bash
curl -s https://auth.idchsuite.my.id/application/o/<slug>/.well-known/openid-configuration | jq -r .issuer
```
Copy **exact value** termasuk trailing slash ke Stalwart `issuerUrl`.
