// Package sso logs in to IAM Identity Center with the device authorization
// flow and turns the cached token into role credentials. Nothing from ~/.aws
// is read: profiles come only from the environment files.
package sso

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sso"
	"github.com/aws/aws-sdk-go-v2/service/ssooidc"
	ssooidctypes "github.com/aws/aws-sdk-go-v2/service/ssooidc/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"

	"github.com/v-felipeolliveira/ssm-helper/internal/env"
	"github.com/v-felipeolliveira/ssm-helper/internal/msg"
)

// ErrLoginRequired means there is no valid SSO token for the profile's start URL.
var ErrLoginRequired = msg.Err(msg.SSOLoginExpired)

const (
	clientName      = "ssm-helper"
	clientType      = "public"
	accountScope    = "sso:account:access"
	deviceGrantType = "urn:ietf:params:oauth:grant-type:device_code"
	refreshGrant    = "refresh_token"
	// Tokens and credentials closer than this to expiring are treated as expired.
	expiryMargin = 5 * time.Minute
)

// DeviceCode is shown to the user while the browser login is pending.
type DeviceCode struct {
	UserCode                string `json:"userCode"`
	VerificationURI         string `json:"verificationUri"`
	VerificationURIComplete string `json:"verificationUriComplete"`
}

// Status describes the SSO session of a profile.
type Status struct {
	LoggedIn  bool      `json:"loggedIn"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type cachedToken struct {
	StartURL              string    `json:"startUrl"`
	Region                string    `json:"region"`
	AccessToken           string    `json:"accessToken"`
	ExpiresAt             time.Time `json:"expiresAt"`
	RefreshToken          string    `json:"refreshToken,omitempty"`
	ClientID              string    `json:"clientId"`
	ClientSecret          string    `json:"clientSecret"`
	ClientSecretExpiresAt time.Time `json:"clientSecretExpiresAt"`
}

// Manager keeps one token per start URL, so profiles sharing AWS_SSO_URL share the login.
type Manager struct {
	cacheDir string
	openURL  func(string)

	mu          sync.Mutex
	credentials map[string]aws.Credentials // by profile name
	logins      map[string]*sync.Mutex     // by start URL
}

// NewManager stores tokens in cacheDir; openURL opens the login page in the browser.
func NewManager(cacheDir string, openURL func(string)) *Manager {
	return &Manager{
		cacheDir:    cacheDir,
		openURL:     openURL,
		credentials: map[string]aws.Credentials{},
		logins:      map[string]*sync.Mutex{},
	}
}

// Status reports whether the profile has a usable SSO token.
func (m *Manager) Status(ctx context.Context, p env.Profile) Status {
	token, err := m.token(ctx, p)
	if err != nil {
		return Status{}
	}
	return Status{LoggedIn: true, ExpiresAt: token.ExpiresAt}
}

// Login runs the device authorization flow, calling onCode with the code to confirm.
func (m *Manager) Login(ctx context.Context, p env.Profile, onCode func(DeviceCode)) error {
	lock := m.loginLock(p.SSOURL)
	lock.Lock()
	defer lock.Unlock()

	client := ssooidc.NewFromConfig(aws.Config{Region: p.SSORegion})
	cached, _ := m.readCache(p.SSOURL)

	clientID, clientSecret := "", ""
	if cached != nil && cached.Region == p.SSORegion && time.Now().Add(expiryMargin).Before(cached.ClientSecretExpiresAt) {
		clientID, clientSecret = cached.ClientID, cached.ClientSecret
	}
	var clientSecretExpiresAt time.Time
	if clientID == "" {
		reg, err := client.RegisterClient(ctx, &ssooidc.RegisterClientInput{
			ClientName: aws.String(clientName),
			ClientType: aws.String(clientType),
			Scopes:     []string{accountScope},
		})
		if err != nil {
			return msg.Wrap(msg.SSORegisterFailed, err)
		}
		clientID, clientSecret = aws.ToString(reg.ClientId), aws.ToString(reg.ClientSecret)
		clientSecretExpiresAt = time.Unix(reg.ClientSecretExpiresAt, 0)
	} else {
		clientSecretExpiresAt = cached.ClientSecretExpiresAt
	}

	auth, err := client.StartDeviceAuthorization(ctx, &ssooidc.StartDeviceAuthorizationInput{
		ClientId:     aws.String(clientID),
		ClientSecret: aws.String(clientSecret),
		StartUrl:     aws.String(p.SSOURL),
	})
	if err != nil {
		return msg.Wrap(msg.SSOStartFailed, err)
	}

	code := DeviceCode{
		UserCode:                aws.ToString(auth.UserCode),
		VerificationURI:         aws.ToString(auth.VerificationUri),
		VerificationURIComplete: aws.ToString(auth.VerificationUriComplete),
	}
	if onCode != nil {
		onCode(code)
	}
	if m.openURL != nil {
		m.openURL(code.VerificationURIComplete)
	}

	interval := time.Duration(auth.Interval) * time.Second
	if interval <= 0 {
		interval = 5 * time.Second
	}
	deadline := time.Now().Add(time.Duration(auth.ExpiresIn) * time.Second)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}
		if time.Now().After(deadline) {
			return msg.Err(msg.SSOCodeExpired)
		}

		out, err := client.CreateToken(ctx, &ssooidc.CreateTokenInput{
			ClientId:     aws.String(clientID),
			ClientSecret: aws.String(clientSecret),
			DeviceCode:   auth.DeviceCode,
			GrantType:    aws.String(deviceGrantType),
		})
		var pending *ssooidctypes.AuthorizationPendingException
		var slowDown *ssooidctypes.SlowDownException
		switch {
		case errors.As(err, &pending):
			continue
		case errors.As(err, &slowDown):
			interval += 5 * time.Second
			continue
		case err != nil:
			return msg.Wrap(msg.SSOLoginFailed, err)
		}

		token := &cachedToken{
			StartURL:              p.SSOURL,
			Region:                p.SSORegion,
			AccessToken:           aws.ToString(out.AccessToken),
			ExpiresAt:             time.Now().Add(time.Duration(out.ExpiresIn) * time.Second),
			RefreshToken:          aws.ToString(out.RefreshToken),
			ClientID:              clientID,
			ClientSecret:          clientSecret,
			ClientSecretExpiresAt: clientSecretExpiresAt,
		}
		m.clearCredentials()
		return m.writeCache(token)
	}
}

// Credentials returns role credentials for the profile, or ErrLoginRequired.
func (m *Manager) Credentials(ctx context.Context, p env.Profile) (aws.Credentials, error) {
	m.mu.Lock()
	creds, ok := m.credentials[p.Name]
	m.mu.Unlock()
	if ok && time.Now().Add(expiryMargin).Before(creds.Expires) {
		return creds, nil
	}

	token, err := m.token(ctx, p)
	if err != nil {
		return aws.Credentials{}, err
	}

	client := sso.NewFromConfig(aws.Config{Region: p.SSORegion})
	out, err := client.GetRoleCredentials(ctx, &sso.GetRoleCredentialsInput{
		AccessToken: aws.String(token.AccessToken),
		AccountId:   aws.String(p.AccountID),
		RoleName:    aws.String(p.RoleName),
	})
	if err != nil {
		return aws.Credentials{}, msg.Wrap(msg.SSORoleCredentialsFailed, err)
	}
	rc := out.RoleCredentials
	creds = aws.Credentials{
		AccessKeyID:     aws.ToString(rc.AccessKeyId),
		SecretAccessKey: aws.ToString(rc.SecretAccessKey),
		SessionToken:    aws.ToString(rc.SessionToken),
		Source:          "ssm-helper-sso",
		CanExpire:       true,
		Expires:         time.UnixMilli(rc.Expiration),
	}

	m.mu.Lock()
	m.credentials[p.Name] = creds
	m.mu.Unlock()
	return creds, nil
}

// Verify confirms the role works by calling STS GetCallerIdentity.
func (m *Manager) Verify(ctx context.Context, p env.Profile) error {
	creds, err := m.Credentials(ctx, p)
	if err != nil {
		return err
	}
	client := sts.NewFromConfig(AWSConfig(creds, p.SSORegion))
	if _, err := client.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{}); err != nil {
		return msg.Wrap(msg.SSOVerifyFailed, err, "role", p.RoleName)
	}
	return nil
}

// AWSConfig builds an SDK config from static credentials only.
func AWSConfig(creds aws.Credentials, region string) aws.Config {
	return aws.Config{
		Region: region,
		Credentials: aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) {
			return creds, nil
		}),
	}
}

// token returns a valid cached token, refreshing it when possible.
func (m *Manager) token(ctx context.Context, p env.Profile) (*cachedToken, error) {
	cached, err := m.readCache(p.SSOURL)
	if err != nil || cached == nil {
		return nil, ErrLoginRequired
	}
	if time.Now().Add(expiryMargin).Before(cached.ExpiresAt) {
		return cached, nil
	}
	if cached.RefreshToken == "" || !time.Now().Before(cached.ClientSecretExpiresAt) {
		return nil, ErrLoginRequired
	}

	client := ssooidc.NewFromConfig(aws.Config{Region: cached.Region})
	out, err := client.CreateToken(ctx, &ssooidc.CreateTokenInput{
		ClientId:     aws.String(cached.ClientID),
		ClientSecret: aws.String(cached.ClientSecret),
		GrantType:    aws.String(refreshGrant),
		RefreshToken: aws.String(cached.RefreshToken),
	})
	if err != nil {
		return nil, ErrLoginRequired
	}
	cached.AccessToken = aws.ToString(out.AccessToken)
	cached.ExpiresAt = time.Now().Add(time.Duration(out.ExpiresIn) * time.Second)
	if rt := aws.ToString(out.RefreshToken); rt != "" {
		cached.RefreshToken = rt
	}
	if err := m.writeCache(cached); err != nil {
		return nil, err
	}
	return cached, nil
}

func (m *Manager) loginLock(startURL string) *sync.Mutex {
	m.mu.Lock()
	defer m.mu.Unlock()
	lock, ok := m.logins[startURL]
	if !ok {
		lock = &sync.Mutex{}
		m.logins[startURL] = lock
	}
	return lock
}

// clearCredentials drops cached role credentials after a new login; they are
// cheap to fetch again.
func (m *Manager) clearCredentials() {
	m.mu.Lock()
	m.credentials = map[string]aws.Credentials{}
	m.mu.Unlock()
}

func (m *Manager) cachePath(startURL string) string {
	sum := sha1.Sum([]byte(startURL))
	return filepath.Join(m.cacheDir, hex.EncodeToString(sum[:])+".json")
}

func (m *Manager) readCache(startURL string) (*cachedToken, error) {
	data, err := os.ReadFile(m.cachePath(startURL))
	if err != nil {
		return nil, err
	}
	var token cachedToken
	if err := json.Unmarshal(data, &token); err != nil {
		return nil, err
	}
	if token.StartURL != startURL {
		return nil, nil
	}
	return &token, nil
}

func (m *Manager) writeCache(token *cachedToken) error {
	if err := os.MkdirAll(m.cacheDir, 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(token)
	if err != nil {
		return err
	}
	tmp := m.cachePath(token.StartURL) + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, m.cachePath(token.StartURL))
}
