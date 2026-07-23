package server_test

import (
	"context"
	"testing"
	"time"

	"github.com/trqsh-uz/trqsh/internal/server"
)

func TestRouteKey(t *testing.T) {
	cases := []struct {
		r    server.Route
		want string
	}{
		{server.Route{Host: "Abc.TRQSH.uz"}, "host:abc.trqsh.uz"},
		{server.Route{Proto: "tcp", Port: 2222}, "port:tcp:2222"},
		{server.Route{Proto: "udp", Port: 5353}, "port:udp:5353"},
	}
	for _, c := range cases {
		if got := c.r.Key(); got != c.want {
			t.Errorf("Key(%+v) = %q, want %q", c.r, got, c.want)
		}
	}
}

func TestInMemoryRegistryBindLookupUnbind(t *testing.T) {
	ctx := context.Background()
	reg := server.NewInMemoryRegistry()
	route := server.Route{Host: "demo.lvh.me"}
	bind := server.Binding{EdgeID: "edge-1", SessionID: "sess_1"}

	if _, ok, _ := reg.Lookup(ctx, route); ok {
		t.Fatal("expected miss before bind")
	}
	if err := reg.Bind(ctx, route, bind, time.Minute); err != nil {
		t.Fatalf("Bind: %v", err)
	}
	got, ok, err := reg.Lookup(ctx, route)
	if err != nil || !ok {
		t.Fatalf("Lookup after bind: ok=%v err=%v", ok, err)
	}
	if got != bind {
		t.Errorf("Lookup = %+v, want %+v", got, bind)
	}
	if err := reg.Unbind(ctx, route); err != nil {
		t.Fatalf("Unbind: %v", err)
	}
	if _, ok, _ := reg.Lookup(ctx, route); ok {
		t.Fatal("expected miss after unbind")
	}
}

func TestInMemoryRegistryTTLExpiry(t *testing.T) {
	ctx := context.Background()
	reg := server.NewInMemoryRegistry()
	route := server.Route{Proto: "tcp", Port: 2222}
	if err := reg.Bind(ctx, route, server.Binding{EdgeID: "e"}, 20*time.Millisecond); err != nil {
		t.Fatalf("Bind: %v", err)
	}
	if _, ok, _ := reg.Lookup(ctx, route); !ok {
		t.Fatal("expected hit before expiry")
	}
	time.Sleep(40 * time.Millisecond)
	if _, ok, _ := reg.Lookup(ctx, route); ok {
		t.Fatal("expected miss after TTL expiry")
	}
}
