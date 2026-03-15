// Este arquivo contém o handler de recursos (resources), que expõe as Knowledge Bases
// ativas do AgentHub como MCP Resources com URI no formato agenthub://kb/{id}.
package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/AgentHub-Studio/agenthub-mcp-server-runtime/internal/backend"
	"github.com/AgentHub-Studio/agenthub-mcp-server-runtime/internal/mcp"
)

// kbURIPrefix is the URI prefix used to identify knowledge bases.
const kbURIPrefix = "agenthub://kb/"

// BackendClientResourcesIface defines the backend operations used by the resources handler.
type BackendClientResourcesIface interface {
	ListKnowledgeBases(ctx context.Context) ([]backend.KnowledgeBaseDTO, error)
	GetKnowledgeBase(ctx context.Context, kbID string) (*backend.KnowledgeBaseDTO, error)
}

// ResourcesHandlerImpl implements the ResourcesHandler interface of the MCPServer.
// Exposes AgentHub Knowledge Bases as MCP resources accessible via URI.
type ResourcesHandlerImpl struct {
	backendClient BackendClientResourcesIface
}

// NewResourcesHandler creates a new MCP resources handler.
func NewResourcesHandler(backendClient BackendClientResourcesIface) *ResourcesHandlerImpl {
	return &ResourcesHandlerImpl{backendClient: backendClient}
}

// ListResources queries the tenant's knowledge bases and converts them to MCP Resources.
// Each knowledge base receives a URI in the format agenthub://kb/{id}.
func (h *ResourcesHandlerImpl) ListResources(ctx context.Context) ([]mcp.Resource, error) {
	kbs, err := h.backendClient.ListKnowledgeBases(ctx)
	if err != nil {
		return nil, fmt.Errorf("erro ao buscar knowledge bases do backend: %w", err)
	}

	resources := make([]mcp.Resource, 0, len(kbs))
	for _, kb := range kbs {
		resource := kbToResource(kb)
		resources = append(resources, resource)
	}

	return resources, nil
}

// ReadResource returns the metadata of a Knowledge Base by URI.
// The URI must be in the format agenthub://kb/{uuid}.
func (h *ResourcesHandlerImpl) ReadResource(ctx context.Context, uri string) (*mcp.ResourceContent, error) {
	kbID, err := extractIDFromURI(uri)
	if err != nil {
		return nil, err
	}

	kb, err := h.backendClient.GetKnowledgeBase(ctx, kbID)
	if err != nil {
		return nil, fmt.Errorf("erro ao ler knowledge base '%s': %w", kbID, err)
	}

	// Serialize KB metadata as JSON
	metadata, err := json.Marshal(map[string]interface{}{
		"id":          kb.ID,
		"name":        kb.Name,
		"description": kb.Description,
		"status":      kb.Status,
	})
	if err != nil {
		return nil, fmt.Errorf("erro ao serializar metadados da knowledge base: %w", err)
	}

	return &mcp.ResourceContent{
		URI:      uri,
		MIMEType: "application/json",
		Text:     string(metadata),
	}, nil
}

// kbToResource converts a KnowledgeBaseDTO to an MCP Resource.
func kbToResource(kb backend.KnowledgeBaseDTO) mcp.Resource {
	return mcp.Resource{
		URI:         kbURIPrefix + kb.ID,
		Name:        kb.Name,
		Description: kb.Description,
		MIMEType:    "application/json",
	}
}

// extractIDFromURI extracts the knowledge base UUID from a URI in the format agenthub://kb/{id}.
func extractIDFromURI(uri string) (string, error) {
	if !strings.HasPrefix(uri, kbURIPrefix) {
		return "", fmt.Errorf("URI inválido: esperado formato '%s{id}', recebido '%s'",
			kbURIPrefix, uri)
	}

	kbID := strings.TrimPrefix(uri, kbURIPrefix)
	if kbID == "" {
		return "", fmt.Errorf("URI inválido: ID da knowledge base não pode ser vazio")
	}

	return kbID, nil
}
