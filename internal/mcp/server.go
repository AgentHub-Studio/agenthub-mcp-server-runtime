// Pacote mcp implementa o MCPServer que gerencia a sessão com um cliente MCP.
// O servidor conecta o protocolo de comunicação aos handlers de ferramentas,
// recursos e prompts do AgentHub.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
)

// ToolsHandler defines the interface for listing and invoking MCP tools.
type ToolsHandler interface {
	ListTools(ctx context.Context) ([]Tool, error)
	CallTool(ctx context.Context, name string, arguments map[string]interface{}) (*CallToolResult, error)
}

// ResourcesHandler defines the interface for listing and reading MCP resources.
type ResourcesHandler interface {
	ListResources(ctx context.Context) ([]Resource, error)
	ReadResource(ctx context.Context, uri string) (*ResourceContent, error)
}

// PromptsHandler defines the interface for listing and getting MCP prompts.
type PromptsHandler interface {
	ListPrompts(ctx context.Context) ([]Prompt, error)
	GetPrompt(ctx context.Context, name string, arguments map[string]string) (*GetPromptResult, error)
}

// MCPServer manages the complete session with an MCP client.
// Receives requests via the protocol and dispatches them to the appropriate handlers.
type MCPServer struct {
	tools       ToolsHandler
	resources   ResourcesHandler
	prompts     PromptsHandler
	initialized bool
}

// NewMCPServer creates a new MCP server with the provided handlers.
func NewMCPServer(
	tools ToolsHandler,
	resources ResourcesHandler,
	prompts PromptsHandler,
) *MCPServer {
	return &MCPServer{
		tools:     tools,
		resources: resources,
		prompts:   prompts,
	}
}

// Start starts the stdio reading loop, blocking until the context is cancelled
// or the client closes the connection.
func (s *MCPServer) Start(ctx context.Context, reader io.Reader, writer io.Writer) error {
	protocol := NewProtocol(reader, writer, func(req *JSONRPCRequest) *JSONRPCResponse {
		return s.handleRequest(ctx, req)
	})

	log.Println("MCPServer: aguardando requisições do cliente...")
	return protocol.StartLoop()
}

// handleRequest dispatches a JSON-RPC request to the correct handler.
// Returns nil for notifications (no response expected).
func (s *MCPServer) handleRequest(ctx context.Context, req *JSONRPCRequest) *JSONRPCResponse {
	log.Printf("MCPServer: método=%s id=%v", req.Method, req.ID)

	switch req.Method {
	case "initialize":
		return s.handleInitialize(ctx, req)

	case "notifications/initialized":
		// Client notification confirming initialization — no response
		s.initialized = true
		log.Println("MCPServer: cliente confirmou inicialização")
		return nil

	case "tools/list":
		return s.handleListTools(ctx, req)

	case "tools/call":
		return s.handleCallTool(ctx, req)

	case "resources/list":
		return s.handleListResources(ctx, req)

	case "resources/read":
		return s.handleReadResource(ctx, req)

	case "prompts/list":
		return s.handleListPrompts(ctx, req)

	case "prompts/get":
		return s.handleGetPrompt(ctx, req)

	default:
		log.Printf("MCPServer: método desconhecido: %s", req.Method)
		return NewJSONRPCError(req.ID, ErrCodeMethodNotFound,
			fmt.Sprintf("método não suportado: %s", req.Method))
	}
}

// handleInitialize responds to the MCP handshake with the server capabilities.
func (s *MCPServer) handleInitialize(ctx context.Context, req *JSONRPCRequest) *JSONRPCResponse {
	result := InitializeResult{
		ProtocolVersion: "2024-11-05",
		ServerInfo: ServerInfo{
			Name:            "agenthub-mcp-server-runtime",
			Version:         "1.0.0",
			ProtocolVersion: "2024-11-05",
		},
		Capabilities: ServerCapabilities{
			Tools:     &ToolsCapability{},
			Resources: &ResourcesCapability{},
			Prompts:   &PromptsCapability{},
		},
	}

	resp, err := NewJSONRPCResponse(req.ID, result)
	if err != nil {
		return NewJSONRPCError(req.ID, ErrCodeInternal, "erro ao serializar resultado de inicialização")
	}

	log.Println("MCPServer: handshake de inicialização concluído")
	return resp
}

// handleListTools returns all active tenant skills as MCP tools.
func (s *MCPServer) handleListTools(ctx context.Context, req *JSONRPCRequest) *JSONRPCResponse {
	tools, err := s.tools.ListTools(ctx)
	if err != nil {
		log.Printf("MCPServer: erro ao listar ferramentas: %v", err)
		return NewJSONRPCError(req.ID, ErrCodeInternal, fmt.Sprintf("erro ao listar ferramentas: %v", err))
	}

	result := ListToolsResult{Tools: tools}
	resp, err := NewJSONRPCResponse(req.ID, result)
	if err != nil {
		return NewJSONRPCError(req.ID, ErrCodeInternal, "erro ao serializar ferramentas")
	}

	log.Printf("MCPServer: listadas %d ferramentas", len(tools))
	return resp
}

