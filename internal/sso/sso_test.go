package sso

import (
	"context"
	"testing"
	"time"

	"github.com/felipeolliveira/ssm-helper/internal/env"
	"github.com/felipeolliveira/ssm-helper/internal/msg"
)

func TestStatusUsesCachedToken(t *testing.T) {
	m := NewManager(t.TempDir(), nil)
	p := env.Profile{Name: "qa", SSOURL: "https://example.awsapps.com/start", SSORegion: "us-east-1"}

	if st := m.Status(context.Background(), p); st.LoggedIn {
		t.Fatal("logged in without a cached token")
	}

	expires := time.Now().Add(time.Hour).Truncate(time.Second)
	if err := m.writeCache(&cachedToken{StartURL: p.SSOURL, Region: p.SSORegion, AccessToken: "t", ExpiresAt: expires}); err != nil {
		t.Fatal(err)
	}
	st := m.Status(context.Background(), p)
	if !st.LoggedIn || !st.ExpiresAt.Equal(expires) {
		t.Fatalf("status = %+v", st)
	}

	// Another profile with the same start URL shares the login
	other := env.Profile{Name: "dev", SSOURL: p.SSOURL, SSORegion: p.SSORegion}
	if !m.Status(context.Background(), other).LoggedIn {
		t.Error("profile with the same start URL is not logged in")
	}
}

func TestExpiredTokenWithoutRefreshRequiresLogin(t *testing.T) {
	m := NewManager(t.TempDir(), nil)
	p := env.Profile{Name: "qa", SSOURL: "https://example.awsapps.com/start", SSORegion: "us-east-1"}
	if err := m.writeCache(&cachedToken{StartURL: p.SSOURL, Region: p.SSORegion, AccessToken: "t", ExpiresAt: time.Now().Add(time.Minute)}); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Credentials(context.Background(), p); err != ErrLoginRequired {
		t.Fatalf("err = %v", err)
	}
}

// A region that is not an AWS region name is refused before any request, so
// the SSO token is never sent to a host built from it.
func TestCredentialsRefuseInvalidRegion(t *testing.T) {
	m := NewManager(t.TempDir(), nil)
	p := env.Profile{Name: "qa", SSOURL: "https://example.awsapps.com/start", SSORegion: "evil.example.com/x"}
	if err := m.writeCache(&cachedToken{StartURL: p.SSOURL, Region: "us-east-1", AccessToken: "t", ExpiresAt: time.Now().Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Credentials(context.Background(), p); msg.From(err).Code != msg.RegionInvalid {
		t.Fatalf("Credentials err = %v", err)
	}
	if err := m.Login(context.Background(), p, nil); msg.From(err).Code != msg.RegionInvalid {
		t.Fatalf("Login err = %v", err)
	}
}
