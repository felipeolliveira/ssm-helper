package main

import (
	"crypto/ed25519"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/felipeolliveira/ssm-helper/internal/update"
)

func TestGenerateAndSign(t *testing.T) {
	dir := t.TempDir()
	name := filepath.Join(dir, "throwaway")
	if err := generate([]string{"-out", name}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(name + ".key")
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o077 != 0 && os.PathSeparator == '/' {
		t.Errorf("private key mode = %v, want readable only by the owner", info.Mode().Perm())
	}
	if err := generate([]string{"-out", name}); err == nil {
		t.Error("generate overwrote an existing key")
	}

	priv, _ := os.ReadFile(name + ".key")
	pubText, _ := os.ReadFile(name + ".pub")
	pub, err := update.ParsePublicKey(string(pubText))
	if err != nil {
		t.Fatal(err)
	}

	sums := filepath.Join(dir, "SHA256SUMS.txt")
	os.WriteFile(sums, []byte("abc  ssm-helper.exe\n"), 0o644)
	t.Setenv("TEST_SIGNING_KEY", string(priv))
	if err := sign([]string{"-key-env", "TEST_SIGNING_KEY", sums}); err != nil {
		t.Fatal(err)
	}
	sig, _ := os.ReadFile(sums + ".sig")
	data, _ := os.ReadFile(sums)
	if !update.Verify([]ed25519.PublicKey{pub}, data, string(sig)) {
		t.Error("the signature written by sign does not verify")
	}

	t.Setenv("TEST_SIGNING_KEY", "")
	if err := sign([]string{"-key-env", "TEST_SIGNING_KEY", sums}); err == nil || !strings.Contains(err.Error(), "empty") {
		t.Errorf("sign with an empty secret = %v", err)
	}
}
