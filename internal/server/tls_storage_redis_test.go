package server

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"reflect"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/caddyserver/certmagic"
)

// TestACMECertManagerStorageSelection verifies the wiring: a configured Redis URL
// selects the shared RedisStorage (cluster mode), while ACME-on without Redis keeps
// the unchanged on-disk FileStorage (single-edge default). This is the item-2
// requirement that Redis is opt-in and never forced.
func TestACMECertManagerStorageSelection(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)

	redisCfg := Config{BaseDomain: "trqsh.uz", ACMEEmail: "ops@trqsh.uz", ACMEStaging: true, RedisURL: "redis://" + mr.Addr()}
	cm, err := newACMECertManager(redisCfg, nil, func(context.Context, string) error { return nil })
	if err != nil {
		t.Fatalf("newACMECertManager (redis): %v", err)
	}
	if _, ok := cm.storage.(*RedisStorage); !ok {
		t.Fatalf("storage = %T, want *RedisStorage when RedisURL is set", cm.storage)
	}
	if err := cm.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	fileCfg := Config{BaseDomain: "trqsh.uz", ACMEEmail: "ops@trqsh.uz", ACMEStaging: true, TLSStorageDir: t.TempDir()}
	cm2, err := newACMECertManager(fileCfg, nil, func(context.Context, string) error { return nil })
	if err != nil {
		t.Fatalf("newACMECertManager (file): %v", err)
	}
	if _, ok := cm2.storage.(*certmagic.FileStorage); !ok {
		t.Fatalf("storage = %T, want *certmagic.FileStorage when only TLSStorageDir is set", cm2.storage)
	}
}

func newTestStorage(t *testing.T) *RedisStorage {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	st, err := NewRedisStorage("redis://"+mr.Addr(), nil)
	if err != nil {
		t.Fatalf("NewRedisStorage: %v", err)
	}
	st.pollWait = 10 * time.Millisecond // speed up lock-contention polling in tests
	t.Cleanup(func() { _ = st.Close() })
	return st
}

// TestRedisStorageStoreLoad round-trips bytes (including a NUL and high bytes to
// prove binary safety), then Stat/Exists/Delete behave.
func TestRedisStorageStoreLoad(t *testing.T) {
	st := newTestStorage(t)
	ctx := context.Background()
	key := "certificates/acme-v02.api.letsencrypt.org-directory/example.com/example.com.crt"
	val := []byte("-----BEGIN CERTIFICATE-----\x00\x01\xff binary\n-----END-----")

	if err := st.Store(ctx, key, val); err != nil {
		t.Fatalf("Store: %v", err)
	}
	got, err := st.Load(ctx, key)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !bytes.Equal(got, val) {
		t.Fatalf("Load round-trip mismatch: got %q want %q", got, val)
	}
	if !st.Exists(ctx, key) {
		t.Fatal("Exists=false right after Store")
	}
	ki, err := st.Stat(ctx, key)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if ki.Key != key || !ki.IsTerminal || ki.Size != int64(len(val)) {
		t.Fatalf("Stat = %+v, want key=%q terminal size=%d", ki, key, len(val))
	}
	if ki.Modified.IsZero() {
		t.Error("Stat.Modified is zero; want the store timestamp")
	}

	if err := st.Delete(ctx, key); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if st.Exists(ctx, key) {
		t.Fatal("Exists=true after Delete")
	}
	if _, err := st.Load(ctx, key); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("Load after Delete err=%v, want fs.ErrNotExist", err)
	}
}

func TestRedisStorageLoadMissing(t *testing.T) {
	st := newTestStorage(t)
	if _, err := st.Load(context.Background(), "does/not/exist"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("Load missing err=%v, want fs.ErrNotExist", err)
	}
	if _, err := st.Stat(context.Background(), "does/not/exist"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("Stat missing err=%v, want fs.ErrNotExist", err)
	}
	// Deleting a missing key is not an error (matches FileStorage's os.RemoveAll).
	if err := st.Delete(context.Background(), "does/not/exist"); err != nil {
		t.Fatalf("Delete missing = %v, want nil", err)
	}
}

