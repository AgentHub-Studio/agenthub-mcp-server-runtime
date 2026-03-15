// Pacote main é o ponto de entrada do agenthub-mcp-server-runtime.
// Inicia o servidor MCP em modo stdio (padrão) e opcionalmente um servidor HTTP.
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/AgentHub-Studio/agenthub-mcp-server-runtime/internal/api"
	"github.com/AgentHub-Studio/agenthub-mcp-server-runtime/internal/backend"
	"github.com/AgentHub-Studio/agenthub-mcp-server-runtime/internal/handler"
	"github.com/AgentHub-Studio/agenthub-mcp-server-runtime/internal/mcp"
	skillruntime "github.com/AgentHub-Studio/agenthub-mcp-server-runtime/internal/skill_runtime"
)

// Config stores settings loaded from environment variables.
type Config struct {
	// BackendURL is the base URL of the agenthub-backend (e.g. http://agenthub-backend:8080)
	BackendURL string
	// SkillRuntimeURL is the base URL of the agenthub-skill-runtime (e.g. http://agenthub-skill-runtime:8082)
	SkillRuntimeURL string
	// TenantID is the UUID of the tenant for which this server is configured
	TenantID string
	// APIToken is the Bearer token for backend authentication
	APIToken string
	// StdioMode defines whether the server should start in stdio mode (default: true)
	StdioMode bool
	// HTTPPort is the HTTP server port (0 = disabled)
	HTTPPort int
}

// loadConfig loads settings from environment variables.
func loadConfig() *Config {
	httpPort := 0
	if port := getEnv("HTTP_PORT", ""); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			httpPort = p
		}
	}

	stdioMode := true
	if mode := getEnv("STDIO_MODE", "true"); mode == "false" {
		stdioMode = false
	}

	return &Config{
		BackendURL:      getEnv("AGENTHUB_BACKEND_URL", "http://agenthub-backend:8080"),
		SkillRuntimeURL: getEnv("AGENTHUB_SKILL_RUNTIME_URL", "http://agenthub-skill-runtime:8082"),
		TenantID:        getEnv("AGENTHUB_TENANT_ID", ""),
		APIToken:        getEnv("AGENTHUB_API_TOKEN", ""),
		StdioMode:       stdioMode,
		HTTPPort:        httpPort,
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
	log.Println("Iniciando AgentHub MCP Server Runtime...")

	// Load configuration
	config := loadConfig()

	if config.TenantID == "" {
		log.Fatal("AGENTHUB_TENANT_ID é obrigatório — defina a variável de ambiente antes de iniciar")
	}

	log.Printf("Configuração: backendURL=%s, skillRuntimeURL=%s, tenantID=%s, stdioMode=%v, httpPort=%d",
		config.BackendURL, config.SkillRuntimeURL, config.TenantID, config.StdioMode, config.HTTPPort)

	// Create HTTP clients
	backendClient := backend.NewBackendClient(config.BackendURL, config.TenantID, config.APIToken)
	skillRuntimeClient := skillruntime.NewSkillRuntimeClient(config.SkillRuntimeURL, config.TenantID)

	// Create MCP handlers
	toolsHandler := handler.NewToolsHandler(backendClient, skillRuntimeClient, config.TenantID)
	resourcesHandler := handler.NewResourcesHandler(backendClient)
	promptsHandler := handler.NewPromptsHandler()

	// Create MCP server
	server := mcp.NewMCPServer(toolsHandler, resourcesHandler, promptsHandler)

	// Context with cancellation for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Channel for server errors
	errChan := make(chan error, 2)

	// Start HTTP server if configured
	if config.HTTPPort > 0 {
		httpServer := api.NewHTTPServer(config.HTTPPort, server)
		go func() {
			log.Printf("Iniciando servidor HTTP na porta %d...", config.HTTPPort)
			if err := httpServer.Start(); err != nil {
				errChan <- err
			}
		}()

		// Goroutine for graceful HTTP shutdown
		go func() {
			<-ctx.Done()
			log.Println("Parando servidor HTTP...")
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer shutdownCancel()
			if err := httpServer.Stop(shutdownCtx); err != nil {
				log.Printf("Erro ao parar servidor HTTP: %v", err)
			}
		}()
	}

	// Start MCP server in stdio mode (default)
	if config.StdioMode {
		go func() {
			log.Println("Iniciando servidor MCP em modo stdio...")
			if err := server.Start(ctx, os.Stdin, os.Stdout); err != nil {
				errChan <- err
			}
		}()
	}

	log.Println("AgentHub MCP Server Runtime iniciado com sucesso")
	if config.StdioMode {
		log.Println("Modo stdio: aguardando mensagens do cliente MCP via stdin")
	}
	if config.HTTPPort > 0 {
		log.Printf("Servidor HTTP disponível em http://localhost:%d", config.HTTPPort)
		log.Printf("Health check: GET http://localhost:%d/health", config.HTTPPort)
		log.Printf("MCP via HTTP: POST http://localhost:%d/mcp", config.HTTPPort)
	}

	// Wait for interrupt signal or error
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	select {
	case sig := <-sigChan:
		log.Printf("Sinal recebido: %v. Iniciando shutdown graceful...", sig)
	case err := <-errChan:
		log.Printf("Erro fatal em servidor: %v. Encerrando...", err)
	}

	cancel()
	log.Println("AgentHub MCP Server Runtime encerrado com sucesso")
}
