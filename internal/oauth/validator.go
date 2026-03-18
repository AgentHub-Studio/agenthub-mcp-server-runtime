// Package oauth implements JWT Bearer token validation for the MCP Server Runtime.
// Supports multi-tenant Keycloak deployments via a JWKS URL template containing
// the {tenantId} placeholder, derived from the token's "iss" claim on every call.
package oauth

import (
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/AgentHub-Studio/agenthub-mcp-server-runtime/internal/tenant"
)

// jwksKey represents a single key entry in a JWKS response.
type jwksKey struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Alg string `json:"alg"`
	Use string `json:"use"`
	N   string `json:"n"`
	E   string `json:"e"`
}

// jwksResponse represents the JSON body returned by a JWKS endpoint.
type jwksResponse struct {
	Keys []jwksKey `json:"keys"`
}

// tenantCache holds the RSA public keys for a single tenant.
type tenantCache struct {
	mu        sync.RWMutex
	keys      map[string]*rsa.PublicKey
	fetchedAt time.Time
}

// Claims holds the validated token claims returned by Validate.
type Claims struct {
	// TenantID extracted from the iss claim via tenant.ExtractFromISS.
	TenantID string
	// Scopes from the "scope" claim (space-separated string split into a slice).
	Scopes []string
	// Audience from the "aud" claim.
	Audience []string
	// Raw jwt.MapClaims for any additional inspection.
	Raw jwt.MapClaims
}

// Validator validates Bearer JWT tokens using per-tenant JWKS endpoints.
// The JWKS URL is built from a URL template containing {tenantId}, where the
// tenant ID is extracted from the token's "iss" claim on every validation.
// When the template has no placeholder it behaves as a single-tenant validator.
// When the template is empty the validator operates in disabled (no-op) mode.
type Validator struct {
	jwksURLTemplate string // e.g. http://keycloak:8080/realms/{tenantId}/protocol/openid-connect/certs
	issuerPrefix    string // when non-empty, the iss claim must start with this value
	cache           sync.Map // tenantID -> *tenantCache
	cacheTTL        time.Duration
}

// NewValidator creates a new JWT validator.
//   - jwksURLTemplate: JWKS endpoint URL (static or with {tenantId} placeholder).
//   - issuerPrefix: optional prefix that all valid issuers must match (e.g. https://keycloak.cezar.dev/realms/).
//
// When jwksURLTemplate is empty the validator is disabled and Validate always succeeds.
func NewValidator(jwksURLTemplate, issuerPrefix string) *Validator {
	return &Validator{
		jwksURLTemplate: jwksURLTemplate,
		issuerPrefix:    issuerPrefix,
		cacheTTL:        5 * time.Minute,
	}
}

// Enabled reports whether JWT validation is active.
func (v *Validator) Enabled() bool {
	return v.jwksURLTemplate != ""
}

// Validate parses and validates a raw JWT token string.
// Returns Claims on success; returns an error if the token is invalid.
// When disabled, always returns empty Claims with no error.
func (v *Validator) Validate(tokenStr string) (*Claims, error) {
	if !v.Enabled() {
		return &Claims{}, nil
	}

	// Step 1: decode without signature verification to read iss → tenant ID.
	parsed, _, err := jwt.NewParser().ParseUnverified(tokenStr, jwt.MapClaims{})
	if err != nil {
		return nil, fmt.Errorf("failed to decode token: %w", err)
	}

	rawClaims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.New("invalid token claims")
	}

	iss, _ := rawClaims["iss"].(string)
	if iss == "" {
		return nil, errors.New("missing iss claim")
	}

	if v.issuerPrefix != "" && !strings.HasPrefix(iss, v.issuerPrefix) {
		return nil, fmt.Errorf("issuer %q does not match expected prefix %q", iss, v.issuerPrefix)
	}

	tenantID := tenant.ExtractFromISS(iss)
	jwksURL := v.buildJWKSURL(tenantID)

	// Step 2: validate signature using the tenant's JWKS.
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		kid, _ := t.Header["kid"].(string)
		return v.getKey(tenantID, kid, jwksURL)
	}, jwt.WithValidMethods([]string{"RS256", "RS384", "RS512"}))

	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token claims")
	}

	return &Claims{
		TenantID: tenantID,
		Scopes:   extractScopes(claims),
		Audience: extractAudience(claims),
		Raw:      claims,
	}, nil
}

