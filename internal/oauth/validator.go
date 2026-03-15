// Pacote oauth implementa validação de tokens JWT para o servidor MCP.
// Suporta validação via JWKS (JSON Web Key Set), compatível com Keycloak.
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
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
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

// Validator validates Bearer JWT tokens using a remote JWKS endpoint.
// When jwksURL is empty, validation is disabled and all requests pass through.
type Validator struct {
	jwksURL   string
	issuer    string
	mu        sync.RWMutex
	keys      map[string]*rsa.PublicKey
	fetchedAt time.Time
	cacheTTL  time.Duration
}

// NewValidator creates a new JWT validator.
// When jwksURL is empty the validator operates in disabled mode (no-op).
func NewValidator(jwksURL, issuer string) *Validator {
	return &Validator{
		jwksURL:  jwksURL,
		issuer:   issuer,
		keys:     make(map[string]*rsa.PublicKey),
		cacheTTL: 5 * time.Minute,
	}
}

// Enabled reports whether JWT validation is active.
func (v *Validator) Enabled() bool {
	return v.jwksURL != ""
}

// Validate parses and validates a raw JWT token string.
// Returns the token claims on success, or an error if the token is invalid.
// When disabled, always returns empty claims with no error.
func (v *Validator) Validate(tokenStr string) (jwt.MapClaims, error) {
	if !v.Enabled() {
		return jwt.MapClaims{}, nil
	}

	token, err := jwt.Parse(tokenStr, v.keyFunc,
		jwt.WithValidMethods([]string{"RS256", "RS384", "RS512"}),
	)
	if err != nil {
		return nil, fmt.Errorf("token inválido: %w", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, errors.New("token inválido: claims não reconhecidos")
	}

	if v.issuer != "" {
		iss, _ := claims["iss"].(string)
		if iss != v.issuer {
			return nil, fmt.Errorf("issuer inválido: esperado %q, recebido %q", v.issuer, iss)
		}
	}

	return claims, nil
}

// keyFunc is the jwt.Keyfunc implementation that resolves the signing key
// from the JWKS cache using the token's "kid" header.
func (v *Validator) keyFunc(token *jwt.Token) (interface{}, error) {
	kid, _ := token.Header["kid"].(string)
	return v.getKey(kid)
}

// getKey returns the RSA public key for the given key ID.
// Refreshes the JWKS cache when expired or when the key is not found.
func (v *Validator) getKey(kid string) (*rsa.PublicKey, error) {
	v.mu.RLock()
	cacheValid := time.Since(v.fetchedAt) < v.cacheTTL
	if cacheValid {
		key := v.lookupKey(kid)
		v.mu.RUnlock()
		if key != nil {
			return key, nil
		}
	} else {
		v.mu.RUnlock()
	}

	if err := v.fetchJWKS(); err != nil {
		return nil, fmt.Errorf("erro ao buscar JWKS: %w", err)
	}

	v.mu.RLock()
	defer v.mu.RUnlock()

	key := v.lookupKey(kid)
	if key == nil {
		if kid == "" {
			return nil, errors.New("nenhuma chave encontrada no JWKS")
		}
		return nil, fmt.Errorf("chave kid=%q não encontrada no JWKS", kid)
	}
	return key, nil
}

// lookupKey returns the key for the given kid, or the first available key when kid is empty.
// Must be called with v.mu held for reading.
func (v *Validator) lookupKey(kid string) *rsa.PublicKey {
	if kid == "" {
		for _, k := range v.keys {
			return k
		}
		return nil
	}
	return v.keys[kid]
}

// fetchJWKS downloads the JWKS from the configured URL and updates the key cache.
func (v *Validator) fetchJWKS() error {
	resp, err := http.Get(v.jwksURL) //nolint:noctx
	if err != nil {
		return fmt.Errorf("erro HTTP ao buscar JWKS de %s: %w", v.jwksURL, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("erro ao ler corpo do JWKS: %w", err)
	}

	var jwks jwksResponse
	if err := json.Unmarshal(body, &jwks); err != nil {
		return fmt.Errorf("erro ao parsear JWKS: %w", err)
	}

	keys := make(map[string]*rsa.PublicKey, len(jwks.Keys))
	for _, k := range jwks.Keys {
		if k.Kty != "RSA" || k.Use != "sig" && k.Use != "" {
			continue
		}
		pubKey, err := parseRSAPublicKey(k.N, k.E)
		if err != nil {
			continue
		}
		keys[k.Kid] = pubKey
	}

	v.mu.Lock()
	v.keys = keys
	v.fetchedAt = time.Now()
	v.mu.Unlock()

	return nil
}

// parseRSAPublicKey constructs an *rsa.PublicKey from base64url-encoded N and E values.
func parseRSAPublicKey(nStr, eStr string) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(nStr)
	if err != nil {
		return nil, fmt.Errorf("erro ao decodificar N: %w", err)
	}

	eBytes, err := base64.RawURLEncoding.DecodeString(eStr)
	if err != nil {
		return nil, fmt.Errorf("erro ao decodificar E: %w", err)
	}

	n := new(big.Int).SetBytes(nBytes)

	var e int
	for _, b := range eBytes {
		e = e<<8 + int(b)
	}

	return &rsa.PublicKey{N: n, E: e}, nil
}
