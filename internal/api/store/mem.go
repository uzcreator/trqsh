package store

import (
	"context"
	"strings"
	"sync"
	"time"
)

// MemStore is an in-memory Store for dev and tests.
type MemStore struct {
	mu sync.RWMutex

	users      map[string]User
	usersEmail map[string]string // email -> id
	orgs       map[string]Org
	members    []OrgMember
	keys       map[string]APIKey // id -> key
	keyPrefix  map[string]string // prefix -> id
	subs       map[string]ReservedSubdomain
	subByName  map[string]string // subdomain -> id
	domains    map[string]CustomDomain
	domByName  map[string]string // domain -> id
	usage      []UsageRecord

	subsByOrg    map[string]Subscription // orgID -> subscription
	subsByStripe map[string]string       // stripe sub id -> orgID
	events       map[string]bool         // processed webhook event ids
	metered      []MeteredUsage
	meteredSeq   int64
}

// NewMemStore builds an empty in-memory store.
func NewMemStore() *MemStore {
	return &MemStore{
		users:      map[string]User{},
		usersEmail: map[string]string{},
		orgs:       map[string]Org{},
		keys:       map[string]APIKey{},
		keyPrefix:  map[string]string{},
		subs:       map[string]ReservedSubdomain{},
		subByName:  map[string]string{},
		domains:    map[string]CustomDomain{},
		domByName:  map[string]string{},

		subsByOrg:    map[string]Subscription{},
		subsByStripe: map[string]string{},
		events:       map[string]bool{},
	}
}

func (m *MemStore) CreateUser(_ context.Context, u User) (User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.usersEmail[strings.ToLower(u.Email)]; ok {
		return User{}, ErrConflict
	}
	if u.ID == "" {
		u.ID = NewID("user")
	}
	if u.CreatedAt.IsZero() {
		u.CreatedAt = time.Now()
	}
	m.users[u.ID] = u
	m.usersEmail[strings.ToLower(u.Email)] = u.ID
	return u, nil
}

func (m *MemStore) UpdateUserProfile(_ context.Context, id, name, avatarURL string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.users[id]
	if !ok {
		return ErrNotFound
	}
	if name != "" {
		u.Name = name
	}
	if avatarURL != "" {
		u.AvatarURL = avatarURL
	}
	m.users[id] = u
	return nil
}

func (m *MemStore) GetUser(_ context.Context, id string) (User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	u, ok := m.users[id]
	if !ok {
		return User{}, ErrNotFound
	}
	return u, nil
}

func (m *MemStore) GetUserByEmail(_ context.Context, email string) (User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	id, ok := m.usersEmail[strings.ToLower(email)]
	if !ok {
		return User{}, ErrNotFound
	}
	return m.users[id], nil
}

func (m *MemStore) CreateOrg(_ context.Context, o Org) (Org, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if o.ID == "" {
		o.ID = NewID("org")
	}
	if o.Plan == "" {
		o.Plan = "free"
	}
	if o.CreatedAt.IsZero() {
		o.CreatedAt = time.Now()
	}
	m.orgs[o.ID] = o
	return o, nil
}

func (m *MemStore) GetOrg(_ context.Context, id string) (Org, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	o, ok := m.orgs[id]
	if !ok {
		return Org{}, ErrNotFound
	}
	return o, nil
}

func (m *MemStore) OrgsByPlan(_ context.Context, plan string) ([]Org, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []Org
	for _, o := range m.orgs {
		if o.Plan == plan {
			out = append(out, o)
		}
	}
	return out, nil
}

func (m *MemStore) UpdateOrgPlan(_ context.Context, orgID, plan string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	o, ok := m.orgs[orgID]
	if !ok {
		return ErrNotFound
	}
	o.Plan = plan
	o.PlanExpiresAt = nil // Stripe-backed plans don't time-expire.
	m.orgs[orgID] = o
	return nil
}

func (m *MemStore) SetOrgPlan(_ context.Context, orgID, plan string, expiresAt *time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	o, ok := m.orgs[orgID]
	if !ok {
		return ErrNotFound
	}
	o.Plan = plan
	o.PlanExpiresAt = expiresAt
	m.orgs[orgID] = o
	return nil
}

func (m *MemStore) SetOrgStripeCustomer(_ context.Context, orgID, customerID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	o, ok := m.orgs[orgID]
	if !ok {
		return ErrNotFound
	}
	o.StripeCustomerID = customerID
	m.orgs[orgID] = o
	return nil
}

func (m *MemStore) AddOrgMember(_ context.Context, mem OrgMember) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, e := range m.members {
		if e.OrgID == mem.OrgID && e.UserID == mem.UserID {
			return ErrConflict
		}
	}
	m.members = append(m.members, mem)
	return nil
}

