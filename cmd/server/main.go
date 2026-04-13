// Package main is the entry point for the agenthub-mcp-server-runtime.
//
// OAuth 2.1 authorization flow (RFC 9728):
//   - GET /.well-known/oauth-protected-resource — Protected Resource Metadata discovery
//   - 401 WWW-Authenticate: Bearer realm="mcp", resource_metadata="<prm-url>"
//   - JWT validation via per-tenant JWKS (template URL with {tenantId} placeholder)
//   - Audience validation against MCP_SERVER_URL
//   - Scope validation against OAUTH_REQUIRED_SCOPES
//
// Tenant identity is resolved per-request from the JWT "iss" claim — no
// static AGENTHUB_TENANT_ID environment variable is required at startup.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/AgentHub-Studio/agenthub-mcp-server-runtime/internal/api"
	"github.com/AgentHub-Studio/agenthub-mcp-server-runtime/internal/backend"
	"github.com/AgentHub-Studio/agenthub-mcp-server-runtime/internal/handler"
	"github.com/AgentHub-Studio/agenthub-mcp-server-runtime/internal/mcp"
	"github.com/AgentHub-Studio/agenthub-mcp-server-runtime/internal/oauth"
	skillruntime "github.com/AgentHub-Studio/agenthub-mcp-server-runtime/internal/skill_runtime"
)

// Config stores settings loaded from environment variables.
type Config struct {
	// BackendURL is the base URL of the agenthub-backend.
	BackendURL string
	// SkillRuntimeURL is the base URL of the agenthub-skill-runtime.
	SkillRuntimeURL string
	// APIToken is a static Bearer token for backend authentication (dev fallback).
	APIToken string
	// StdioMode defines whether the server should start in stdio mode (default: true).
	StdioMode bool
	// HTTPPort is the HTTP server port (0 = disabled).
	HTTPPort int

	// OAuth / Authorization (RFC 9728)

	// ServerURL is the canonical public URL of this MCP server.
	// Used as the expected audience in JWT validation and in the PRM document.
	// Example: https://mcp.cezar.dev
	ServerURL string
	// AuthServerURL is the OAuth/Keycloak authorization server base URL.
	// Listed in the Protected Resource Metadata document.
	// Example: https://keycloak.cezar.dev
	AuthServerURL string
	// JWKSURLTemplate is the JWKS endpoint URL, with optional {tenantId} placeholder
	// for multi-tenant Keycloak deployments.
	// Example: http://keycloak.internal:8080/realms/{tenantId}/protocol/openid-connect/certs
	JWKSURLTemplate string
	// IssuerPrefix, when non-empty, requires every JWT "iss" claim to start with this value.
	// Example: https://keycloak.cezar.dev/realms/
	IssuerPrefix string
	// RequiredScopes are the OAuth scopes that every authenticated request must carry.
	// Comma-separated. Example: mcp:tools,mcp:resources
	RequiredScopes []string
}

// loadConfig loads settings from environment variables.
func loadConfig() *Config {
	httpPort := 0
	if port := getEnv("HTTP_PORT", ""); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			httpPort = p
		}
	}

	stdioMode := getEnv("STDIO_MODE", "true") != "false"

	var requiredScopes []string
	if raw := getEnv("OAUTH_REQUIRED_SCOPES", ""); raw != "" {
		for _, s := range strings.Split(raw, ",") {
			if s = strings.TrimSpace(s); s != "" {
				requiredScopes = append(requiredScopes, s)
			}
		}
	}

	return &Config{
		BackendURL:      getEnv("AGENTHUB_BACKEND_URL", "http://agenthub.backend.internal:8081"),
		SkillRuntimeURL: getEnv("AGENTHUB_SKILL_RUNTIME_URL", "http://agenthub.skill-runtime.internal:8083"),
		APIToken:        getEnv("AGENTHUB_API_TOKEN", ""),
		StdioMode:       stdioMode,
		HTTPPort:        httpPort,
		ServerURL:       getEnv("MCP_SERVER_URL", ""),
		AuthServerURL:   getEnv("OAUTH_AUTH_SERVER_URL", ""),
		JWKSURLTemplate: getEnv("OAUTH_JWKS_URL", ""),
		IssuerPrefix:    getEnv("OAUTH_ISSUER_PREFIX", ""),
		RequiredScopes:  requiredScopes,
	}
}

