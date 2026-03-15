// Pacote api implementa o servidor HTTP do MCP Server Runtime.
// Suporta o transporte Streamable HTTP conforme spec MCP 2025-03-26:
// um único endpoint POST /mcp que negocia a resposta via Accept header
// (application/json para JSON síncrono, text/event-stream para SSE).
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

	"github.com/AgentHub-Studio/agenthub-mcp-server-runtime/internal/mcp"
	"github.com/AgentHub-Studio/agenthub-mcp-server-runtime/internal/oauth"
)

// MessageProcessor defines the interface for processing a JSON-RPC request
// and returning a response. Implemented by MCPServer.
type MessageProcessor interface {
	HandleHTTPRequest(ctx context.Context, req *mcp.JSONRPCRequest) *mcp.JSONRPCResponse
}

// HTTPServer manages the HTTP server of the MCP Server Runtime.
type HTTPServer struct {
	port      int
	processor MessageProcessor
	validator *oauth.Validator
	router    *gin.Engine
	server    *http.Server
}

// NewHTTPServer creates a new HTTP server for the MCP Server Runtime.
// validator may be nil; when provided and enabled, JWT validation is applied to /mcp.
func NewHTTPServer(port int, processor MessageProcessor, validator *oauth.Validator) *HTTPServer {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())

	s := &HTTPServer{
		port:      port,
		processor: processor,
		validator: validator,
		router:    router,
	}

	s.registerRoutes()
	return s
}

// registerRoutes configures all HTTP routes of the server.
func (s *HTTPServer) registerRoutes() {
	s.router.GET("/health", s.health)

	// Streamable HTTP transport (MCP spec 2025-03-26):
	// single POST /mcp endpoint replaces the previous POST /mcp + GET /mcp/sse pair.
	mcpGroup := s.router.Group("/mcp")
	if s.validator != nil && s.validator.Enabled() {
		mcpGroup.Use(s.jwtMiddleware())
	}
	mcpGroup.POST("", s.handleMCPMessage)
}

// Start starts the HTTP server on the configured port.
func (s *HTTPServer) Start() error {
	s.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", s.port),
		Handler: s.router,
	}

	log.Printf("HTTPServer: iniciando na porta %d (Streamable HTTP, MCP spec 2025-03-26)", s.port)
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
		"servico": "agenthub-mcp-server-runtime",
		"versao":  "1.0.0",
		"agora":   time.Now().UTC().Format(time.RFC3339),
	})
}

// jwtMiddleware validates the Bearer token in the Authorization header.
// Returns 401 when the token is absent or invalid.
func (s *HTTPServer) jwtMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "token de autenticação ausente ou malformado"})
			c.Abort()
			return
		}

		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
		if _, err := s.validator.Validate(tokenStr); err != nil {
			log.Printf("HTTPServer: token JWT inválido: %v", err)
			c.JSON(http.StatusUnauthorized, gin.H{"error": fmt.Sprintf("token inválido: %v", err)})
			c.Abort()
			return
		}

		c.Next()
	}
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
		c.JSON(http.StatusBadRequest, mcp.NewJSONRPCError(nil, mcp.ErrCodeParse, "erro ao ler corpo da requisição"))
		return
	}

	var req mcp.JSONRPCRequest
	if err := json.Unmarshal(body, &req); err != nil {
		c.JSON(http.StatusBadRequest, mcp.NewJSONRPCError(nil, mcp.ErrCodeParse, "JSON inválido na requisição"))
		return
	}

	response := s.processor.HandleHTTPRequest(c.Request.Context(), &req)
	if response == nil {
		// Notification — no response body
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
// Format: "data: <json>\n\n" per spec MCP 2025-03-26.
func (s *HTTPServer) writeSSEResponse(c *gin.Context, response *mcp.JSONRPCResponse) {
	responseJSON, err := json.Marshal(response)
	if err != nil {
		c.JSON(http.StatusInternalServerError, mcp.NewJSONRPCError(nil, mcp.ErrCodeInternal, "erro ao serializar resposta"))
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	c.String(http.StatusOK, "data: %s\n\n", string(responseJSON))
}