func (m *MemStore) ListOrgMembers(_ context.Context, orgID string) ([]OrgMember, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []OrgMember
	for _, e := range m.members {
		if e.OrgID == orgID {
			out = append(out, e)
		}
	}
	return out, nil
}

func (m *MemStore) OrgsForUser(_ context.Context, userID string) ([]Org, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []Org
	for _, e := range m.members {
		if e.UserID == userID {
			if o, ok := m.orgs[e.OrgID]; ok {
				out = append(out, o)
			}
		}
	}
	return out, nil
}

func (m *MemStore) CreateAPIKey(_ context.Context, k APIKey) (APIKey, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if k.ID == "" {
		k.ID = NewID("key")
	}
	if k.CreatedAt.IsZero() {
		k.CreatedAt = time.Now()
	}
	if _, ok := m.keyPrefix[k.Prefix]; ok {
		return APIKey{}, ErrConflict
	}
	m.keys[k.ID] = k
	m.keyPrefix[k.Prefix] = k.ID
	return k, nil
}

func (m *MemStore) GetAPIKeyByPrefix(_ context.Context, prefix string) (APIKey, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	id, ok := m.keyPrefix[prefix]
	if !ok {
		return APIKey{}, ErrNotFound
	}
	return m.keys[id], nil
}

func (m *MemStore) ListAPIKeys(_ context.Context, orgID string) ([]APIKey, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []APIKey
	for _, k := range m.keys {
		if k.OrgID == orgID {
			out = append(out, k)
		}
	}
	return out, nil
}

func (m *MemStore) TouchAPIKey(_ context.Context, id string, at time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	k, ok := m.keys[id]
	if !ok {
		return ErrNotFound
	}
	k.LastUsedAt = &at
	m.keys[id] = k
	return nil
}

func (m *MemStore) RevokeAPIKey(_ context.Context, id, orgID string, at time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	k, ok := m.keys[id]
	if !ok || k.OrgID != orgID {
		return ErrNotFound
	}
	k.RevokedAt = &at
	m.keys[id] = k
	return nil
}

func (m *MemStore) ReserveSubdomain(_ context.Context, s ReservedSubdomain) (ReservedSubdomain, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	name := strings.ToLower(s.Subdomain)
	if _, ok := m.subByName[name]; ok {
		return ReservedSubdomain{}, ErrConflict
	}
	if s.ID == "" {
		s.ID = NewID("sub")
	}
	if s.CreatedAt.IsZero() {
		s.CreatedAt = time.Now()
	}
	s.Subdomain = name
	m.subs[s.ID] = s
	m.subByName[name] = s.ID
	return s, nil
}

func (m *MemStore) ListSubdomains(_ context.Context, orgID string) ([]ReservedSubdomain, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []ReservedSubdomain
	for _, s := range m.subs {
		if s.OrgID == orgID {
			out = append(out, s)
		}
	}
	return out, nil
}

func (m *MemStore) ReleaseSubdomain(_ context.Context, id, orgID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.subs[id]
	if !ok || s.OrgID != orgID {
		return ErrNotFound
	}
	delete(m.subs, id)
	delete(m.subByName, s.Subdomain)
	return nil
}

func (m *MemStore) IsSubdomainReserved(_ context.Context, orgID, subdomain string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	id, ok := m.subByName[strings.ToLower(subdomain)]
	if !ok {
		return false, nil
	}
	return m.subs[id].OrgID == orgID, nil
}

func (m *MemStore) CountSubdomains(_ context.Context, orgID string) (int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	n := 0
	for _, s := range m.subs {
		if s.OrgID == orgID {
			n++
		}
	}
	return n, nil
}

func (m *MemStore) AddCustomDomain(_ context.Context, d CustomDomain) (CustomDomain, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	name := strings.ToLower(d.Domain)
	if _, ok := m.domByName[name]; ok {
		return CustomDomain{}, ErrConflict
	}
	if d.ID == "" {
		d.ID = NewID("dom")
	}
	if d.CreatedAt.IsZero() {
		d.CreatedAt = time.Now()
	}
	if d.CertStatus == "" {
		d.CertStatus = "pending"
	}
	d.Domain = name
	m.domains[d.ID] = d
	m.domByName[name] = d.ID
	return d, nil
}

func (m *MemStore) GetCustomDomain(_ context.Context, id, orgID string) (CustomDomain, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	d, ok := m.domains[id]
	if !ok || d.OrgID != orgID {
		return CustomDomain{}, ErrNotFound
	}
	return d, nil
}

