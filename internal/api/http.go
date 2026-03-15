// Pacote api implementa o servidor HTTP do MCP Server Runtime.
// Expõe endpoints para uso do protocolo MCP via HTTP além do transporte stdio padrão.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/AgentHub-Studio/agenthub-mcp-server-runtime/internal/mcp"
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
	router    *gin.Engine
	server    *http.Server
}

// NewHTTPServer creates a new HTTP server for the MCP Server Runtime.
func NewHTTPServer(port int, processor MessageProcessor) *HTTPServer {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())

	s := &HTTPServer{
		port:      port,
		processor: processor,
		router:    router,
	}

	s.registerRoutes()
	return s
}

// registerRoutes configures all HTTP routes of the server.
func (s *HTTPServer) registerRoutes() {
	s.router.GET("/health", s.health)
	s.router.POST("/mcp", s.handleMCPMessage)
	s.router.GET("/mcp/sse", s.handleSSE)
}

// Start starts the HTTP server on the configured port.
func (s *HTTPServer) Start() error {
	s.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", s.port),
		Handler: s.router,
	}

	log.Printf("HTTPServer: iniciando na porta %d", s.port)
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

// handleMCPMessage receives a JSON-RPC request via HTTP and returns the response.
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
		// Notification — no response
		c.Status(http.StatusNoContent)
		return
	}

	c.JSON(http.StatusOK, response)
}

// handleSSE maintains an SSE (Server-Sent Events) connection for clients that prefer
// the persistent HTTP transport instead of stdio.
// GET /mcp/sse
func (s *HTTPServer) handleSSE(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	log.Printf("HTTPServer: cliente SSE conectado: %s", c.ClientIP())

	// Channel to send events to the client
	eventChan := make(chan string, 10)

	// Send connection established event
	eventChan <- `data: {"type":"connected","server":"agenthub-mcp-server-runtime"}` + "\n\n"

	c.Stream(func(w io.Writer) bool {
		select {
		case event, ok := <-eventChan:
			if !ok {
				return false
			}
			fmt.Fprintf(w, "%s", event)
			return true
		case <-c.Request.Context().Done():
			log.Printf("HTTPServer: cliente SSE desconectado: %s", c.ClientIP())
			return false
		}
	})
}