// getEnv returns the value of an environment variable or the default value.
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func main() {
	printBanner()
	log.Println("Starting AgentHub MCP Server Runtime...")

	config := loadConfig()

	log.Printf("Config: backendURL=%s skillRuntimeURL=%s stdioMode=%v httpPort=%d",
		config.BackendURL, config.SkillRuntimeURL, config.StdioMode, config.HTTPPort)

	jwtValidator := oauth.NewValidator(config.JWKSURLTemplate, config.IssuerPrefix)
	if jwtValidator.Enabled() {
		log.Printf("OAuth: JWT validation enabled (jwksTemplate=%s issuerPrefix=%q)",
			config.JWKSURLTemplate, config.IssuerPrefix)
		if config.ServerURL != "" {
			log.Printf("OAuth: audience validation enabled (serverURL=%s)", config.ServerURL)
		}
		if len(config.RequiredScopes) > 0 {
			log.Printf("OAuth: required scopes: %v", config.RequiredScopes)
		}
	} else {
		log.Println("OAuth: JWT validation disabled (OAUTH_JWKS_URL not set)")
	}

	if config.AuthServerURL != "" {
		log.Printf("Authorization server: %s", config.AuthServerURL)
	}

	if config.HTTPPort > 0 {
		log.Printf("Protected Resource Metadata: http://localhost:%d/.well-known/oauth-protected-resource", config.HTTPPort)
	}

	// Build the token provider for backend calls.
	// Priority: Keycloak client credentials > static token > nil (anonymous).
	var tokenProvider backend.TokenProvider
	kcBase := getEnv("AGENTHUB_KEYCLOAK_BASE_URL", "")
	kcTenant := getEnv("AGENTHUB_TENANT_ID", "")
	kcClientID := getEnv("AGENTHUB_CLIENT_ID", "")
	kcClientSecret := getEnv("AGENTHUB_CLIENT_SECRET", "")
	if kcBase != "" && kcTenant != "" && kcClientID != "" && kcClientSecret != "" {
		tokenURL := fmt.Sprintf("%s/realms/%s/protocol/openid-connect/token", kcBase, kcTenant)
		log.Printf("Backend auth: Keycloak client credentials (clientId=%s tokenURL=%s)", kcClientID, tokenURL)
		tokenProvider = backend.NewKeycloakTokenProvider(tokenURL, kcClientID, kcClientSecret)
	} else if config.APIToken != "" {
		log.Println("Backend auth: static API token (AGENTHUB_API_TOKEN)")
		tokenProvider = backend.NewStaticTokenProvider(config.APIToken)
	} else {
		log.Println("Backend auth: none (requests may be rejected by the backend)")
	}

	backendClient := backend.NewBackendClient(config.BackendURL, tokenProvider)

	skillRuntimeClient := skillruntime.NewSkillRuntimeClient(config.SkillRuntimeURL)
	if tokenProvider != nil {
		skillRuntimeClient.WithFallbackToken(tokenProvider.Token)
	}
	if kcTenant != "" {
		skillRuntimeClient.WithFallbackTenant(kcTenant)
	}

	toolsHandler := handler.NewToolsHandler(backendClient, skillRuntimeClient)
	resourcesHandler := handler.NewResourcesHandler(backendClient)
	promptsHandler := handler.NewPromptsHandler(backendClient)

	server := mcp.NewMCPServer(toolsHandler, resourcesHandler, promptsHandler)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errChan := make(chan error, 2)

	if config.HTTPPort > 0 {
		httpServer := api.NewHTTPServer(
			config.HTTPPort,
			server,
			jwtValidator,
			config.ServerURL,
			config.AuthServerURL,
			config.RequiredScopes,
		)
		go func() {
			log.Printf("Starting HTTP server on port %d...", config.HTTPPort)
			if err := httpServer.Start(); err != nil {
				errChan <- err
			}
		}()

		go func() {
			<-ctx.Done()
			log.Println("Stopping HTTP server...")
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer shutdownCancel()
			if err := httpServer.Stop(shutdownCtx); err != nil {
				log.Printf("Error stopping HTTP server: %v", err)
			}
		}()
	}

	if config.StdioMode {
		go func() {
			log.Println("Starting MCP server in stdio mode...")
			if err := server.Start(ctx, os.Stdin, os.Stdout); err != nil {
				errChan <- err
			}
		}()
	}

	log.Println("AgentHub MCP Server Runtime started successfully")
	if config.HTTPPort > 0 {
		log.Printf("MCP endpoint:  POST http://localhost:%d/mcp", config.HTTPPort)
		log.Printf("Health check:  GET  http://localhost:%d/health", config.HTTPPort)
		log.Printf("PRM document:  GET  http://localhost:%d/.well-known/oauth-protected-resource", config.HTTPPort)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	select {
	case sig := <-sigChan:
		log.Printf("Signal received: %v. Starting graceful shutdown...", sig)
	case err := <-errChan:
		log.Printf("Fatal server error: %v. Shutting down...", err)
	}

	cancel()
	log.Println("AgentHub MCP Server Runtime stopped")
}