// handleCallTool invokes an AgentHub skill and returns the result.
func (s *MCPServer) handleCallTool(ctx context.Context, req *JSONRPCRequest) *JSONRPCResponse {
	var params CallToolParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewJSONRPCError(req.ID, ErrCodeInvalidParams, "parâmetros inválidos para tools/call")
	}

	result, err := s.tools.CallTool(ctx, params.Name, params.Arguments)
	if err != nil {
		log.Printf("MCPServer: erro ao chamar ferramenta '%s': %v", params.Name, err)
		// Return as MCP error result (not JSON-RPC error)
		errResult := &CallToolResult{
			IsError: true,
			Content: []ContentItem{
				{Type: "text", Text: fmt.Sprintf("erro ao executar ferramenta: %v", err)},
			},
		}
		resp, _ := NewJSONRPCResponse(req.ID, errResult)
		return resp
	}

	resp, err := NewJSONRPCResponse(req.ID, result)
	if err != nil {
		return NewJSONRPCError(req.ID, ErrCodeInternal, "erro ao serializar resultado da ferramenta")
	}

	log.Printf("MCPServer: ferramenta '%s' executada com sucesso", params.Name)
	return resp
}

// handleListResources returns all active Knowledge Bases as MCP resources.
func (s *MCPServer) handleListResources(ctx context.Context, req *JSONRPCRequest) *JSONRPCResponse {
	resources, err := s.resources.ListResources(ctx)
	if err != nil {
		log.Printf("MCPServer: erro ao listar recursos: %v", err)
		return NewJSONRPCError(req.ID, ErrCodeInternal, fmt.Sprintf("erro ao listar recursos: %v", err))
	}

	result := ListResourcesResult{Resources: resources}
	resp, err := NewJSONRPCResponse(req.ID, result)
	if err != nil {
		return NewJSONRPCError(req.ID, ErrCodeInternal, "erro ao serializar recursos")
	}

	log.Printf("MCPServer: listados %d recursos", len(resources))
	return resp
}

// handleReadResource returns the content of a Knowledge Base by URI.
func (s *MCPServer) handleReadResource(ctx context.Context, req *JSONRPCRequest) *JSONRPCResponse {
	var params ReadResourceParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewJSONRPCError(req.ID, ErrCodeInvalidParams, "parâmetros inválidos para resources/read")
	}

	content, err := s.resources.ReadResource(ctx, params.URI)
	if err != nil {
		log.Printf("MCPServer: erro ao ler recurso '%s': %v", params.URI, err)
		return NewJSONRPCError(req.ID, ErrCodeInternal, fmt.Sprintf("erro ao ler recurso: %v", err))
	}

	result := ReadResourceResult{Contents: []ResourceContent{*content}}
	resp, err := NewJSONRPCResponse(req.ID, result)
	if err != nil {
		return NewJSONRPCError(req.ID, ErrCodeInternal, "erro ao serializar conteúdo do recurso")
	}

	log.Printf("MCPServer: recurso '%s' lido com sucesso", params.URI)
	return resp
}

// handleListPrompts returns the server's curated prompts.
func (s *MCPServer) handleListPrompts(ctx context.Context, req *JSONRPCRequest) *JSONRPCResponse {
	prompts, err := s.prompts.ListPrompts(ctx)
	if err != nil {
		log.Printf("MCPServer: erro ao listar prompts: %v", err)
		return NewJSONRPCError(req.ID, ErrCodeInternal, fmt.Sprintf("erro ao listar prompts: %v", err))
	}

	result := ListPromptsResult{Prompts: prompts}
	resp, err := NewJSONRPCResponse(req.ID, result)
	if err != nil {
		return NewJSONRPCError(req.ID, ErrCodeInternal, "erro ao serializar prompts")
	}

	log.Printf("MCPServer: listados %d prompts", len(prompts))
	return resp
}

// HandleHTTPRequest is the entry point for requests coming from the HTTP transport.
// Allows the HTTPServer to delegate JSON-RPC processing to the MCPServer.
func (s *MCPServer) HandleHTTPRequest(ctx context.Context, req *JSONRPCRequest) *JSONRPCResponse {
	return s.handleRequest(ctx, req)
}

// handleGetPrompt returns a prompt rendered with the provided arguments.
func (s *MCPServer) handleGetPrompt(ctx context.Context, req *JSONRPCRequest) *JSONRPCResponse {
	var params GetPromptParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NewJSONRPCError(req.ID, ErrCodeInvalidParams, "parâmetros inválidos para prompts/get")
	}

	result, err := s.prompts.GetPrompt(ctx, params.Name, params.Arguments)
	if err != nil {
		log.Printf("MCPServer: erro ao obter prompt '%s': %v", params.Name, err)
		return NewJSONRPCError(req.ID, ErrCodeInternal, fmt.Sprintf("erro ao obter prompt: %v", err))
	}

	resp, err := NewJSONRPCResponse(req.ID, result)
	if err != nil {
		return NewJSONRPCError(req.ID, ErrCodeInternal, "erro ao serializar resultado do prompt")
	}

	log.Printf("MCPServer: prompt '%s' obtido com sucesso", params.Name)
	return resp
}
