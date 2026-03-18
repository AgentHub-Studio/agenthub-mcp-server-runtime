// Package tenant provides context helpers for propagating tenant identity
// through the MCP server request pipeline.
//
// The tenant ID is extracted from the JWT "iss" claim on every HTTP request
// and stored in the context so backend/skill-runtime clients can use it
// without depending on a startup environment variable.
package tenant

import (
	"context"
	"regexp"
)

type contextKey struct{ name string }

var (
	tenantIDKey    = contextKey{"tenantID"}
	bearerTokenKey = contextKey{"bearerToken"}
	realmRegexp    = regexp.MustCompile(`/realms/([^/]+)`)
)

// WithTenant returns a new context carrying the tenant ID and Bearer token.
func WithTenant(ctx context.Context, tenantID, token string) context.Context {
	ctx = context.WithValue(ctx, tenantIDKey, tenantID)
	return context.WithValue(ctx, bearerTokenKey, token)
}

// IDFromContext returns the tenant ID stored in the context, or empty string.
func IDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(tenantIDKey).(string)
	return v
}

// TokenFromContext returns the Bearer token stored in the context, or empty string.
func TokenFromContext(ctx context.Context) string {
	v, _ := ctx.Value(bearerTokenKey).(string)
	return v
}

// ExtractFromISS extracts the tenant ID (realm name) from a Keycloak issuer URL.
// Expected format: http(s)://host/realms/{tenantId}
// Returns empty string when the pattern is not found.
func ExtractFromISS(iss string) string {
	m := realmRegexp.FindStringSubmatch(iss)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}
