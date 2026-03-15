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

// prefixoURIKnowledgeBase é o prefixo URI utilizado para identificar knowledge bases.
const prefixoURIKnowledgeBase = "agenthub://kb/"

// InterfaceClienteBackendRecursos define as operações de backend usadas pelo handler de recursos.
type InterfaceClienteBackendRecursos interface {
	ListarKnowledgeBases(ctx context.Context) ([]backend.KnowledgeBaseDTO, error)
	LerKnowledgeBase(ctx context.Context, kbID string) (*backend.KnowledgeBaseDTO, error)
}

// HandlerRecursos implementa a interface GerenciadorRecursos do MCPServidor.
// Expõe as Knowledge Bases do AgentHub como recursos MCP acessíveis via URI.
type HandlerRecursos struct {
	clienteBackend InterfaceClienteBackendRecursos
}

// NovoHandlerRecursos cria um novo handler de recursos MCP.
func NovoHandlerRecursos(clienteBackend InterfaceClienteBackendRecursos) *HandlerRecursos {
	return &HandlerRecursos{clienteBackend: clienteBackend}
}

// ListarRecursos consulta as knowledge bases do tenant e as converte para MCP Resources.
// Cada knowledge base recebe um URI no formato agenthub://kb/{id}.
func (h *HandlerRecursos) ListarRecursos(ctx context.Context) ([]mcp.Recurso, error) {
	kbs, err := h.clienteBackend.ListarKnowledgeBases(ctx)
	if err != nil {
		return nil, fmt.Errorf("erro ao buscar knowledge bases do backend: %w", err)
	}

	recursos := make([]mcp.Recurso, 0, len(kbs))
	for _, kb := range kbs {
		recurso := converterKBParaRecurso(kb)
		recursos = append(recursos, recurso)
	}

	return recursos, nil
}

// LerRecurso retorna os metadados de uma Knowledge Base pelo URI.
// O URI deve estar no formato agenthub://kb/{uuid}.
func (h *HandlerRecursos) LerRecurso(ctx context.Context, uri string) (*mcp.ConteudoRecurso, error) {
	kbID, err := extrairIDdoURI(uri)
	if err != nil {
		return nil, err
	}

	kb, err := h.clienteBackend.LerKnowledgeBase(ctx, kbID)
	if err != nil {
		return nil, fmt.Errorf("erro ao ler knowledge base '%s': %w", kbID, err)
	}

	// Serializar metadados da KB como JSON
	metadados, err := json.Marshal(map[string]interface{}{
		"id":        kb.ID,
		"nome":      kb.Nome,
		"descricao": kb.Descricao,
		"status":    kb.Status,
	})
	if err != nil {
		return nil, fmt.Errorf("erro ao serializar metadados da knowledge base: %w", err)
	}

	return &mcp.ConteudoRecurso{
		URI:      uri,
		TipoMIME: "application/json",
		Texto:    string(metadados),
	}, nil
}

// converterKBParaRecurso converte um KnowledgeBaseDTO para um Recurso MCP.
func converterKBParaRecurso(kb backend.KnowledgeBaseDTO) mcp.Recurso {
	return mcp.Recurso{
		URI:       prefixoURIKnowledgeBase + kb.ID,
		Nome:      kb.Nome,
		Descricao: kb.Descricao,
		TipoMIME:  "application/json",
	}
}

// extrairIDdoURI extrai o UUID da knowledge base de um URI no formato agenthub://kb/{id}.
func extrairIDdoURI(uri string) (string, error) {
	if !strings.HasPrefix(uri, prefixoURIKnowledgeBase) {
		return "", fmt.Errorf("URI inválido: esperado formato '%s{id}', recebido '%s'",
			prefixoURIKnowledgeBase, uri)
	}

	kbID := strings.TrimPrefix(uri, prefixoURIKnowledgeBase)
	if kbID == "" {
		return "", fmt.Errorf("URI inválido: ID da knowledge base não pode ser vazio")
	}

	return kbID, nil
}
