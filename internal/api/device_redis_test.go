package api

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/trqsh-uz/trqsh/internal/api/auth"
)

// TestDeviceRedisCrossInstance is the actual bug being fixed: the CLI polls
// whichever API replica a load balancer picks, which is not necessarily the
// replica the browser's Approve landed on. Three separate redisDeviceStore
// instances (simulating three replicas hit at three different points in the
// flow) share one miniredis.
func TestDeviceRedisCrossInstance(t *testing.T) {
	_, rdb := newTestRedis(t)
	replicaA := newRedisDeviceStore(rdb, discardLogger())
	replicaB := newRedisDeviceStore(rdb, discardLogger())
	replicaC := newRedisDeviceStore(rdb, discardLogger())

	req := replicaA.Create()
	if req.DeviceCode == "" || req.UserCode == "" {
		t.Fatal("Create returned empty codes")
	}

	if err := replicaB.Approve(req.UserCode, "org_123", "tq_live_abc"); err != nil {
		t.Fatalf("Approve on replicaB: %v", err)
	}

	key, err := replicaC.Poll(req.DeviceCode)
	if err != nil {
		t.Fatalf("Poll on replicaC: %v", err)
	}
	if key != "tq_live_abc" {
		t.Fatalf("Poll returned %q, want the approved API key", key)
	}
}

func TestDeviceRedisPollBeforeApprovePending(t *testing.T) {
	_, rdb := newTestRedis(t)
	store := newRedisDeviceStore(rdb, discardLogger())

	req := store.Create()
	_, err := store.Poll(req.DeviceCode)
	if err != auth.ErrDevicePending {
		t.Fatalf("Poll before Approve = %v, want ErrDevicePending", err)
	}
}

func TestDeviceRedisPollUnknownDeviceCode(t *testing.T) {
	_, rdb := newTestRedis(t)
	store := newRedisDeviceStore(rdb, discardLogger())

	if _, err := store.Poll("never-issued"); err != auth.ErrDeviceUnknown {
		t.Fatalf("Poll(unknown) = %v, want ErrDeviceUnknown", err)
	}
}

func TestDeviceRedisApproveUnknownUserCode(t *testing.T) {
	_, rdb := newTestRedis(t)
	store := newRedisDeviceStore(rdb, discardLogger())

	if err := store.Approve("NEVER-ISSUED", "org_1", "key"); err != auth.ErrDeviceUnknown {
		t.Fatalf("Approve(unknown user_code) = %v, want ErrDeviceUnknown", err)
	}
}

// TestDeviceRedisPollConsumesOnSuccess: a device code is single-use — once
// Poll returns the key, a second Poll must not hand it out again.
func TestDeviceRedisPollConsumesOnSuccess(t *testing.T) {
	_, rdb := newTestRedis(t)
	store := newRedisDeviceStore(rdb, discardLogger())

	req := store.Create()
	if err := store.Approve(req.UserCode, "org_1", "key"); err != nil {
		t.Fatalf("Approve: %v", err)
	}
	if _, err := store.Poll(req.DeviceCode); err != nil {
		t.Fatalf("first Poll: %v", err)
	}
	if _, err := store.Poll(req.DeviceCode); err != auth.ErrDeviceUnknown {
		t.Fatalf("second Poll = %v, want ErrDeviceUnknown (already consumed)", err)
	}
}

// TestDeviceRedisExpiry proves Poll distinguishes "expired" from "unknown"
// exactly like the in-process store: expiry is judged by the embedded
// CreatedAt, not by Redis's own TTL (which only backstops truly abandoned
// records — see redisDeviceSafetyMargin). A record is written directly,
// bypassing Create, to control CreatedAt precisely without needing to wait
// out the real TTL.
func TestDeviceRedisExpiry(t *testing.T) {
	_, rdb := newTestRedis(t)
	store := newRedisDeviceStore(rdb, discardLogger())

	old := auth.DeviceRequest{
		DeviceCode: "dc_old",
		UserCode:   "OLD-CODE",
		CreatedAt:  time.Now().Add(-(auth.DeviceTTL + time.Minute)),
	}
	buf, err := json.Marshal(old)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	ctx := t.Context()
	if err := rdb.Set(ctx, store.codeKey(old.DeviceCode), buf, time.Hour).Err(); err != nil {
		t.Fatalf("seed redis: %v", err)
	}
	if err := rdb.Set(ctx, store.userKey(old.UserCode), old.DeviceCode, time.Hour).Err(); err != nil {
		t.Fatalf("seed redis: %v", err)
	}

	if _, err := store.Poll(old.DeviceCode); err != auth.ErrDeviceExpired {
		t.Fatalf("Poll(expired) = %v, want ErrDeviceExpired", err)
	}
	// Expiry must also consume the record, matching the in-process store.
	if _, err := store.Poll(old.DeviceCode); err != auth.ErrDeviceUnknown {
		t.Fatalf("Poll after expiry-consume = %v, want ErrDeviceUnknown", err)
	}
}

// TestDeviceRedisApproveDoesNotRejectExpired locks in the in-process store's
// existing (if surprising) behavior that Approve does not itself check
// expiry — only Poll does, lazily. Changing this would be a real behavior
// change, not a bug fix, so this test guards against accidentally tightening
// it during the Redis port.
func TestDeviceRedisApproveDoesNotRejectExpired(t *testing.T) {
	_, rdb := newTestRedis(t)
	store := newRedisDeviceStore(rdb, discardLogger())

	old := auth.DeviceRequest{
		DeviceCode: "dc_old2",
		UserCode:   "OLD-CODE2",
		CreatedAt:  time.Now().Add(-(auth.DeviceTTL + time.Minute)),
	}
	buf, err := json.Marshal(old)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	ctx := t.Context()
	if err := rdb.Set(ctx, store.codeKey(old.DeviceCode), buf, time.Hour).Err(); err != nil {
		t.Fatalf("seed redis: %v", err)
	}
	if err := rdb.Set(ctx, store.userKey(old.UserCode), old.DeviceCode, time.Hour).Err(); err != nil {
		t.Fatalf("seed redis: %v", err)
	}

	if err := store.Approve(old.UserCode, "org_1", "key"); err != nil {
		t.Fatalf("Approve on an expired-but-not-yet-polled record = %v, want nil (matches in-process semantics)", err)
	}
}
