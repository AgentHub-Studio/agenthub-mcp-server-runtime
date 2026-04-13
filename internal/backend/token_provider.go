// Package backend — token provider for authenticating with the agenthub-api.
package backend

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// TokenProvider issues Bearer tokens for backend HTTP calls.
type TokenProvider interface {
	// Token returns a valid Bearer token, refreshing it if necessary.
	Token() (string, error)
}

// StaticTokenProvider returns the same token on every call.
type StaticTokenProvider struct{ token string }

// NewStaticTokenProvider wraps a fixed Bearer token.
func NewStaticTokenProvider(token string) *StaticTokenProvider {
	return &StaticTokenProvider{token: token}
}

// Token implements TokenProvider.
func (p *StaticTokenProvider) Token() (string, error) { return p.token, nil }

// tokenResponse is the OAuth2 /token endpoint response.
type tokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"` // seconds
}

// KeycloakTokenProvider fetches tokens via Keycloak Client Credentials grant
// and caches them until they are about to expire.
type KeycloakTokenProvider struct {
	tokenURL     string
	clientID     string
	clientSecret string

	mu      sync.Mutex
	cached  string
	expiry  time.Time
	http    *http.Client
}

// NewKeycloakTokenProvider creates a token provider for the given Keycloak realm.
// tokenURL format: http://keycloak:8080/realms/{tenant}/protocol/openid-connect/token
func NewKeycloakTokenProvider(tokenURL, clientID, clientSecret string) *KeycloakTokenProvider {
	return &KeycloakTokenProvider{
		tokenURL:     tokenURL,
		clientID:     clientID,
		clientSecret: clientSecret,
		http:         &http.Client{Timeout: 10 * time.Second},
	}
}

// Token returns a valid Bearer token, refreshing via Keycloak when needed.
func (p *KeycloakTokenProvider) Token() (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Return cached token if still valid with a 30s buffer.
	if p.cached != "" && time.Now().Add(30*time.Second).Before(p.expiry) {
		return p.cached, nil
	}

	tok, exp, err := p.fetch()
	if err != nil {
		return "", err
	}
	p.cached = tok
	p.expiry = exp
	return tok, nil
}

func (p *KeycloakTokenProvider) fetch() (string, time.Time, error) {
	body := url.Values{}
	body.Set("grant_type", "client_credentials")
	body.Set("client_id", p.clientID)
	body.Set("client_secret", p.clientSecret)

	req, err := http.NewRequest(http.MethodPost, p.tokenURL, strings.NewReader(body.Encode()))
	if err != nil {
		return "", time.Time{}, fmt.Errorf("keycloak token: new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := p.http.Do(req)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("keycloak token: request: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", time.Time{}, fmt.Errorf("keycloak token: status %d: %s", resp.StatusCode, string(raw))
	}

	var tr tokenResponse
	if err := json.Unmarshal(raw, &tr); err != nil {
		return "", time.Time{}, fmt.Errorf("keycloak token: decode: %w", err)
	}
	if tr.AccessToken == "" {
		return "", time.Time{}, fmt.Errorf("keycloak token: empty access_token")
	}

	exp := time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second)
	return tr.AccessToken, exp, nil
}
