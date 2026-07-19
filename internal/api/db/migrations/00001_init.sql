-- +goose Up
-- +goose StatementBegin
CREATE TABLE users (
    id             TEXT PRIMARY KEY,
    email          TEXT NOT NULL UNIQUE,
    name           TEXT NOT NULL DEFAULT '',
    avatar_url     TEXT NOT NULL DEFAULT '',
    oauth_provider TEXT NOT NULL DEFAULT '',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE orgs (
    id                 TEXT PRIMARY KEY,
    name               TEXT NOT NULL,
    plan               TEXT NOT NULL DEFAULT 'free',
    stripe_customer_id TEXT,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE org_members (
    org_id  TEXT NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role    TEXT NOT NULL DEFAULT 'member',
    PRIMARY KEY (org_id, user_id)
);

CREATE TABLE api_keys (
    id           TEXT PRIMARY KEY,
    org_id       TEXT NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    name         TEXT NOT NULL DEFAULT '',
    prefix       TEXT NOT NULL UNIQUE,
    hash         TEXT NOT NULL,
    last_used_at TIMESTAMPTZ,
    revoked_at   TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_api_keys_org ON api_keys(org_id);

CREATE TABLE reserved_subdomains (
    id         TEXT PRIMARY KEY,
    org_id     TEXT NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    subdomain  TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_reserved_subdomains_org ON reserved_subdomains(org_id);

CREATE TABLE custom_domains (
    id           TEXT PRIMARY KEY,
    org_id       TEXT NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    domain       TEXT NOT NULL UNIQUE,
    verify_token TEXT NOT NULL,
    verified_at  TIMESTAMPTZ,
    cert_status  TEXT NOT NULL DEFAULT 'pending',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_custom_domains_org ON custom_domains(org_id);

CREATE TABLE plans (
    code        TEXT PRIMARY KEY,
    limits_json JSONB NOT NULL
);

CREATE TABLE usage_records (
    id           BIGSERIAL PRIMARY KEY,
    org_id       TEXT NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    tunnel_id    TEXT NOT NULL DEFAULT '',
    bytes_in     BIGINT NOT NULL DEFAULT 0,
    bytes_out    BIGINT NOT NULL DEFAULT 0,
    requests     BIGINT NOT NULL DEFAULT 0,
    window_start TIMESTAMPTZ NOT NULL,
    window_end   TIMESTAMPTZ NOT NULL
);
CREATE INDEX idx_usage_org_window ON usage_records(org_id, window_end);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS usage_records;
DROP TABLE IF EXISTS plans;
DROP TABLE IF EXISTS custom_domains;
DROP TABLE IF EXISTS reserved_subdomains;
DROP TABLE IF EXISTS api_keys;
DROP TABLE IF EXISTS org_members;
DROP TABLE IF EXISTS orgs;
DROP TABLE IF EXISTS users;
-- +goose StatementEnd