// buildJWKSURL constructs the JWKS endpoint URL for a given tenant.
func (v *Validator) buildJWKSURL(tenantID string) string {
	if tenantID == "" {
		return v.jwksURLTemplate
	}
	return strings.ReplaceAll(v.jwksURLTemplate, "{tenantId}", tenantID)
}

// getKey returns the RSA public key for the given tenant and key ID.
func (v *Validator) getKey(tenantID, kid, jwksURL string) (*rsa.PublicKey, error) {
	tc := v.getOrCreateCache(tenantID)

	tc.mu.RLock()
	if time.Since(tc.fetchedAt) < v.cacheTTL {
		key := lookupKey(tc.keys, kid)
		tc.mu.RUnlock()
		if key != nil {
			return key, nil
		}
	} else {
		tc.mu.RUnlock()
	}

	if err := v.fetchJWKS(tc, jwksURL); err != nil {
		return nil, fmt.Errorf("error fetching JWKS for tenant %q: %w", tenantID, err)
	}

	tc.mu.RLock()
	defer tc.mu.RUnlock()

	key := lookupKey(tc.keys, kid)
	if key == nil {
		if kid == "" {
			return nil, errors.New("no key found in JWKS")
		}
		return nil, fmt.Errorf("key kid=%q not found in JWKS", kid)
	}
	return key, nil
}

// getOrCreateCache returns the tenantCache for the given tenant, creating one if absent.
func (v *Validator) getOrCreateCache(tenantID string) *tenantCache {
	actual, _ := v.cache.LoadOrStore(tenantID, &tenantCache{
		keys: make(map[string]*rsa.PublicKey),
	})
	return actual.(*tenantCache)
}

// lookupKey returns the RSA key for kid, falling back to the first available key when kid is empty.
// Must be called with the tenantCache mu held for reading.
func lookupKey(keys map[string]*rsa.PublicKey, kid string) *rsa.PublicKey {
	if kid == "" {
		for _, k := range keys {
			return k
		}
		return nil
	}
	return keys[kid]
}

// fetchJWKS downloads the JWKS from the given URL and updates the cache.
func (v *Validator) fetchJWKS(tc *tenantCache, jwksURL string) error {
	resp, err := http.Get(jwksURL) //nolint:noctx
	if err != nil {
		return fmt.Errorf("HTTP error fetching JWKS from %s: %w", jwksURL, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error reading JWKS response body: %w", err)
	}

	var jwks jwksResponse
	if err := json.Unmarshal(body, &jwks); err != nil {
		return fmt.Errorf("error parsing JWKS: %w", err)
	}

	keys := make(map[string]*rsa.PublicKey, len(jwks.Keys))
	for _, k := range jwks.Keys {
		if k.Kty != "RSA" || (k.Use != "sig" && k.Use != "") {
			continue
		}
		pubKey, err := parseRSAPublicKey(k.N, k.E)
		if err != nil {
			continue
		}
		keys[k.Kid] = pubKey
	}

	tc.mu.Lock()
	tc.keys = keys
	tc.fetchedAt = time.Now()
	tc.mu.Unlock()

	return nil
}

// parseRSAPublicKey constructs an *rsa.PublicKey from base64url-encoded N and E values.
func parseRSAPublicKey(nStr, eStr string) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(nStr)
	if err != nil {
		return nil, fmt.Errorf("error decoding N: %w", err)
	}

	eBytes, err := base64.RawURLEncoding.DecodeString(eStr)
	if err != nil {
		return nil, fmt.Errorf("error decoding E: %w", err)
	}

	n := new(big.Int).SetBytes(nBytes)

	var e int
	for _, b := range eBytes {
		e = e<<8 + int(b)
	}

	return &rsa.PublicKey{N: n, E: e}, nil
}

// extractScopes parses the "scope" claim (space-separated string) into a slice.
func extractScopes(claims jwt.MapClaims) []string {
	scope, _ := claims["scope"].(string)
	if scope == "" {
		return nil
	}
	return strings.Fields(scope)
}

// extractAudience returns the "aud" claim as a string slice (handles both string and []interface{}).
func extractAudience(claims jwt.MapClaims) []string {
	switch v := claims["aud"].(type) {
	case string:
		return []string{v}
	case []interface{}:
		out := make([]string, 0, len(v))
		for _, a := range v {
			if s, ok := a.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}
