package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib" // pgx database/sql driver
)

// PostgresStore is the production Store backed by Postgres. Its queries match
// the goose migrations in internal/api/db/migrations. It is validated by
// compilation + schema review here; run the migrations and point TRQSH_DATABASE_URL
// at a live database to exercise it.
type PostgresStore struct {
	db *sql.DB
}

// PoolConfig tunes the database/sql connection pool. Zero fields fall back to
// sensible defaults in NewPostgresStore.
type PoolConfig struct {
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

// NewPostgresStore opens a Postgres connection pool from a postgres:// DSN.
func NewPostgresStore(dsn string, pc PoolConfig) (*PostgresStore, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("store: open postgres: %w", err)
	}
	// MaxIdleConns tracks MaxOpenConns: this is a small number of long-lived server
	// processes, so keeping idle connections warm avoids churn (the database/sql
	// default of 2 would repeatedly close/reopen against a much larger open pool).
	if pc.MaxOpenConns <= 0 {
		pc.MaxOpenConns = 20
	}
	if pc.MaxIdleConns <= 0 {
		pc.MaxIdleConns = pc.MaxOpenConns
	}
	if pc.ConnMaxLifetime <= 0 {
		pc.ConnMaxLifetime = 45 * time.Minute // let LB/DNS changes eventually rotate connections
	}
	if pc.ConnMaxIdleTime <= 0 {
		pc.ConnMaxIdleTime = 5 * time.Minute
	}
	db.SetMaxOpenConns(pc.MaxOpenConns)
	db.SetMaxIdleConns(pc.MaxIdleConns)
	db.SetConnMaxLifetime(pc.ConnMaxLifetime)
	db.SetConnMaxIdleTime(pc.ConnMaxIdleTime)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("store: ping postgres: %w", err)
	}
	return &PostgresStore{db: db}, nil
}

