// Este arquivo contém o handler de prompts, que expõe prompts curados para
// operações comuns do AgentHub como MCP Prompts.
// Nesta versão inicial, os prompts são definidos estaticamente no código.
package handler

import (
	"context"
	"fmt"
	"strings"

	"github.com/AgentHub-Studio/agenthub-mcp-server-runtime/internal/mcp"
)

// promptCatalog holds the curated prompts available on the MCP server.
var promptCatalog = []mcp.Prompt{
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
// Provides curated prompts for common AgentHub operations.
type PromptsHandlerImpl struct{}

// NewPromptsHandler creates a new MCP prompts handler.
func NewPromptsHandler() *PromptsHandlerImpl {
	return &PromptsHandlerImpl{}
}

// ListPrompts returns the list of curated prompts available on the server.
func (h *PromptsHandlerImpl) ListPrompts(ctx context.Context) ([]mcp.Prompt, error) {
	return promptCatalog, nil
}

// GetPrompt returns a prompt rendered with the provided arguments.
// Each prompt has a message template that is filled with the arguments.
func (h *PromptsHandlerImpl) GetPrompt(
	ctx context.Context,
	name string,
	arguments map[string]string,
) (*mcp.GetPromptResult, error) {
	switch name {
	case "pesquisar-conhecimento":
		return h.renderSearchKnowledge(arguments)
	case "executar-skill":
		return h.renderExecuteSkill(arguments)
	default:
		return nil, fmt.Errorf("prompt não encontrado: '%s'", name)
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
