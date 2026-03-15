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

// ProcessadorMensagem define a interface para processar uma requisição JSON-RPC
// e retornar uma resposta. Implementada pelo MCPServidor.
type ProcessadorMensagem interface {
	ProcessarRequisicaoHTTP(ctx context.Context, req *mcp.RequisicaoJSONRPC) *mcp.RespostaJSONRPC
}

// ServidorHTTP gerencia o servidor HTTP do MCP Server Runtime.
type ServidorHTTP struct {
	porta       int
	processador ProcessadorMensagem
	router      *gin.Engine
	servidor    *http.Server
}

// NovoServidorHTTP cria um novo servidor HTTP para o MCP Server Runtime.
func NovoServidorHTTP(porta int, processador ProcessadorMensagem) *ServidorHTTP {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())

	s := &ServidorHTTP{
		porta:       porta,
		processador: processador,
		router:      router,
	}

	s.registrarRotas()
	return s
}

// registrarRotas configura todas as rotas HTTP do servidor.
func (s *ServidorHTTP) registrarRotas() {
	s.router.GET("/health", s.health)
	s.router.POST("/mcp", s.processarMensagemMCP)
	s.router.GET("/mcp/sse", s.iniciarSSE)
}

// Iniciar inicia o servidor HTTP na porta configurada.
func (s *ServidorHTTP) Iniciar() error {
	s.servidor = &http.Server{
		Addr:    fmt.Sprintf(":%d", s.porta),
		Handler: s.router,
	}

	log.Printf("ServidorHTTP: iniciando na porta %d", s.porta)
	return s.servidor.ListenAndServe()
}

// Parar realiza o shutdown graceful do servidor HTTP.
func (s *ServidorHTTP) Parar(ctx context.Context) error {
	if s.servidor == nil {
		return nil
	}
	return s.servidor.Shutdown(ctx)
}

// health retorna o status de saúde do servidor.
// GET /health
func (s *ServidorHTTP) health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "UP",
		"servico": "agenthub-mcp-server-runtime",
		"versao":  "1.0.0",
		"agora":   time.Now().UTC().Format(time.RFC3339),
	})
}

// processarMensagemMCP recebe uma requisição JSON-RPC via HTTP e retorna a resposta.
// POST /mcp
func (s *ServidorHTTP) processarMensagemMCP(c *gin.Context) {
	corpo, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, mcp.NovoErroJSONRPC(nil, mcp.ErroParseamento, "erro ao ler corpo da requisição"))
		return
	}

	var req mcp.RequisicaoJSONRPC
	if err := json.Unmarshal(corpo, &req); err != nil {
		c.JSON(http.StatusBadRequest, mcp.NovoErroJSONRPC(nil, mcp.ErroParseamento, "JSON inválido na requisição"))
		return
	}

	resposta := s.processador.ProcessarRequisicaoHTTP(c.Request.Context(), &req)
	if resposta == nil {
		// Notificação — sem resposta
		c.Status(http.StatusNoContent)
		return
	}

	c.JSON(http.StatusOK, resposta)
}

// iniciarSSE mantém uma conexão SSE (Server-Sent Events) para clientes que preferem
// o transporte HTTP persistente ao invés de stdio.
// GET /mcp/sse
func (s *ServidorHTTP) iniciarSSE(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	log.Printf("ServidorHTTP: cliente SSE conectado: %s", c.ClientIP())

	// Canal para enviar eventos ao cliente
	canalEventos := make(chan string, 10)

	// Enviar evento de conexão estabelecida
	canalEventos <- `data: {"type":"connected","server":"agenthub-mcp-server-runtime"}` + "\n\n"

	c.Stream(func(w io.Writer) bool {
		select {
		case evento, ok := <-canalEventos:
			if !ok {
				return false
			}
			fmt.Fprintf(w, "%s", evento)
			return true
		case <-c.Request.Context().Done():
			log.Printf("ServidorHTTP: cliente SSE desconectado: %s", c.ClientIP())
			return false
		}
	})
}
