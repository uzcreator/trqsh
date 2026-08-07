-- +goose Up
-- +goose StatementBegin

-- Per-instance tunnel history: one row per bound tunnel, opened at bind and
-- closed at release. Powers per-org history views (/v1/tunnels/history) and the
-- admin fleet dashboard, including where the agent connected from (client_ip ->
-- country/city, resolved by the control API's geo service on report).
CREATE TABLE tunnel_sessions (
    id         TEXT PRIMARY KEY,
    org_id     TEXT NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    edge_id    TEXT NOT NULL DEFAULT '',
    session_id TEXT NOT NULL DEFAULT '',
    tunnel_id  TEXT NOT NULL DEFAULT '',
    type       TEXT NOT NULL DEFAULT '',
    public_url TEXT NOT NULL DEFAULT '',
    host       TEXT NOT NULL DEFAULT '',
    port       INTEGER NOT NULL DEFAULT 0,
    region     TEXT NOT NULL DEFAULT '',
    client_ip  TEXT NOT NULL DEFAULT '',
    country    TEXT NOT NULL DEFAULT '',
    city       TEXT NOT NULL DEFAULT '',
    bytes_in   BIGINT NOT NULL DEFAULT 0,
    bytes_out  BIGINT NOT NULL DEFAULT 0,
    requests   BIGINT NOT NULL DEFAULT 0,
    status     TEXT NOT NULL DEFAULT 'active',
    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    ended_at   TIMESTAMPTZ
);

-- At most one ACTIVE row per live instance (edge/session/tunnel), so an "open"
-- upsert and a keyed "close" are exact; closed rows may repeat the triple if an
-- agent rebinds the same client id, preserving full history.
CREATE UNIQUE INDEX idx_tunnel_sessions_live
    ON tunnel_sessions (edge_id, session_id, tunnel_id) WHERE status = 'active';
CREATE INDEX idx_tunnel_sessions_org ON tunnel_sessions (org_id, started_at DESC);
CREATE INDEX idx_tunnel_sessions_started ON tunnel_sessions (started_at DESC);
CREATE INDEX idx_tunnel_sessions_active ON tunnel_sessions (status) WHERE status = 'active';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS tunnel_sessions;
-- +goose StatementEnd
