-- +goose Up
-- +goose StatementBegin

-- One row per org mirroring its Stripe subscription. Webhooks own this table and
-- the derived orgs.plan; there is at most one subscription per org.
CREATE TABLE subscriptions (
    id                     TEXT PRIMARY KEY,
    org_id                 TEXT NOT NULL UNIQUE REFERENCES orgs(id) ON DELETE CASCADE,
    stripe_subscription_id TEXT NOT NULL UNIQUE,
    stripe_customer_id     TEXT NOT NULL DEFAULT '',
    plan                   TEXT NOT NULL,
    status                 TEXT NOT NULL,
    current_period_end     TIMESTAMPTZ,
    cancel_at_period_end   BOOLEAN NOT NULL DEFAULT false,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_subscriptions_customer ON subscriptions(stripe_customer_id);

-- Dedupe table so redelivered Stripe webhooks are processed at most once.
CREATE TABLE billing_events (
    event_id     TEXT PRIMARY KEY,
    type         TEXT NOT NULL DEFAULT '',
    processed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Pending/settled metered-usage pushes to Stripe for Pay-as-you-go / overage.
-- Unique per (org, metric, window_start) so collection is idempotent.
CREATE TABLE metered_usage (
    id           BIGSERIAL PRIMARY KEY,
    org_id       TEXT NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    metric       TEXT NOT NULL,
    quantity     BIGINT NOT NULL DEFAULT 0,
    window_start TIMESTAMPTZ NOT NULL,
    window_end   TIMESTAMPTZ NOT NULL,
    reported_at  TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (org_id, metric, window_start)
);
CREATE INDEX idx_metered_usage_pending ON metered_usage(reported_at) WHERE reported_at IS NULL;

-- Index the existing orgs.stripe_customer_id so webhooks can map customer -> org.
CREATE INDEX idx_orgs_stripe_customer ON orgs(stripe_customer_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_orgs_stripe_customer;
DROP TABLE IF EXISTS metered_usage;
DROP TABLE IF EXISTS billing_events;
DROP TABLE IF EXISTS subscriptions;
-- +goose StatementEnd
