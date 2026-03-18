// Package api implements the HTTP server for the MCP Server Runtime.
// Supports the Streamable HTTP transport per MCP spec 2025-03-26 and the
// OAuth 2.1 authorization flow per RFC 9728 (Protected Resource Metadata).
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"

	"github.com/AgentHub-Studio/agenthub-mcp-server-runtime/internal/mcp"
	"github.com/AgentHub-Studio/agenthub-mcp-server-runtime/internal/oauth"
	"github.com/AgentHub-Studio/agenthub-mcp-server-runtime/internal/tenant"
)

// MessageProcessor defines the interface for processing a JSON-RPC request
// and returning a response. Implemented by MCPServer.
type MessageProcessor interface {
	HandleHTTPRequest(ctx context.Context, req *mcp.JSONRPCRequest) *mcp.JSONRPCResponse
}

// protectedResourceMetadata is the RFC 9728 Protected Resource Metadata document.
type protectedResourceMetadata struct {
	Resource             string   `json:"resource"`
	AuthorizationServers []string `json:"authorization_servers"`
	ScopesSupported      []string `json:"scopes_supported"`
	BearerMethodsSupported []string `json:"bearer_methods_supported"`
}

// HTTPServer manages the HTTP server of the MCP Server Runtime.
type HTTPServer struct {
	port           int
	processor      MessageProcessor
	validator      *oauth.Validator
	serverURL      string   // canonical public URL of this MCP server (audience)
	authServerURL  string   // authorization server base URL (for PRM document)
	requiredScopes []string // scopes every request must carry
	router         *gin.Engine
	server         *http.Server
}

// NewHTTPServer creates a new HTTP server for the MCP Server Runtime.
//   - validator: when enabled performs JWT signature validation.
//   - serverURL: canonical public URL used as the expected audience and in the PRM document.
//   - authServerURL: Keycloak/OAuth base URL listed in the PRM document.
//   - requiredScopes: scopes that every authenticated request must carry.
func NewHTTPServer(
	port int,
	processor MessageProcessor,
	validator *oauth.Validator,
	serverURL string,
	authServerURL string,
	requiredScopes []string,
) *HTTPServer {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())

	s := &HTTPServer{
		port:           port,
		processor:      processor,
		validator:      validator,
		serverURL:      strings.TrimRight(serverURL, "/"),
		authServerURL:  strings.TrimRight(authServerURL, "/"),
		requiredScopes: requiredScopes,
		router:         router,
	}

	s.registerRoutes()
	return s
}

// registerRoutes configures all HTTP routes of the server.
func (s *HTTPServer) registerRoutes() {
	// Unauthenticated discovery endpoints
	s.router.GET("/health", s.health)
	s.router.GET("/.well-known/oauth-protected-resource", s.protectedResourceMetadataHandler)

	// MCP endpoint — Streamable HTTP transport (MCP spec 2025-03-26)
	mcpGroup := s.router.Group("/mcp")
	mcpGroup.Use(s.tenantExtractorMiddleware())
	if s.validator != nil && s.validator.Enabled() {
		mcpGroup.Use(s.jwtValidationMiddleware())
	}
	mcpGroup.POST("", s.handleMCPMessage)
}

// Start starts the HTTP server on the configured port.
func (s *HTTPServer) Start() error {
	s.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", s.port),
		Handler: s.router,
	}
	log.Printf("HTTPServer: starting on port %d", s.port)
	return s.server.ListenAndServe()
}

// Stop performs a graceful shutdown of the HTTP server.
func (s *HTTPServer) Stop(ctx context.Context) error {
	if s.server == nil {
		return nil
	}
	return s.server.Shutdown(ctx)
}

// health returns the health status of the server.
// GET /health
func (s *HTTPServer) health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "UP",
		"service": "agenthub-mcp-server-runtime",
		"version": "1.0.0",
		"time":    time.Now().UTC().Format(time.RFC3339),
	})
}

// protectedResourceMetadataHandler serves the RFC 9728 Protected Resource Metadata document.
// GET /.well-known/oauth-protected-resource
func (s *HTTPServer) protectedResourceMetadataHandler(c *gin.Context) {
	resource := s.serverURL
	if resource == "" {
		// Fall back to the request's own origin
		scheme := "http"
		if c.Request.TLS != nil {
			scheme = "https"
		}
		resource = fmt.Sprintf("%s://%s", scheme, c.Request.Host)
	}

	doc := protectedResourceMetadata{
		Resource:             resource,
		BearerMethodsSupported: []string{"header"},
	}

	if s.authServerURL != "" {
		doc.AuthorizationServers = []string{s.authServerURL}
	}

	if len(s.requiredScopes) > 0 {
		doc.ScopesSupported = s.requiredScopes
	} else {
		doc.ScopesSupported = []string{"mcp:tools", "mcp:resources"}
	}

	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, doc)
}

// wwwAuthenticate builds the WWW-Authenticate header value for a 401 response.
// Points the client to the PRM document for authorization discovery.
func (s *HTTPServer) wwwAuthenticate(c *gin.Context) string {
	serverURL := s.serverURL
	if serverURL == "" {
		scheme := "http"
		if c.Request.TLS != nil {
			scheme = "https"
		}
		serverURL = fmt.Sprintf("%s://%s", scheme, c.Request.Host)
	}
	prmURL := strings.TrimRight(serverURL, "/") + "/.well-known/oauth-protected-resource"
	return fmt.Sprintf(`Bearer realm="mcp", resource_metadata="%s"`, prmURL)
}

