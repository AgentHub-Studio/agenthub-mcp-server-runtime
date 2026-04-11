// Package handler contains MCP protocol handlers for the AgentHub MCP Server Runtime.
// This file implements the MCP Prompts handler, which exposes curated prompts for
// common AgentHub operations. Prompts are loaded dynamically from the backend API
// (prompt_template table) and augmented with built-in static prompts as fallback.
package handler

import (
	"context"
	"fmt"
	"strings"

	"github.com/AgentHub-Studio/agenthub-mcp-server-runtime/internal/backend"
	"github.com/AgentHub-Studio/agenthub-mcp-server-runtime/internal/mcp"
)

// BackendClientPromptsIface defines the backend operations used by the prompts handler.
type BackendClientPromptsIface interface {
	ListPromptTemplates(ctx context.Context) ([]backend.PromptTemplateDTO, error)
}

// staticPromptCatalog provides built-in prompts used as fallback when the backend
// is unavailable or returns no templates.
var staticPromptCatalog = []mcp.Prompt{
	{
		Name:        "pesquisar-conhecimento",
		Description: "Realiza uma busca semântica na base de conhecimento do AgentHub e retorna documentos relevantes.",
		Arguments: []mcp.PromptArgument{
			{
				Name:        "consulta",
				Description: "Texto ou pergunta para buscar na base de conhecimento",
				Required:    true,
			},
			{
				Name:        "kb_id",
				Description: "ID da knowledge base específica (opcional; busca em todas se omitido)",
				Required:    false,
			},
			{
				Name:        "limite",
				Description: "Número máximo de documentos a retornar (padrão: 5)",
				Required:    false,
			},
		},
	},
	{
		Name:        "executar-skill",
		Description: "Executa uma skill do AgentHub com os parâmetros fornecidos via skill-runtime.",
		Arguments: []mcp.PromptArgument{
			{
				Name:        "skill_slug",
				Description: "Identificador único da skill a executar (ex: 'document-search', 'sql-query')",
				Required:    true,
			},
			{
				Name:        "input_json",
				Description: "Parâmetros de entrada para a skill em formato JSON",
				Required:    true,
			},
		},
	},
}

// PromptsHandlerImpl implements the PromptsHandler interface of the MCPServer.
// Loads prompts dynamically from the backend API; falls back to static catalog
// on error so the MCP server remains functional during backend outages.
type PromptsHandlerImpl struct {
	backendClient BackendClientPromptsIface
}

// NewPromptsHandler creates a new MCP prompts handler.
// When backendClient is nil, only static prompts are served.
func NewPromptsHandler(backendClient BackendClientPromptsIface) *PromptsHandlerImpl {
	return &PromptsHandlerImpl{backendClient: backendClient}
}

// ListPrompts returns the combined list of dynamic (backend) and static prompts.
// Dynamic prompts from the backend take precedence; static prompts fill in any gaps.
func (h *PromptsHandlerImpl) ListPrompts(ctx context.Context) ([]mcp.Prompt, error) {
	dynamic := h.loadDynamic(ctx)
	if len(dynamic) > 0 {
		// Merge: start with dynamic, append static prompts not already present.
		dynamicSlugs := make(map[string]struct{}, len(dynamic))
		for _, p := range dynamic {
			dynamicSlugs[p.Name] = struct{}{}
		}
		combined := append([]mcp.Prompt(nil), dynamic...)
		for _, p := range staticPromptCatalog {
			if _, exists := dynamicSlugs[p.Name]; !exists {
				combined = append(combined, p)
			}
		}
		return combined, nil
	}
	// Fallback: return static catalog when backend is unavailable.
	return staticPromptCatalog, nil
}

// loadDynamic fetches prompt templates from the backend and converts them to
// mcp.Prompt. Returns an empty slice (not an error) on failure so callers can
// gracefully fall back to the static catalog.
func (h *PromptsHandlerImpl) loadDynamic(ctx context.Context) []mcp.Prompt {
	if h.backendClient == nil {
		return nil
	}
	templates, err := h.backendClient.ListPromptTemplates(ctx)
	if err != nil {
		// Non-fatal — backend may be temporarily unavailable.
		return nil
	}
	prompts := make([]mcp.Prompt, 0, len(templates))
	for _, t := range templates {
		prompts = append(prompts, mcp.Prompt{
			Name:        t.Slug,
			Description: t.Description,
			// Dynamic prompts expose a single required "context" argument that
			// passes extra variables into the template content.
			Arguments: []mcp.PromptArgument{
				{
					Name:        "context",
					Description: "Optional extra context to inject into the prompt",
					Required:    false,
				},
			},
		})
	}
	return prompts
}

