// Pacote mcp define os tipos do protocolo MCP (Model Context Protocol)
// e JSON-RPC 2.0 utilizados pelo servidor AgentHub.
package mcp

import "encoding/json"

// ========== JSON-RPC 2.0 ==========

// JSONRPCRequest represents a JSON-RPC 2.0 request received by the server.
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// JSONRPCResponse represents a JSON-RPC 2.0 response sent by the server.
type JSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
}

// JSONRPCError represents a JSON-RPC 2.0 error object.
type JSONRPCError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// ========== Códigos de Erro JSON-RPC ==========

const (
	// ErrCodeParse indicates failure to parse JSON
	ErrCodeParse = -32700
	// ErrCodeInvalidRequest indicates a malformed request
	ErrCodeInvalidRequest = -32600
	// ErrCodeMethodNotFound indicates an unknown method
	ErrCodeMethodNotFound = -32601
	// ErrCodeInvalidParams indicates invalid parameters
	ErrCodeInvalidParams = -32602
	// ErrCodeInternal indicates an internal server error
	ErrCodeInternal = -32603
	// ErrCodeServer indicates an MCP server-specific error
	ErrCodeServer = -32000
)

// ========== Capacidades do Servidor MCP ==========

// ServerInfo describes the MCP server.
type ServerInfo struct {
	Name            string `json:"name"`
	Version         string `json:"version"`
	ProtocolVersion string `json:"protocolVersion"`
}

// ClientInfo describes the connected MCP client.
type ClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// ToolsCapability indicates support for listing and calling tools.
type ToolsCapability struct{}

// ResourcesCapability indicates support for listing and reading resources.
type ResourcesCapability struct{}

// PromptsCapability indicates support for listing and getting prompts.
type PromptsCapability struct{}

// ServerCapabilities groups all capabilities supported by the server.
type ServerCapabilities struct {
	Tools     *ToolsCapability     `json:"tools,omitempty"`
	Resources *ResourcesCapability `json:"resources,omitempty"`
	Prompts   *PromptsCapability   `json:"prompts,omitempty"`
}

// InitializeParams contains parameters sent by the client during the handshake.
type InitializeParams struct {
	ProtocolVersion string      `json:"protocolVersion"`
	Capabilities    interface{} `json:"capabilities"`
	ClientInfo      ClientInfo  `json:"clientInfo"`
}

// InitializeResult is the server response to the initialization handshake.
type InitializeResult struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    ServerCapabilities `json:"capabilities"`
	ServerInfo      ServerInfo         `json:"serverInfo"`
}

// ========== Ferramentas (Tools) ==========

// Tool represents an AgentHub skill exposed as an MCP Tool.
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

// ListToolsResult is the result of the tools/list method.
type ListToolsResult struct {
	Tools []Tool `json:"tools"`
}

// CallToolParams contains the parameters of the tools/call method.
type CallToolParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

// ContentItem represents a content item returned by a tool.
type ContentItem struct {
	Type string `json:"type"` // "text", "image", "resource"
	Text string `json:"text,omitempty"`
	Data string `json:"data,omitempty"`
	URI  string `json:"uri,omitempty"`
}

// CallToolResult is the result of the tools/call method.
type CallToolResult struct {
	Content []ContentItem `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

// ========== Recursos (Resources) ==========

// Resource represents an AgentHub Knowledge Base exposed as an MCP Resource.
type Resource struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MIMEType    string `json:"mimeType,omitempty"`
}

// ListResourcesResult is the result of the resources/list method.
type ListResourcesResult struct {
	Resources []Resource `json:"resources"`
}

// ReadResourceParams contains the parameters of the resources/read method.
type ReadResourceParams struct {
	URI string `json:"uri"`
}

// ResourceContent represents the content of a read resource.
type ResourceContent struct {
	URI      string `json:"uri"`
	MIMEType string `json:"mimeType,omitempty"`
	Text     string `json:"text,omitempty"`
	Blob     string `json:"blob,omitempty"` // Base64 encoded
}

// ReadResourceResult is the result of the resources/read method.
type ReadResourceResult struct {
	Contents []ResourceContent `json:"contents"`
}

// ========== Prompts ==========

// PromptArgument describes an argument accepted by a prompt.
type PromptArgument struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
}

// Prompt represents a curated prompt exposed by the MCP server.
type Prompt struct {
	Name        string           `json:"name"`
	Description string           `json:"description,omitempty"`
	Arguments   []PromptArgument `json:"arguments,omitempty"`
}

// ListPromptsResult is the result of the prompts/list method.
type ListPromptsResult struct {
	Prompts []Prompt `json:"prompts"`
}

// GetPromptParams contains the parameters of the prompts/get method.
type GetPromptParams struct {
	Name      string            `json:"name"`
	Arguments map[string]string `json:"arguments,omitempty"`
}

// PromptMessage represents a message in the result of a prompt.
type PromptMessage struct {
	Role    string      `json:"role"` // "user", "assistant"
	Content ContentItem `json:"content"`
}

// GetPromptResult is the result of the prompts/get method.
type GetPromptResult struct {
	Description string          `json:"description,omitempty"`
	Messages    []PromptMessage `json:"messages"`
}

// ========== Funções auxiliares ==========

// NewJSONRPCResponse creates a successful JSON-RPC 2.0 response.
func NewJSONRPCResponse(id interface{}, result interface{}) (*JSONRPCResponse, error) {
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  resultJSON,
	}, nil
}

// NewJSONRPCError creates a JSON-RPC 2.0 error response.
func NewJSONRPCError(id interface{}, code int, message string) *JSONRPCResponse {
	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &JSONRPCError{
			Code:    code,
			Message: message,
		},
	}
}
