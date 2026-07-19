package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// API key format: "rk_live_<16-hex-prefix>_<48-hex-secret>". The prefix segment
// (including "rk_live_") is the lookup key stored in the clear; the argon2id
// hash of the full key is stored for verification. The plaintext is shown once.
const (
	keyEnv    = "rk_live_"
	prefixLen = 8  // bytes -> 16 hex chars
	secretLen = 24 // bytes -> 48 hex chars
)

// GeneratedKey is a freshly minted API key.
type GeneratedKey struct {
	Full   string // shown once to the user
	Prefix string // stored for lookup (e.g. "rk_live_ab12…")
	Hash   string // argon2id encoded hash, stored
}

// GenerateAPIKey mints a new key, returning its plaintext, lookup prefix, and hash.
func GenerateAPIKey() (GeneratedKey, error) {
	pb := make([]byte, prefixLen)
	sb := make([]byte, secretLen)
	if _, err := rand.Read(pb); err != nil {
		return GeneratedKey{}, err
	}
	if _, err := rand.Read(sb); err != nil {
		return GeneratedKey{}, err
	}
	prefix := keyEnv + hex.EncodeToString(pb)
	full := prefix + "_" + hex.EncodeToString(sb)
	return GeneratedKey{Full: full, Prefix: prefix, Hash: HashKey(full)}, nil
}

// KeyPrefix extracts the lookup prefix from a raw key.
func KeyPrefix(raw string) (string, bool) {
	if !strings.HasPrefix(raw, keyEnv) {
		return "", false
	}
	// prefix is keyEnv + 16 hex chars, then "_" then the secret.
	rest := raw[len(keyEnv):]
	i := strings.IndexByte(rest, '_')
	if i < 0 {
		return "", false
	}
	return keyEnv + rest[:i], true
}

// argon2id parameters (interactive-strength).
const (
	argonTime    = 1
	argonMemory  = 64 * 1024
	argonThreads = 4
	argonKeyLen  = 32
	argonSaltLen = 16
)

// HashKey returns a PHC-style argon2id encoding of the key.
func HashKey(full string) string {
	salt := make([]byte, argonSaltLen)
	_, _ = rand.Read(salt)
	sum := argon2.IDKey([]byte(full), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemory, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(sum))
}

// VerifyKey checks a raw key against an encoded argon2id hash.
func VerifyKey(full, encoded string) bool {
	parts := strings.Split(encoded, "$")
	// ["", "argon2id", "v=19", "m=...,t=...,p=...", salt, hash]
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false
	}
	var memory, tCost uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &tCost, &threads); err != nil {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false
	}
	got := argon2.IDKey([]byte(full), salt, tCost, memory, threads, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1
}
