package sso

import (
	"context"
	"testing"
	"time"

	"github.com/v-felipeolliveira/ssm-helper/internal/env"
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
