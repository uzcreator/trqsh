package stripe

import (
	"strings"
	"testing"
	"time"
)

func TestVerifySignatureRoundTrip(t *testing.T) {
	secret := "whsec_test_123"
	payload := []byte(`{"id":"evt_1","type":"customer.subscription.updated"}`)
	sig := SignPayload(payload, secret, time.Now())

	if err := VerifySignature(payload, sig, secret, 5*time.Minute); err != nil {
		t.Fatalf("valid signature rejected: %v", err)
	}
}

func TestVerifySignatureRejectsTamperedPayload(t *testing.T) {
	secret := "whsec_test_123"
	payload := []byte(`{"amount":100}`)
	sig := SignPayload(payload, secret, time.Now())

	tampered := []byte(`{"amount":999}`)
	if err := VerifySignature(tampered, sig, secret, 5*time.Minute); err == nil {
		t.Fatal("tampered payload should fail verification")
	}
}

func TestVerifySignatureRejectsWrongSecret(t *testing.T) {
	payload := []byte(`{"x":1}`)
	sig := SignPayload(payload, "whsec_real", time.Now())
	if err := VerifySignature(payload, sig, "whsec_attacker", 5*time.Minute); err == nil {
		t.Fatal("wrong secret should fail verification")
	}
}

func TestVerifySignatureRejectsStaleTimestamp(t *testing.T) {
	secret := "whsec_test"
	payload := []byte(`{"x":1}`)
	old := time.Now().Add(-10 * time.Minute)
	sig := SignPayload(payload, secret, old)
	if err := VerifySignature(payload, sig, secret, 5*time.Minute); err == nil {
		t.Fatal("stale timestamp should fail within a 5m tolerance")
	}
	// With a generous tolerance the same signature verifies (proves it was the age).
	if err := VerifySignature(payload, sig, secret, time.Hour); err != nil {
		t.Fatalf("signature should verify with a wide tolerance: %v", err)
	}
}

func TestVerifySignatureMalformedHeader(t *testing.T) {
	secret := "whsec_test"
	payload := []byte(`{}`)
	for _, h := range []string{"", "garbage", "t=123", "v1=abc", "t=,v1="} {
		if err := VerifySignature(payload, h, secret, 0); err == nil {
			t.Fatalf("malformed header %q should fail", h)
		}
	}
}

func TestVerifySignatureMultipleV1(t *testing.T) {
	secret := "whsec_test"
	payload := []byte(`{"x":1}`)
	good := SignPayload(payload, secret, time.Now())
	// A header may carry several v1 signatures during secret rotation; any match wins.
	combined := good + ",v1=deadbeef"
	if err := VerifySignature(payload, combined, secret, time.Minute); err != nil {
		t.Fatalf("one matching v1 among many should verify: %v", err)
	}
	if !strings.Contains(combined, "v1=deadbeef") {
		t.Fatal("test setup error")
	}
}

func TestParseEventAndSubscriptionObject(t *testing.T) {
	payload := []byte(`{
	  "id":"evt_9","type":"customer.subscription.updated",
	  "data":{"object":{
	    "id":"sub_9","customer":"cus_9","status":"active","cancel_at_period_end":false,
	    "current_period_end":1893456000,
	    "items":{"data":[{"price":{"id":"price_pro_monthly"}}]},
	    "metadata":{"org_id":"org_9"}
	  }}
	}`)
	ev, err := ParseEvent(payload)
	if err != nil {
		t.Fatalf("ParseEvent: %v", err)
	}
	if ev.ID != "evt_9" || ev.Type != "customer.subscription.updated" {
		t.Fatalf("bad envelope: %+v", ev)
	}
	var sub SubscriptionObject
	if err := ev.Decode(&sub); err != nil {
		t.Fatalf("decode object: %v", err)
	}
	if sub.PriceID() != "price_pro_monthly" {
		t.Fatalf("PriceID = %q", sub.PriceID())
	}
	if sub.Customer != "cus_9" || sub.Metadata["org_id"] != "org_9" {
		t.Fatalf("bad sub object: %+v", sub)
	}
	if sub.PeriodEnd().IsZero() {
		t.Fatal("PeriodEnd should be set")
	}
}