func (m *MemStore) GetCustomDomainByName(_ context.Context, domain string) (CustomDomain, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	id, ok := m.domByName[strings.ToLower(domain)]
	if !ok {
		return CustomDomain{}, ErrNotFound
	}
	return m.domains[id], nil
}

func (m *MemStore) ListCustomDomains(_ context.Context, orgID string) ([]CustomDomain, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []CustomDomain
	for _, d := range m.domains {
		if d.OrgID == orgID {
			out = append(out, d)
		}
	}
	return out, nil
}

func (m *MemStore) VerifyCustomDomain(_ context.Context, id, orgID string, at time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	d, ok := m.domains[id]
	if !ok || d.OrgID != orgID {
		return ErrNotFound
	}
	d.VerifiedAt = &at
	d.CertStatus = "verified"
	m.domains[id] = d
	return nil
}

func (m *MemStore) CountCustomDomains(_ context.Context, orgID string) (int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	n := 0
	for _, d := range m.domains {
		if d.OrgID == orgID {
			n++
		}
	}
	return n, nil
}

func (m *MemStore) UpsertUsage(_ context.Context, u UsageRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.usage = append(m.usage, u)
	return nil
}

func (m *MemStore) UsageForOrg(_ context.Context, orgID string, since time.Time) (UsageRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	agg := UsageRecord{OrgID: orgID, WindowStart: since, WindowEnd: time.Now()}
	for _, u := range m.usage {
		if u.OrgID == orgID && !u.WindowEnd.Before(since) {
			agg.BytesIn += u.BytesIn
			agg.BytesOut += u.BytesOut
			agg.Requests += u.Requests
		}
	}
	return agg, nil
}

// --- Billing ---

func (m *MemStore) UpsertSubscription(_ context.Context, s Subscription) (Subscription, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s.ID == "" {
		if existing, ok := m.subsByOrg[s.OrgID]; ok {
			s.ID = existing.ID
			if s.CreatedAt.IsZero() {
				s.CreatedAt = existing.CreatedAt
			}
		} else {
			s.ID = NewID("sub")
		}
	}
	if s.CreatedAt.IsZero() {
		s.CreatedAt = time.Now()
	}
	s.UpdatedAt = time.Now()
	m.subsByOrg[s.OrgID] = s
	if s.StripeSubID != "" {
		m.subsByStripe[s.StripeSubID] = s.OrgID
	}
	return s, nil
}

func (m *MemStore) GetSubscriptionForOrg(_ context.Context, orgID string) (Subscription, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.subsByOrg[orgID]
	if !ok {
		return Subscription{}, ErrNotFound
	}
	return s, nil
}

func (m *MemStore) DeleteSubscription(_ context.Context, stripeSubID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	orgID, ok := m.subsByStripe[stripeSubID]
	if !ok {
		return ErrNotFound
	}
	delete(m.subsByStripe, stripeSubID)
	delete(m.subsByOrg, orgID)
	return nil
}

func (m *MemStore) GetOrgByStripeCustomer(_ context.Context, customerID string) (Org, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if customerID == "" {
		return Org{}, ErrNotFound
	}
	for _, o := range m.orgs {
		if o.StripeCustomerID == customerID {
			return o, nil
		}
	}
	return Org{}, ErrNotFound
}

func (m *MemStore) MarkEventProcessed(_ context.Context, eventID, _ string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if eventID == "" {
		return true, nil // unidentified events are always processed (never deduped)
	}
	if m.events[eventID] {
		return false, nil
	}
	m.events[eventID] = true
	return true, nil
}

func (m *MemStore) InsertMeteredUsage(_ context.Context, u MeteredUsage) (MeteredUsage, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, e := range m.metered {
		if e.OrgID == u.OrgID && e.Metric == u.Metric && e.WindowStart.Equal(u.WindowStart) {
			return MeteredUsage{}, ErrConflict
		}
	}
	m.meteredSeq++
	u.ID = m.meteredSeq
	if u.CreatedAt.IsZero() {
		u.CreatedAt = time.Now()
	}
	m.metered = append(m.metered, u)
	return u, nil
}

func (m *MemStore) PendingMeteredUsage(_ context.Context, limit int) ([]MeteredUsage, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []MeteredUsage
	for _, e := range m.metered {
		if e.ReportedAt == nil {
			out = append(out, e)
			if limit > 0 && len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}

func (m *MemStore) MarkMeteredUsageReported(_ context.Context, ids []int64, at time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	want := make(map[int64]bool, len(ids))
	for _, id := range ids {
		want[id] = true
	}
	for i := range m.metered {
		if want[m.metered[i].ID] {
			t := at
			m.metered[i].ReportedAt = &t
		}
	}
	return nil
}

func (m *MemStore) Close() error { return nil }