// TestRedisStorageDirectory covers the hierarchical "directory" semantics certmagic
// relies on: List of immediate children, Exists/Stat on a directory, and a prefix
// Delete removing a whole subtree while leaving siblings intact.
func TestRedisStorageDirectory(t *testing.T) {
	st := newTestStorage(t)
	ctx := context.Background()
	const issuer = "certificates/acme-v02"
	keys := []string{
		issuer + "/example.com/example.com.crt",
		issuer + "/example.com/example.com.key",
		issuer + "/example.com/example.com.json",
		issuer + "/other.com/other.com.crt",
	}
	for _, k := range keys {
		if err := st.Store(ctx, k, []byte("x")); err != nil {
			t.Fatalf("Store %s: %v", k, err)
		}
	}

	// A prefix with children Exists as a directory and Stats as non-terminal.
	if !st.Exists(ctx, issuer+"/example.com") {
		t.Fatal("directory Exists=false")
	}
	ki, err := st.Stat(ctx, issuer+"/example.com")
	if err != nil {
		t.Fatalf("Stat dir: %v", err)
	}
	if ki.IsTerminal {
		t.Fatal("directory Stat IsTerminal=true, want false")
	}

	// Non-recursive List of the issuer returns the two immediate site directories.
	children, err := st.List(ctx, issuer, false)
	if err != nil {
		t.Fatalf("List issuer: %v", err)
	}
	sort.Strings(children)
	want := []string{issuer + "/example.com", issuer + "/other.com"}
	if !reflect.DeepEqual(children, want) {
		t.Fatalf("List(%q) = %v, want %v", issuer, children, want)
	}

	// Non-recursive List of a leaf site returns its three files.
	files, err := st.List(ctx, issuer+"/example.com", false)
	if err != nil {
		t.Fatalf("List site: %v", err)
	}
	if len(files) != 3 {
		t.Fatalf("List site returned %d entries, want 3: %v", len(files), files)
	}

	// Recursive List of the issuer returns all four descendant file keys.
	all, err := st.List(ctx, issuer, true)
	if err != nil {
		t.Fatalf("List recursive: %v", err)
	}
	if len(all) != 4 {
		t.Fatalf("recursive List returned %d, want 4: %v", len(all), all)
	}

	// Prefix Delete removes the whole example.com subtree, siblings untouched.
	if err := st.Delete(ctx, issuer+"/example.com"); err != nil {
		t.Fatalf("Delete dir: %v", err)
	}
	if st.Exists(ctx, issuer+"/example.com") {
		t.Fatal("directory still Exists after prefix Delete")
	}
	if st.Exists(ctx, issuer+"/example.com/example.com.crt") {
		t.Fatal("file under deleted directory still Exists")
	}
	if !st.Exists(ctx, issuer+"/other.com/other.com.crt") {
		t.Fatal("sibling directory wrongly deleted")
	}
	// Listing the now-missing directory reports fs.ErrNotExist.
	if _, err := st.List(ctx, issuer+"/example.com", false); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("List deleted dir err=%v, want fs.ErrNotExist", err)
	}
}

// TestRedisStorageLockBlocks proves mutual exclusion: while one lock is held a
// second Lock for the same name blocks, then acquires once the first is released.
func TestRedisStorageLockBlocks(t *testing.T) {
	st := newTestStorage(t)
	ctx := context.Background()
	const name = "issue_cert:example.com"

	if err := st.Lock(ctx, name); err != nil {
		t.Fatalf("first Lock: %v", err)
	}

	second := make(chan error, 1)
	go func() { second <- st.Lock(ctx, name) }()

	select {
	case err := <-second:
		t.Fatalf("second Lock returned (%v) while the lock is held; want it to block", err)
	case <-time.After(200 * time.Millisecond):
		// still blocked — correct
	}

	if err := st.Unlock(ctx, name); err != nil {
		t.Fatalf("Unlock: %v", err)
	}
	select {
	case err := <-second:
		if err != nil {
			t.Fatalf("second Lock after Unlock: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("second Lock did not acquire within 2s after Unlock")
	}
	if err := st.Unlock(ctx, name); err != nil {
		t.Fatalf("final Unlock: %v", err)
	}
}

// TestRedisStorageLockSingleHolder is the race test: many goroutines contend for
// the same lock; at no instant may more than one hold it. A broken distributed lock
// would let the observed max concurrency exceed 1.
func TestRedisStorageLockSingleHolder(t *testing.T) {
	st := newTestStorage(t)
	ctx := context.Background()
	const name = "issue_cert:race.example.com"
	const workers = 25

	var concurrent int32
	var maxConcurrent int32
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := st.Lock(ctx, name); err != nil {
				t.Errorf("Lock: %v", err)
				return
			}
			c := atomic.AddInt32(&concurrent, 1)
			for {
				m := atomic.LoadInt32(&maxConcurrent)
				if c <= m || atomic.CompareAndSwapInt32(&maxConcurrent, m, c) {
					break
				}
			}
			time.Sleep(3 * time.Millisecond) // hold the critical section briefly
			atomic.AddInt32(&concurrent, -1)
			if err := st.Unlock(ctx, name); err != nil {
				t.Errorf("Unlock: %v", err)
			}
		}()
	}
	wg.Wait()

	if maxConcurrent != 1 {
		t.Fatalf("max concurrent lock holders = %d, want 1 (mutual exclusion violated)", maxConcurrent)
	}
}
