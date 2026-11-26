package update

import (
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
)

// SigName is the signature of SumsName attached to each release.
const SigName = SumsName + ".sig"

// signContext keeps a signature made for this purpose from being valid for
// anything else signed with the same key.
const signContext = "ssm-helper update checksums v1\n"

// SignedMessage is what is signed: the context, the release tag and
// SHA256SUMS.txt. With the tag in it, the files of an older release cannot be
// published again under a newer tag to roll every copy back to that version.
func SignedMessage(tag string, sums []byte) []byte {
	msg := []byte(signContext + "version " + tag + "\n")
	return append(msg, sums...)
}

// Sign returns the signature of the checksums of the release tag, base64
// encoded, as written to SigName.
func Sign(key ed25519.PrivateKey, tag string, sums []byte) string {
	return base64.StdEncoding.EncodeToString(ed25519.Sign(key, SignedMessage(tag, sums)))
}

// ParsePrivateKey reads a key written by "updatekey generate": the base64 seed.
func ParsePrivateKey(s string) (ed25519.PrivateKey, error) {
	seed, err := base64.StdEncoding.DecodeString(strings.TrimSpace(s))
	if err != nil || len(seed) != ed25519.SeedSize {
		return nil, errors.New("not an update signing key")
	}
	return ed25519.NewKeyFromSeed(seed), nil
}

// ParsePublicKey reads a public key written by "updatekey generate".
func ParsePublicKey(s string) (ed25519.PublicKey, error) {
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(s))
	if err != nil || len(key) != ed25519.PublicKeySize {
		return nil, errors.New("not an update public key")
	}
	return ed25519.PublicKey(key), nil
}

// PublicKeys returns the keys built into the app.
func PublicKeys() ([]ed25519.PublicKey, error) {
	var keys []ed25519.PublicKey
	for i, s := range publicKeys {
		if strings.TrimSpace(s) == "" {
			continue
		}
		key, err := ParsePublicKey(s)
		if err != nil {
			return nil, fmt.Errorf("public key %d: %w", i+1, err)
		}
		keys = append(keys, key)
	}
	if len(keys) == 0 {
		return nil, errors.New("no update public key is set")
	}
	return keys, nil
}

// Verify checks sig (as written to SigName) against the checksums of the
// release tag with any of keys.
func Verify(keys []ed25519.PublicKey, tag string, sums []byte, sig string) bool {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(sig))
	if err != nil || len(raw) != ed25519.SignatureSize {
		return false
	}
	message := SignedMessage(tag, sums)
	for _, key := range keys {
		if ed25519.Verify(key, message, raw) {
			return true
		}
	}
	return false
}