func mapErr(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func (p *PostgresStore) CreateUser(ctx context.Context, u User) (User, error) {
	if u.ID == "" {
		u.ID = NewID("user")
	}
	if u.CreatedAt.IsZero() {
		u.CreatedAt = time.Now()
	}
	_, err := p.db.ExecContext(ctx,
		`INSERT INTO users (id, email, name, avatar_url, oauth_provider, created_at)
		 VALUES ($1,$2,$3,$4,$5,$6)`,
		u.ID, u.Email, u.Name, u.AvatarURL, u.OAuthProvider, u.CreatedAt)
	return u, err
}

func (p *PostgresStore) GetUser(ctx context.Context, id string) (User, error) {
	return p.scanUser(p.db.QueryRowContext(ctx,
		`SELECT id,email,name,avatar_url,oauth_provider,created_at FROM users WHERE id=$1`, id))
}

func (p *PostgresStore) GetUserByEmail(ctx context.Context, email string) (User, error) {
	return p.scanUser(p.db.QueryRowContext(ctx,
		`SELECT id,email,name,avatar_url,oauth_provider,created_at FROM users WHERE lower(email)=lower($1)`, email))
}

func (p *PostgresStore) scanUser(row *sql.Row) (User, error) {
	var u User
	err := row.Scan(&u.ID, &u.Email, &u.Name, &u.AvatarURL, &u.OAuthProvider, &u.CreatedAt)
	return u, mapErr(err)
}

func (p *PostgresStore) CreateOrg(ctx context.Context, o Org) (Org, error) {
	if o.ID == "" {
		o.ID = NewID("org")
	}
	if o.Plan == "" {
		o.Plan = "free"
	}
	if o.CreatedAt.IsZero() {
		o.CreatedAt = time.Now()
	}
	_, err := p.db.ExecContext(ctx,
		`INSERT INTO orgs (id, name, plan, stripe_customer_id, created_at) VALUES ($1,$2,$3,$4,$5)`,
		o.ID, o.Name, o.Plan, nullStr(o.StripeCustomerID), o.CreatedAt)
	return o, err
}

func (p *PostgresStore) GetOrg(ctx context.Context, id string) (Org, error) {
	var o Org
	var cust sql.NullString
	var exp sql.NullTime
	err := p.db.QueryRowContext(ctx,
		`SELECT id,name,plan,COALESCE(stripe_customer_id,''),created_at,plan_expires_at FROM orgs WHERE id=$1`, id).
		Scan(&o.ID, &o.Name, &o.Plan, &cust, &o.CreatedAt, &exp)
	o.StripeCustomerID = cust.String
	o.PlanExpiresAt = nullTimePtr(exp)
	return o, mapErr(err)
}

func (p *PostgresStore) OrgsByPlan(ctx context.Context, plan string) ([]Org, error) {
	rows, err := p.db.QueryContext(ctx,
		`SELECT id,name,plan,COALESCE(stripe_customer_id,''),created_at,plan_expires_at FROM orgs WHERE plan=$1`, plan)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Org
	for rows.Next() {
		var o Org
		var exp sql.NullTime
		if err := rows.Scan(&o.ID, &o.Name, &o.Plan, &o.StripeCustomerID, &o.CreatedAt, &exp); err != nil {
			return nil, err
		}
		o.PlanExpiresAt = nullTimePtr(exp)
		out = append(out, o)
	}
	return out, rows.Err()
}

func (p *PostgresStore) UpdateOrgPlan(ctx context.Context, orgID, plan string) error {
	// The Stripe/webhook path: a subscription-backed plan never time-expires, so
	// clear any manual expiry that was set previously.
	return execOne(p.db.ExecContext(ctx, `UPDATE orgs SET plan=$2, plan_expires_at=NULL WHERE id=$1`, orgID, plan))
}

func (p *PostgresStore) SetOrgPlan(ctx context.Context, orgID, plan string, expiresAt *time.Time) error {
	var exp any
	if expiresAt != nil {
		exp = nullTime(*expiresAt)
	}
	return execOne(p.db.ExecContext(ctx,
		`UPDATE orgs SET plan=$2, plan_expires_at=$3 WHERE id=$1`, orgID, plan, exp))
}

func (p *PostgresStore) UpdateUserProfile(ctx context.Context, id, name, avatarURL string) error {
	return execOne(p.db.ExecContext(ctx,
		`UPDATE users SET name = COALESCE(NULLIF($2,''), name), avatar_url = COALESCE(NULLIF($3,''), avatar_url) WHERE id = $1`,
		id, name, avatarURL))
}

func (p *PostgresStore) SetOrgStripeCustomer(ctx context.Context, orgID, customerID string) error {
	return execOne(p.db.ExecContext(ctx, `UPDATE orgs SET stripe_customer_id=$2 WHERE id=$1`, orgID, customerID))
}

func (p *PostgresStore) AddOrgMember(ctx context.Context, m OrgMember) error {
	_, err := p.db.ExecContext(ctx,
		`INSERT INTO org_members (org_id,user_id,role) VALUES ($1,$2,$3)`, m.OrgID, m.UserID, m.Role)
	return err
}

func (p *PostgresStore) ListOrgMembers(ctx context.Context, orgID string) ([]OrgMember, error) {
	rows, err := p.db.QueryContext(ctx, `SELECT org_id,user_id,role FROM org_members WHERE org_id=$1`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []OrgMember
	for rows.Next() {
		var m OrgMember
		if err := rows.Scan(&m.OrgID, &m.UserID, &m.Role); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (p *PostgresStore) OrgsForUser(ctx context.Context, userID string) ([]Org, error) {
	rows, err := p.db.QueryContext(ctx,
		`SELECT o.id,o.name,o.plan,COALESCE(o.stripe_customer_id,''),o.created_at,o.plan_expires_at
		 FROM orgs o JOIN org_members m ON m.org_id=o.id WHERE m.user_id=$1`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Org
	for rows.Next() {
		var o Org
		var exp sql.NullTime
		if err := rows.Scan(&o.ID, &o.Name, &o.Plan, &o.StripeCustomerID, &o.CreatedAt, &exp); err != nil {
			return nil, err
		}
		o.PlanExpiresAt = nullTimePtr(exp)
		out = append(out, o)
	}
	return out, rows.Err()
}

func (p *PostgresStore) CreateAPIKey(ctx context.Context, k APIKey) (APIKey, error) {
	if k.ID == "" {
		k.ID = NewID("key")
	}
	if k.CreatedAt.IsZero() {
		k.CreatedAt = time.Now()
	}
	_, err := p.db.ExecContext(ctx,
		`INSERT INTO api_keys (id,org_id,name,prefix,hash,created_at) VALUES ($1,$2,$3,$4,$5,$6)`,
		k.ID, k.OrgID, k.Name, k.Prefix, k.Hash, k.CreatedAt)
	return k, err
}

func (p *PostgresStore) GetAPIKeyByPrefix(ctx context.Context, prefix string) (APIKey, error) {
	var k APIKey
	var last, revoked sql.NullTime
	err := p.db.QueryRowContext(ctx,
		`SELECT id,org_id,name,prefix,hash,last_used_at,revoked_at,created_at FROM api_keys WHERE prefix=$1`, prefix).
		Scan(&k.ID, &k.OrgID, &k.Name, &k.Prefix, &k.Hash, &last, &revoked, &k.CreatedAt)
	k.LastUsedAt = nullTimePtr(last)
	k.RevokedAt = nullTimePtr(revoked)
	return k, mapErr(err)
}

func (p *PostgresStore) ListAPIKeys(ctx context.Context, orgID string) ([]APIKey, error) {
	rows, err := p.db.QueryContext(ctx,
		`SELECT id,org_id,name,prefix,hash,last_used_at,revoked_at,created_at FROM api_keys WHERE org_id=$1`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []APIKey
	for rows.Next() {
		var k APIKey
		var last, revoked sql.NullTime
		if err := rows.Scan(&k.ID, &k.OrgID, &k.Name, &k.Prefix, &k.Hash, &last, &revoked, &k.CreatedAt); err != nil {
			return nil, err
		}
		k.LastUsedAt = nullTimePtr(last)
		k.RevokedAt = nullTimePtr(revoked)
		out = append(out, k)
	}
	return out, rows.Err()
}

func (p *PostgresStore) TouchAPIKey(ctx context.Context, id string, at time.Time) error {
	return execOne(p.db.ExecContext(ctx, `UPDATE api_keys SET last_used_at=$2 WHERE id=$1`, id, at))
}

func (p *PostgresStore) RevokeAPIKey(ctx context.Context, id, orgID string, at time.Time) error {
	return execOne(p.db.ExecContext(ctx, `UPDATE api_keys SET revoked_at=$3 WHERE id=$1 AND org_id=$2`, id, orgID, at))
}

func (p *PostgresStore) ReserveSubdomain(ctx context.Context, s ReservedSubdomain) (ReservedSubdomain, error) {
	if s.ID == "" {
		s.ID = NewID("sub")
	}
	if s.CreatedAt.IsZero() {
		s.CreatedAt = time.Now()
	}
	_, err := p.db.ExecContext(ctx,
		`INSERT INTO reserved_subdomains (id,org_id,subdomain,created_at) VALUES ($1,$2,lower($3),$4)`,
		s.ID, s.OrgID, s.Subdomain, s.CreatedAt)
	if isUniqueViolation(err) {
		return ReservedSubdomain{}, ErrConflict
	}
	return s, err
}

func (p *PostgresStore) ListSubdomains(ctx context.Context, orgID string) ([]ReservedSubdomain, error) {
	rows, err := p.db.QueryContext(ctx,
		`SELECT id,org_id,subdomain,created_at FROM reserved_subdomains WHERE org_id=$1`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ReservedSubdomain
	for rows.Next() {
		var s ReservedSubdomain
		if err := rows.Scan(&s.ID, &s.OrgID, &s.Subdomain, &s.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (p *PostgresStore) ReleaseSubdomain(ctx context.Context, id, orgID string) error {
	return execOne(p.db.ExecContext(ctx, `DELETE FROM reserved_subdomains WHERE id=$1 AND org_id=$2`, id, orgID))
}

func (p *PostgresStore) IsSubdomainReserved(ctx context.Context, orgID, subdomain string) (bool, error) {
	var owner string
	err := p.db.QueryRowContext(ctx,
		`SELECT org_id FROM reserved_subdomains WHERE subdomain=lower($1)`, subdomain).Scan(&owner)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return owner == orgID, nil
}

func (p *PostgresStore) CountSubdomains(ctx context.Context, orgID string) (int, error) {
	var n int
	err := p.db.QueryRowContext(ctx, `SELECT count(*) FROM reserved_subdomains WHERE org_id=$1`, orgID).Scan(&n)
	return n, err
}

func (p *PostgresStore) AddCustomDomain(ctx context.Context, d CustomDomain) (CustomDomain, error) {
	if d.ID == "" {
		d.ID = NewID("dom")
	}
	if d.CreatedAt.IsZero() {
		d.CreatedAt = time.Now()
	}
	if d.CertStatus == "" {
		d.CertStatus = "pending"
	}
	_, err := p.db.ExecContext(ctx,
		`INSERT INTO custom_domains (id,org_id,domain,verify_token,cert_status,created_at)
		 VALUES ($1,$2,lower($3),$4,$5,$6)`,
		d.ID, d.OrgID, d.Domain, d.VerifyToken, d.CertStatus, d.CreatedAt)
	if isUniqueViolation(err) {
		return CustomDomain{}, ErrConflict
	}
	return d, err
}

func (p *PostgresStore) GetCustomDomain(ctx context.Context, id, orgID string) (CustomDomain, error) {
	return p.scanDomain(p.db.QueryRowContext(ctx,
		`SELECT id,org_id,domain,verify_token,verified_at,cert_status,created_at
		 FROM custom_domains WHERE id=$1 AND org_id=$2`, id, orgID))
}

func (p *PostgresStore) GetCustomDomainByName(ctx context.Context, domain string) (CustomDomain, error) {
	return p.scanDomain(p.db.QueryRowContext(ctx,
		`SELECT id,org_id,domain,verify_token,verified_at,cert_status,created_at
		 FROM custom_domains WHERE domain=lower($1)`, domain))
}

func (p *PostgresStore) scanDomain(row *sql.Row) (CustomDomain, error) {
	var d CustomDomain
	var verified sql.NullTime
	err := row.Scan(&d.ID, &d.OrgID, &d.Domain, &d.VerifyToken, &verified, &d.CertStatus, &d.CreatedAt)
	d.VerifiedAt = nullTimePtr(verified)
	return d, mapErr(err)
}

func (p *PostgresStore) ListCustomDomains(ctx context.Context, orgID string) ([]CustomDomain, error) {
	rows, err := p.db.QueryContext(ctx,
		`SELECT id,org_id,domain,verify_token,verified_at,cert_status,created_at
		 FROM custom_domains WHERE org_id=$1`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CustomDomain
	for rows.Next() {
		var d CustomDomain
		var verified sql.NullTime
		if err := rows.Scan(&d.ID, &d.OrgID, &d.Domain, &d.VerifyToken, &verified, &d.CertStatus, &d.CreatedAt); err != nil {
			return nil, err
		}
		d.VerifiedAt = nullTimePtr(verified)
		out = append(out, d)
	}
	return out, rows.Err()
}

func (p *PostgresStore) VerifyCustomDomain(ctx context.Context, id, orgID string, at time.Time) error {
	return execOne(p.db.ExecContext(ctx,
		`UPDATE custom_domains SET verified_at=$3, cert_status='verified' WHERE id=$1 AND org_id=$2`, id, orgID, at))
}

func (p *PostgresStore) CountCustomDomains(ctx context.Context, orgID string) (int, error) {
	var n int
	err := p.db.QueryRowContext(ctx, `SELECT count(*) FROM custom_domains WHERE org_id=$1`, orgID).Scan(&n)
	return n, err
}

func (p *PostgresStore) UpsertUsage(ctx context.Context, u UsageRecord) error {
	_, err := p.db.ExecContext(ctx,
		`INSERT INTO usage_records (org_id,tunnel_id,bytes_in,bytes_out,requests,window_start,window_end)
		 VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		u.OrgID, u.TunnelID, u.BytesIn, u.BytesOut, u.Requests, u.WindowStart, u.WindowEnd)
	return err
}

func (p *PostgresStore) UsageForOrg(ctx context.Context, orgID string, since time.Time) (UsageRecord, error) {
	agg := UsageRecord{OrgID: orgID, WindowStart: since, WindowEnd: time.Now()}
	err := p.db.QueryRowContext(ctx,
		`SELECT COALESCE(sum(bytes_in),0),COALESCE(sum(bytes_out),0),COALESCE(sum(requests),0)
		 FROM usage_records WHERE org_id=$1 AND window_end>=$2`, orgID, since).
		Scan(&agg.BytesIn, &agg.BytesOut, &agg.Requests)
	return agg, err
}

// --- Billing ---

func (p *PostgresStore) UpsertSubscription(ctx context.Context, s Subscription) (Subscription, error) {
	if s.ID == "" {
		s.ID = NewID("sub")
	}
	if s.CreatedAt.IsZero() {
		s.CreatedAt = time.Now()
	}
	s.UpdatedAt = time.Now()
	// Upsert keyed by org_id (one active subscription per org).
	_, err := p.db.ExecContext(ctx,
		`INSERT INTO subscriptions
		   (id, org_id, stripe_subscription_id, stripe_customer_id, plan, status,
		    current_period_end, cancel_at_period_end, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		 ON CONFLICT (org_id) DO UPDATE SET
		    stripe_subscription_id = EXCLUDED.stripe_subscription_id,
		    stripe_customer_id     = EXCLUDED.stripe_customer_id,
		    plan                   = EXCLUDED.plan,
		    status                 = EXCLUDED.status,
		    current_period_end     = EXCLUDED.current_period_end,
		    cancel_at_period_end   = EXCLUDED.cancel_at_period_end,
		    updated_at             = EXCLUDED.updated_at`,
		s.ID, s.OrgID, s.StripeSubID, s.StripeCustomerID, s.Plan, s.Status,
		nullTime(s.CurrentPeriodEnd), s.CancelAtPeriodEnd, s.CreatedAt, s.UpdatedAt)
	return s, err
}

func (p *PostgresStore) GetSubscriptionForOrg(ctx context.Context, orgID string) (Subscription, error) {
	var s Subscription
	var end sql.NullTime
	err := p.db.QueryRowContext(ctx,
		`SELECT id,org_id,stripe_subscription_id,stripe_customer_id,plan,status,
		        current_period_end,cancel_at_period_end,created_at,updated_at
		 FROM subscriptions WHERE org_id=$1`, orgID).
		Scan(&s.ID, &s.OrgID, &s.StripeSubID, &s.StripeCustomerID, &s.Plan, &s.Status,
			&end, &s.CancelAtPeriodEnd, &s.CreatedAt, &s.UpdatedAt)
	if end.Valid {
		s.CurrentPeriodEnd = end.Time
	}
	return s, mapErr(err)
}

func (p *PostgresStore) DeleteSubscription(ctx context.Context, stripeSubID string) error {
	return execOne(p.db.ExecContext(ctx,
		`DELETE FROM subscriptions WHERE stripe_subscription_id=$1`, stripeSubID))
}

func (p *PostgresStore) GetOrgByStripeCustomer(ctx context.Context, customerID string) (Org, error) {
	if customerID == "" {
		return Org{}, ErrNotFound
	}
	var o Org
	var cust sql.NullString
	var exp sql.NullTime
	err := p.db.QueryRowContext(ctx,
		`SELECT id,name,plan,COALESCE(stripe_customer_id,''),created_at,plan_expires_at
		 FROM orgs WHERE stripe_customer_id=$1`, customerID).
		Scan(&o.ID, &o.Name, &o.Plan, &cust, &o.CreatedAt, &exp)
	o.StripeCustomerID = cust.String
	o.PlanExpiresAt = nullTimePtr(exp)
	return o, mapErr(err)
}

func (p *PostgresStore) MarkEventProcessed(ctx context.Context, eventID, eventType string) (bool, error) {
	if eventID == "" {
		return true, nil
	}
	res, err := p.db.ExecContext(ctx,
		`INSERT INTO billing_events (event_id, type, processed_at) VALUES ($1,$2,now())
		 ON CONFLICT (event_id) DO NOTHING`, eventID, eventType)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n == 1, nil
}

func (p *PostgresStore) InsertMeteredUsage(ctx context.Context, u MeteredUsage) (MeteredUsage, error) {
	if u.CreatedAt.IsZero() {
		u.CreatedAt = time.Now()
	}
	err := p.db.QueryRowContext(ctx,
		`INSERT INTO metered_usage (org_id, metric, quantity, window_start, window_end, created_at)
		 VALUES ($1,$2,$3,$4,$5,$6) RETURNING id`,
		u.OrgID, u.Metric, u.Quantity, u.WindowStart, u.WindowEnd, u.CreatedAt).Scan(&u.ID)
	if isUniqueViolation(err) {
		return MeteredUsage{}, ErrConflict
	}
	return u, err
}

func (p *PostgresStore) PendingMeteredUsage(ctx context.Context, limit int) ([]MeteredUsage, error) {
	if limit <= 0 {
		limit = 1000
	}
	rows, err := p.db.QueryContext(ctx,
		`SELECT id,org_id,metric,quantity,window_start,window_end,reported_at,created_at
		 FROM metered_usage WHERE reported_at IS NULL ORDER BY id LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MeteredUsage
	for rows.Next() {
		var u MeteredUsage
		var reported sql.NullTime
		if err := rows.Scan(&u.ID, &u.OrgID, &u.Metric, &u.Quantity, &u.WindowStart, &u.WindowEnd, &reported, &u.CreatedAt); err != nil {
			return nil, err
		}
		u.ReportedAt = nullTimePtr(reported)
		out = append(out, u)
	}
	return out, rows.Err()
}

func (p *PostgresStore) MarkMeteredUsageReported(ctx context.Context, ids []int64, at time.Time) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := p.db.ExecContext(ctx,
		`UPDATE metered_usage SET reported_at=$2 WHERE id = ANY($1)`, ids, at)
	return err
}

func (p *PostgresStore) Close() error { return p.db.Close() }

// --- helpers ---

func execOne(res sql.Result, err error) error {
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nullTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t
}

func nullTimePtr(t sql.NullTime) *time.Time {
	if !t.Valid {
		return nil
	}
	return &t.Time
}

// isUniqueViolation reports whether err is a Postgres unique-constraint error
// (SQLSTATE 23505). It matches by message to avoid importing the pgconn types.
func isUniqueViolation(err error) bool {
	return err != nil && (strings.Contains(err.Error(), "23505") || strings.Contains(err.Error(), "duplicate key"))
}
