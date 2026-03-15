package oauth

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// generateTestKey creates a new RSA key pair for use in tests.
func generateTestKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("erro ao gerar chave RSA: %v", err)
	}
	return key
}

// buildJWKSServer starts an httptest server that returns a JWKS with the given key.
func buildJWKSServer(t *testing.T, key *rsa.PrivateKey, kid string) *httptest.Server {
	t.Helper()

	nBytes := key.PublicKey.N.Bytes()
	eBytes := big.NewInt(int64(key.PublicKey.E)).Bytes()

	jwks := map[string]interface{}{
		"keys": []map[string]interface{}{
			{
				"kty": "RSA",
				"use": "sig",
				"kid": kid,
				"alg": "RS256",
				"n":   base64.RawURLEncoding.EncodeToString(nBytes),
				"e":   base64.RawURLEncoding.EncodeToString(eBytes),
			},
		},
	}

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jwks)
	}))
}

// signToken creates a signed JWT token using the given key.
func signToken(t *testing.T, key *rsa.PrivateKey, kid, issuer string, expiry time.Time) string {
	t.Helper()

	claims := jwt.MapClaims{
		"sub": "test-subject",
		"iss": issuer,
		"exp": expiry.Unix(),
		"iat": time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = kid

	signed, err := token.SignedString(key)
	if err != nil {
		t.Fatalf("erro ao assinar token: %v", err)
	}
	return signed
}

func TestValidator_DeveAceitarTokenValido(t *testing.T) {
	key := generateTestKey(t)
	srv := buildJWKSServer(t, key, "key-1")
	defer srv.Close()

	issuer := "https://keycloak.test/realms/agenthub"
	tokenStr := signToken(t, key, "key-1", issuer, time.Now().Add(time.Hour))

	v := NewValidator(srv.URL, issuer)
	claims, err := v.Validate(tokenStr)

	if err != nil {
		t.Fatalf("esperava token válido, obteve erro: %v", err)
	}
	if claims["sub"] != "test-subject" {
		t.Errorf("claim 'sub' incorreto: %v", claims["sub"])
	}
}

func TestValidator_DeveRejeitarTokenExpirado(t *testing.T) {
	key := generateTestKey(t)
	srv := buildJWKSServer(t, key, "key-1")
	defer srv.Close()

	issuer := "https://keycloak.test/realms/agenthub"
	tokenStr := signToken(t, key, "key-1", issuer, time.Now().Add(-time.Hour))

	v := NewValidator(srv.URL, issuer)
	_, err := v.Validate(tokenStr)

	if err == nil {
		t.Fatal("esperava erro para token expirado")
	}
}

func TestValidator_DeveRejeitarIssuerErrado(t *testing.T) {
	key := generateTestKey(t)
	srv := buildJWKSServer(t, key, "key-1")
	defer srv.Close()

	tokenStr := signToken(t, key, "key-1", "https://outro-issuer.test", time.Now().Add(time.Hour))

	v := NewValidator(srv.URL, "https://keycloak.test/realms/agenthub")
	_, err := v.Validate(tokenStr)

	if err == nil {
		t.Fatal("esperava erro para issuer inválido")
	}
}

func TestValidator_DevePassarSemValidacaoQuandoDesabilitado(t *testing.T) {
	v := NewValidator("", "")

	if v.Enabled() {
		t.Fatal("validador deveria estar desabilitado quando jwksURL está vazio")
	}

	claims, err := v.Validate("qualquer-token")
	if err != nil {
		t.Fatalf("validador desabilitado não deveria retornar erro: %v", err)
	}
	if len(claims) != 0 {
		t.Errorf("claims deveriam estar vazios no modo desabilitado")
	}
}

func TestValidator_DeveRejeitarTokenComAssinaturaInvalida(t *testing.T) {
	key1 := generateTestKey(t)
	key2 := generateTestKey(t)

	// JWKS has key1, but token is signed with key2
	srv := buildJWKSServer(t, key1, "key-1")
	defer srv.Close()

	tokenStr := signToken(t, key2, "key-1", "https://keycloak.test/realms/agenthub", time.Now().Add(time.Hour))

	v := NewValidator(srv.URL, "")
	_, err := v.Validate(tokenStr)

	if err == nil {
		t.Fatal("esperava erro para token com assinatura inválida")
	}
}
