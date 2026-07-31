package api

import (
	"testing"
	"time"
)

// TestOAuthStateRedisCrossInstance is the actual bug being fixed: a state
// nonce issued by one API replica must validate on the OAuth callback even
// when a load balancer routes that callback to a different replica. Two
// separate redisStateStore instances (simulating two replicas) share one
// miniredis, standing in for the shared Redis every replica points at.
func TestOAuthStateRedisCrossInstance(t *testing.T) {
	_, rdb := newTestRedis(t)
	replicaA := newRedisStateStore(rdb, discardLogger())
	replicaB := newRedisStateStore(rdb, discardLogger())

	state := replicaA.New()
	if state == "" {
		t.Fatal("New returned an empty state")
	}
	if !replicaB.Check(state) {
		t.Fatal("state issued by replicaA did not validate on replicaB")
	}
}

// TestOAuthStateRedisSingleUse: a state nonce is consumed by the first Check,
// so a replay (a duplicated/forged callback) is rejected.
func TestOAuthStateRedisSingleUse(t *testing.T) {
	_, rdb := newTestRedis(t)
	store := newRedisStateStore(rdb, discardLogger())

	state := store.New()
	if !store.Check(state) {
		t.Fatal("first Check should validate")
	}
	if store.Check(state) {
		t.Fatal("second Check of the same state should be rejected (single-use)")
	}
}

func TestOAuthStateRedisUnknownRejected(t *testing.T) {
	_, rdb := newTestRedis(t)
	store := newRedisStateStore(rdb, discardLogger())

	if store.Check("never-issued") {
		t.Fatal("an unissued state must not validate")
	}
}

func TestOAuthStateRedisEmptyRejected(t *testing.T) {
	_, rdb := newTestRedis(t)
	store := newRedisStateStore(rdb, discardLogger())

	if store.Check("") {
		t.Fatal("an empty state must never validate")
	}
}

// TestOAuthStateRedisTTLExpiry: a nonce older than oauthStateTTL must be
// rejected even though it was never explicitly Check-consumed — Redis's own
// TTL is the expiry mechanism here (unlike the device store, which layers an
// explicit CreatedAt check on top for a distinguishable expired/unknown
// error). miniredis's FastForward simulates the TTL lapsing without a real
// wait.
func TestOAuthStateRedisTTLExpiry(t *testing.T) {
	mr, rdb := newTestRedis(t)
	store := newRedisStateStore(rdb, discardLogger())

	state := store.New()
	mr.FastForward(oauthStateTTL + time.Second)

	if store.Check(state) {
		t.Fatal("a state past its TTL must not validate")
	}
}

// TestOAuthStateMemStoreParity locks in that moving the original logic out of
// Server into memStateStore preserved its exact behavior (issue, single
// consume, reject unknown/empty).
func TestOAuthStateMemStoreParity(t *testing.T) {
	store := newMemStateStore()

	state := store.New()
	if !store.Check(state) {
		t.Fatal("first Check should validate")
	}
	if store.Check(state) {
		t.Fatal("second Check of the same state should be rejected (single-use)")
	}
	if store.Check("") {
		t.Fatal("empty state must never validate")
	}
	if store.Check("never-issued") {
		t.Fatal("unissued state must not validate")
	}
}
