# Odoo Notes — CloudSuite (staging)

- **URL:** https://erp.idchsuite.my.id
- **Image:** cloudsuite-odoo:18.0 (FROM odoo:18.0 + python-jose + odoo-addon-auth-oidc 18.0.1.1.0.2)
- **Container:** cloudsuite-odoo
- **DB:** odoo (OWNER cloudsuite) di cloudsuite-postgres
- **SSO:** OCA auth_oidc + OIDC via Authentik (provider pk 3, app slug odoo)

## Konfigurasi penting

- `dbfilter = ^odoo$`, `list_db = False` (cegah DB selector / brute list DB)
- odoo.conf owner 100:101, mode 600 (di host tampil `dhcpcd:uuidd`, di container = odoo)
- Dockerfile custom pakai OCA auth_oidc (bukan native), karena native Odoo tidak dukung
  OIDC authorization-code flow murni
- Install pip pakai `--break-system-packages` (PEP 668 — image Debian tidak izinkan pip global)

## Template user internal

- `base.template_portal_user_id = 8` (cloudsuite_template)
- BUKAN `auth_signup.default_template_user_id`
- JANGAN pakai admin sebagai template (privilege escalation)
- Template: grup HANYA Internal User + Technical Features (share=False)

## Break-glass admin

- Login: admin@idchsuite.my.id (set saat create DB)
- JANGAN dipakai operasional harian — login normal via SSO
- Reset password: via Odoo UI atau shell

## Home action

- Default: Discuss (`mail.action_discuss`, id=109)
- Set via `ir.default` field `res.users.action_id`, json_value=109
- User baru otomatis dapat

## Prosedur test SSO user baru

1. Hapus user Odoo existing (unlink — aman, tidak hapus akun Authentik):

       env['res.users'].search([('login','ilike','provider_6_user')]).unlink()

2. Logout dari Odoo
3. Login via "Login with CloudSuite"
4. User baru auto-create dengan Internal group, action_id=109

## OAuth provider

- Provider Authentik: pk=3, slug=odoo
- Redirect URI (strict): https://erp.idchsuite.my.id/auth_oauth/signin
- grant_types: authorization_code + refresh_token
- Odoo OAuth provider: id=6, flow=id_token_code

## Catatan

- Jangan pakai admin sebagai template user
- SSO user = Internal (bukan Portal)
- Portal user butuh group mapping khusus (Phase 2)
