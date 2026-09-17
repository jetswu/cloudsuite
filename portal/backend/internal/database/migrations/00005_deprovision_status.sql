-- Sprint 1.4b: de-provisioning lifecycle statuses for user_provisioning.
-- provisioning_jobs.action already allows 'deprovision' since migration 00004
-- (chk_prov_action), so only the per-service status enums need widening.

-- +goose Up
-- +goose StatementBegin
ALTER TABLE user_provisioning DROP CONSTRAINT chk_up_stalwart;
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE user_provisioning ADD CONSTRAINT chk_up_stalwart
  CHECK (stalwart_status IN ('pending', 'provisioning', 'active', 'failed', 'deleted', 'pending_delete'));
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE user_provisioning DROP CONSTRAINT chk_up_nextcloud;
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE user_provisioning ADD CONSTRAINT chk_up_nextcloud
  CHECK (nextcloud_status IN ('pending', 'provisioning', 'active', 'failed', 'deleted', 'pending_delete'));
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE user_provisioning DROP CONSTRAINT chk_up_odoo;
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE user_provisioning ADD CONSTRAINT chk_up_odoo
  CHECK (odoo_status IN ('pending', 'provisioning', 'active', 'failed', 'deleted', 'pending_delete'));
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE user_provisioning DROP CONSTRAINT chk_up_stalwart;
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE user_provisioning ADD CONSTRAINT chk_up_stalwart
  CHECK (stalwart_status IN ('pending', 'provisioning', 'active', 'failed', 'deleted'));
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE user_provisioning DROP CONSTRAINT chk_up_nextcloud;
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE user_provisioning ADD CONSTRAINT chk_up_nextcloud
  CHECK (nextcloud_status IN ('pending', 'provisioning', 'active', 'failed', 'deleted'));
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE user_provisioning DROP CONSTRAINT chk_up_odoo;
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE user_provisioning ADD CONSTRAINT chk_up_odoo
  CHECK (odoo_status IN ('pending', 'provisioning', 'active', 'failed', 'deleted'));
-- +goose StatementEnd
