// Command updatekey manages the key that signs the update checksums
// (SHA256SUMS.txt) of each release. See "Signing releases" in DEVELOPMENT.md.
//
//	go run ./tools/updatekey generate -out primary
//	    writes primary.key (private, keep it secret) and primary.pub
//	go run ./tools/updatekey sign -key-env UPDATE_SIGNING_KEY -version v1.4.0 build/bin/SHA256SUMS.txt
//	    writes build/bin/SHA256SUMS.txt.sig for that release (used by the release workflow)
//	go run ./tools/updatekey verify -version v1.4.0 build/bin/SHA256SUMS.txt
//	    checks the .sig with the public keys built into the app
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/felipeolliveira/ssm-helper/internal/update"
)

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	var err error
	switch os.Args[1] {
	case "generate":
		err = generate(os.Args[2:])
	case "sign":
		err = sign(os.Args[2:])
	case "verify":
		err = verify(os.Args[2:])
	default:
		usage()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "updatekey:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: updatekey generate -out <name> | sign -key-env <VAR> -version <tag> <file> | verify -version <tag> <file>")
	os.Exit(2)
}

// generate writes <name>.key and <name>.pub, refusing to overwrite either.
// Nothing secret is printed, so the output is safe to share.
func generate(args []string) error {
	fs := flag.NewFlagSet("generate", flag.ExitOnError)
	out := fs.String("out", "", "name of the files to write: <name>.key and <name>.pub")
	fs.Parse(args)
	if *out == "" {
		return errors.New("-out is required")
	}
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	keyFile, pubFile := *out+".key", *out+".pub"
	if err := writeNew(keyFile, base64.StdEncoding.EncodeToString(priv.Seed())+"\n", 0o600); err != nil {
		return err
	}
	if err := writeNew(pubFile, base64.StdEncoding.EncodeToString(pub)+"\n", 0o644); err != nil {
		os.Remove(keyFile)
		return err
	}
	fmt.Printf("wrote %s (private: keep it secret) and %s (public)\n", keyFile, pubFile)
	return nil
}

func writeNew(path, content string, mode os.FileMode) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	if _, err := f.WriteString(content); err != nil {
		f.Close()
		os.Remove(path)
		return err
	}
	return f.Close()
}

// sign reads the private key from an environment variable, so it never
// touches the disk in CI, and writes <file>.sig.
func sign(args []string) error {
	fs := flag.NewFlagSet("sign", flag.ExitOnError)
	keyEnv := fs.String("key-env", "UPDATE_SIGNING_KEY", "environment variable with the private key")
	version := fs.String("version", "", "tag of the release (v1.2.3), part of what is signed")
	fs.Parse(args)
	if fs.NArg() != 1 {
		return errors.New("sign takes one file")
	}
	if *version == "" {
		return errors.New("-version is required")
	}
	value := os.Getenv(*keyEnv)
	if value == "" {
		return fmt.Errorf("%s is empty: set the secret of the release workflow", *keyEnv)
	}
	key, err := update.ParsePrivateKey(value)
	if err != nil {
		return fmt.Errorf("%s: %w", *keyEnv, err)
	}
	data, err := os.ReadFile(fs.Arg(0))
	if err != nil {
		return err
	}
	if err := os.WriteFile(fs.Arg(0)+".sig", []byte(update.Sign(key, *version, data)+"\n"), 0o644); err != nil {
		return err
	}
	fmt.Printf("signed %s for %s\n", fs.Arg(0), *version)
	return nil
}

// verify checks <file>.sig with the public keys built into the app, so a
// release signed with a key the app does not know fails before publishing.
func verify(args []string) error {
	fs := flag.NewFlagSet("verify", flag.ExitOnError)
	version := fs.String("version", "", "tag of the release (v1.2.3) the signature must be for")
	fs.Parse(args)
	if fs.NArg() != 1 {
		return errors.New("verify takes one file")
	}
	if *version == "" {
		return errors.New("-version is required")
	}
	keys, err := update.PublicKeys()
	if err != nil {
		return err
	}
	data, err := os.ReadFile(fs.Arg(0))
	if err != nil {
		return err
	}
	sig, err := os.ReadFile(fs.Arg(0) + ".sig")
	if err != nil {
		return err
	}
	if !update.Verify(keys, *version, data, string(sig)) {
		return fmt.Errorf("%s.sig is not a signature of %s by a public key built into the app", fs.Arg(0), *version)
	}
	fmt.Printf("%s.sig is valid for %s\n", fs.Arg(0), *version)
	return nil
}