// GetPrompt returns a prompt rendered with the provided arguments.
func (h *PromptsHandlerImpl) GetPrompt(
	ctx context.Context,
	name string,
	arguments map[string]string,
) (*mcp.GetPromptResult, error) {
	// Try to find the prompt in dynamic templates first.
	if h.backendClient != nil {
		templates, err := h.backendClient.ListPromptTemplates(ctx)
		if err == nil {
			for _, t := range templates {
				if t.Slug == name {
					return h.renderDynamic(t, arguments), nil
				}
			}
		}
	}

	// Fall back to static prompts.
	switch name {
	case "pesquisar-conhecimento":
		return h.renderSearchKnowledge(arguments)
	case "executar-skill":
		return h.renderExecuteSkill(arguments)
	default:
		return nil, fmt.Errorf("prompt não encontrado: '%s'", name)
	}
}

// renderDynamic renders a dynamic prompt template with the provided arguments.
func (h *PromptsHandlerImpl) renderDynamic(t backend.PromptTemplateDTO, args map[string]string) *mcp.GetPromptResult {
	content := t.Content
	// Simple variable substitution: {{key}} → args[key]
	for k, v := range args {
		content = strings.ReplaceAll(content, "{{"+k+"}}", v)
	}
	return &mcp.GetPromptResult{
		Description: t.Description,
		Messages: []mcp.PromptMessage{
			{
				Role: "user",
				Content: mcp.ContentItem{
					Type: "text",
					Text: content,
				},
			},
		},
	}
}

// renderSearchKnowledge generates the knowledge base search prompt.
func (h *PromptsHandlerImpl) renderSearchKnowledge(args map[string]string) (*mcp.GetPromptResult, error) {
	query := args["consulta"]
	if query == "" {
		return nil, fmt.Errorf("argumento 'consulta' é obrigatório para o prompt 'pesquisar-conhecimento'")
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Pesquise na base de conhecimento por: %s\n\n", query))

	if kbID := args["kb_id"]; kbID != "" {
		sb.WriteString(fmt.Sprintf("Base de conhecimento específica: %s\n", kbID))
	} else {
		sb.WriteString("Buscar em todas as bases de conhecimento disponíveis.\n")
	}

	if limit := args["limite"]; limit != "" {
		sb.WriteString(fmt.Sprintf("Retornar no máximo %s documentos.\n", limit))
	} else {
		sb.WriteString("Retornar no máximo 5 documentos.\n")
	}

	sb.WriteString("\nPor favor, use a ferramenta 'document-search' para realizar a busca e apresente os resultados de forma clara e organizada.")

	return &mcp.GetPromptResult{
		Description: "Busca semântica na base de conhecimento do AgentHub",
		Messages: []mcp.PromptMessage{
			{
				Role: "user",
				Content: mcp.ContentItem{
					Type: "text",
					Text: sb.String(),
				},
			},
		},
	}, nil
}

// renderExecuteSkill generates the prompt for executing a skill.
func (h *PromptsHandlerImpl) renderExecuteSkill(args map[string]string) (*mcp.GetPromptResult, error) {
	skillSlug := args["skill_slug"]
	if skillSlug == "" {
		return nil, fmt.Errorf("argumento 'skill_slug' é obrigatório para o prompt 'executar-skill'")
	}

	inputJSON := args["input_json"]
	if inputJSON == "" {
		return nil, fmt.Errorf("argumento 'input_json' é obrigatório para o prompt 'executar-skill'")
	}

	message := fmt.Sprintf(
		"Execute a skill '%s' com os seguintes parâmetros:\n\n%s\n\nUse a ferramenta correspondente no AgentHub e retorne o resultado da execução.",
		skillSlug,
		inputJSON,
	)

	return &mcp.GetPromptResult{
		Description: fmt.Sprintf("Execução da skill '%s' via AgentHub", skillSlug),
		Messages: []mcp.PromptMessage{
			{
				Role: "user",
				Content: mcp.ContentItem{
					Type: "text",
					Text: message,
				},
			},
		},
	}, nil
}
