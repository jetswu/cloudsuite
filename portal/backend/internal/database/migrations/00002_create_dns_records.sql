-- +goose Up
-- +goose StatementBegin
CREATE TABLE dns_records (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    domain_id UUID NOT NULL REFERENCES domains(id) ON DELETE CASCADE,
    record_type VARCHAR(20) NOT NULL,
    name VARCHAR(255) NOT NULL,
    value TEXT NOT NULL,
    priority INTEGER,
    purpose VARCHAR(50) NOT NULL,
    is_required BOOLEAN NOT NULL DEFAULT true,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    last_checked_at TIMESTAMPTZ,
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    UNIQUE (domain_id, record_type, name, purpose)
);

CREATE INDEX idx_dns_records_domain ON dns_records(domain_id);
CREATE INDEX idx_dns_records_status ON dns_records(domain_id, status);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE dns_records;
-- +goose StatementEnd