// tenantExtractorMiddleware decodes the Bearer token (without signature validation)
// to extract the tenant ID from the "iss" claim and injects it into the context.
func (s *HTTPServer) tenantExtractorMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		var tenantID, token string

		authHeader := c.GetHeader("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			token = strings.TrimPrefix(authHeader, "Bearer ")

			parsed, _, err := jwt.NewParser().ParseUnverified(token, jwt.MapClaims{})
			if err == nil && parsed != nil {
				if claims, ok := parsed.Claims.(jwt.MapClaims); ok {
					if iss, _ := claims["iss"].(string); iss != "" {
						tenantID = tenant.ExtractFromISS(iss)
					}
				}
			}
		}

		ctx := tenant.WithTenant(c.Request.Context(), tenantID, token)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

// jwtValidationMiddleware validates the Bearer token signature, audience, and scopes.
// Returns 401 with a proper WWW-Authenticate challenge when the token is absent or invalid.
func (s *HTTPServer) jwtValidationMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := tenant.TokenFromContext(c.Request.Context())
		if token == "" {
			c.Header("WWW-Authenticate", s.wwwAuthenticate(c))
			c.JSON(http.StatusUnauthorized, gin.H{"error": "missing Bearer token"})
			c.Abort()
			return
		}

		claims, err := s.validator.Validate(token)
		if err != nil {
			log.Printf("HTTPServer: invalid JWT: %v", err)
			c.Header("WWW-Authenticate", s.wwwAuthenticate(c))
			c.JSON(http.StatusUnauthorized, gin.H{"error": fmt.Sprintf("invalid token: %v", err)})
			c.Abort()
			return
		}

		// Audience validation — token must be issued for this server.
		if s.serverURL != "" {
			if err := validateAudience(claims.Audience, s.serverURL); err != nil {
				log.Printf("HTTPServer: audience validation failed: %v", err)
				c.Header("WWW-Authenticate", s.wwwAuthenticate(c))
				c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
				c.Abort()
				return
			}
		}

		// Scope validation — token must carry all required scopes.
		if len(s.requiredScopes) > 0 {
			if err := validateScopes(claims.Scopes, s.requiredScopes); err != nil {
				log.Printf("HTTPServer: scope validation failed: %v", err)
				c.Header("WWW-Authenticate", s.wwwAuthenticate(c))
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				c.Abort()
				return
			}
		}

		c.Next()
	}
}

// validateAudience checks that at least one of the token audiences matches the server URL.
// Trailing slashes are normalised before comparison.
func validateAudience(audiences []string, serverURL string) error {
	normalised := strings.TrimRight(serverURL, "/")
	for _, aud := range audiences {
		if strings.TrimRight(aud, "/") == normalised {
			return nil
		}
	}
	return fmt.Errorf("token audience %v does not include expected resource %q", audiences, serverURL)
}

// validateScopes checks that all required scopes are present in the token.
func validateScopes(tokenScopes, required []string) error {
	scopeSet := make(map[string]struct{}, len(tokenScopes))
	for _, s := range tokenScopes {
		scopeSet[s] = struct{}{}
	}
	var missing []string
	for _, s := range required {
		if _, ok := scopeSet[s]; !ok {
			missing = append(missing, s)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required scopes: %v", missing)
	}
	return nil
}

// handleMCPMessage receives a JSON-RPC request via HTTP (Streamable HTTP transport).
// Negotiates the response format based on the Accept header:
//   - Accept: text/event-stream  → SSE response (data: <json>\n\n)
//   - Accept: application/json   → plain JSON response (default)
//
// POST /mcp
func (s *HTTPServer) handleMCPMessage(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, mcp.NewJSONRPCError(nil, mcp.ErrCodeParse, "error reading request body"))
		return
	}

	var req mcp.JSONRPCRequest
	if err := json.Unmarshal(body, &req); err != nil {
		c.JSON(http.StatusBadRequest, mcp.NewJSONRPCError(nil, mcp.ErrCodeParse, "invalid JSON in request"))
		return
	}

	response := s.processor.HandleHTTPRequest(c.Request.Context(), &req)
	if response == nil {
		c.Status(http.StatusNoContent)
		return
	}

	accept := c.GetHeader("Accept")
	if strings.Contains(accept, "text/event-stream") {
		s.writeSSEResponse(c, response)
	} else {
		c.JSON(http.StatusOK, response)
	}
}

// writeSSEResponse writes a single JSON-RPC response as an SSE event.
// Format: "data: <json>\n\n" per MCP spec 2025-03-26.
func (s *HTTPServer) writeSSEResponse(c *gin.Context, response *mcp.JSONRPCResponse) {
	responseJSON, err := json.Marshal(response)
	if err != nil {
		c.JSON(http.StatusInternalServerError, mcp.NewJSONRPCError(nil, mcp.ErrCodeInternal, "error serializing response"))
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	c.String(http.StatusOK, "data: %s\n\n", string(responseJSON))
}
