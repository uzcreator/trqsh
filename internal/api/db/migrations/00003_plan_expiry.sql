-- +goose Up
-- +goose StatementBegin

-- When set, the org's manually-granted plan reverts to Free after this instant.
-- NULL means the plan never expires (Stripe/webhook path and the default). The
-- admin grant flow (approve.trqsh.uz) sets this; entitlements enforce it at bind.
ALTER TABLE orgs ADD COLUMN plan_expires_at TIMESTAMPTZ;
CREATE INDEX idx_orgs_plan_expires_at ON orgs(plan_expires_at) WHERE plan_expires_at IS NOT NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_orgs_plan_expires_at;
ALTER TABLE orgs DROP COLUMN IF EXISTS plan_expires_at;
-- +goose StatementEnd
