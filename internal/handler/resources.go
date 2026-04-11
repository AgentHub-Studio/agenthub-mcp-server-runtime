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

// registrySearchURIPrefix is the URI prefix used for registry package searches.
const registrySearchURIPrefix = "agenthub://registry/search/"

// BackendClientResourcesIface defines the backend operations used by the resources handler.
type BackendClientResourcesIface interface {
	ListKnowledgeBases(ctx context.Context) ([]backend.KnowledgeBaseDTO, error)
	GetKnowledgeBase(ctx context.Context, kbID string) (*backend.KnowledgeBaseDTO, error)
	SearchPackages(ctx context.Context, query string, pkgType string) ([]backend.PackageSearchDTO, error)
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
// Also exposes the registry package search capability as a static resource template.
func (h *ResourcesHandlerImpl) ListResources(ctx context.Context) ([]mcp.Resource, error) {
	kbs, err := h.backendClient.ListKnowledgeBases(ctx)
	if err != nil {
		return nil, fmt.Errorf("erro ao buscar knowledge bases do backend: %w", err)
	}

	resources := make([]mcp.Resource, 0, len(kbs)+1)
	for _, kb := range kbs {
		resources = append(resources, kbToResource(kb))
	}

	// Expose registry search capability as a static resource template.
	// Clients read agenthub://registry/search/{query} to get matching packages.
	resources = append(resources, mcp.Resource{
		URI:         registrySearchURIPrefix + "{query}",
		Name:        "Registry Package Search",
		Description: "Search the AgentHub public registry. Replace {query} with your search term to find agents, skills, tools, and knowledge bases.",
		MIMEType:    "application/json",
	})

	return resources, nil
}

// ReadResource returns the content of a resource by URI.
// Supported URI formats:
//   - agenthub://kb/{uuid}               — Knowledge Base metadata
//   - agenthub://registry/search/{query} — Registry package search results
func (h *ResourcesHandlerImpl) ReadResource(ctx context.Context, uri string) (*mcp.ResourceContent, error) {
	if strings.HasPrefix(uri, registrySearchURIPrefix) {
		return h.readRegistrySearch(ctx, uri)
	}
	return h.readKnowledgeBase(ctx, uri)
}

// readKnowledgeBase returns the metadata of a Knowledge Base by URI.
// The URI must be in the format agenthub://kb/{uuid}.
func (h *ResourcesHandlerImpl) readKnowledgeBase(ctx context.Context, uri string) (*mcp.ResourceContent, error) {
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

// readRegistrySearch performs a package search and returns results as JSON.
// The URI must be in the format agenthub://registry/search/{query}.
func (h *ResourcesHandlerImpl) readRegistrySearch(ctx context.Context, uri string) (*mcp.ResourceContent, error) {
	query := strings.TrimPrefix(uri, registrySearchURIPrefix)
	if query == "" {
		return nil, fmt.Errorf("URI inválido: query de busca não pode ser vazia")
	}

	packages, err := h.backendClient.SearchPackages(ctx, query, "")
	if err != nil {
		return nil, fmt.Errorf("erro ao buscar pacotes para query '%s': %w", query, err)
	}

	results, err := json.Marshal(map[string]interface{}{
		"query":   query,
		"results": packages,
		"count":   len(packages),
	})
	if err != nil {
		return nil, fmt.Errorf("erro ao serializar resultados da busca: %w", err)
	}

	return &mcp.ResourceContent{
		URI:      uri,
		MIMEType: "application/json",
		Text:     string(results),
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
