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

// catálogo de prompts curados disponíveis no servidor MCP.
var catalogoPrompts = []mcp.Prompt{
	{
		Nome:      "pesquisar-conhecimento",
		Descricao: "Realiza uma busca semântica na base de conhecimento do AgentHub e retorna documentos relevantes.",
		Argumentos: []mcp.ArgumentoPrompt{
			{
				Nome:        "consulta",
				Descricao:   "Texto ou pergunta para buscar na base de conhecimento",
				Obrigatorio: true,
			},
			{
				Nome:        "kb_id",
				Descricao:   "ID da knowledge base específica (opcional; busca em todas se omitido)",
				Obrigatorio: false,
			},
			{
				Nome:        "limite",
				Descricao:   "Número máximo de documentos a retornar (padrão: 5)",
				Obrigatorio: false,
			},
		},
	},
	{
		Nome:      "executar-skill",
		Descricao: "Executa uma skill do AgentHub com os parâmetros fornecidos via skill-runtime.",
		Argumentos: []mcp.ArgumentoPrompt{
			{
				Nome:        "skill_slug",
				Descricao:   "Identificador único da skill a executar (ex: 'document-search', 'sql-query')",
				Obrigatorio: true,
			},
			{
				Nome:        "input_json",
				Descricao:   "Parâmetros de entrada para a skill em formato JSON",
				Obrigatorio: true,
			},
		},
	},
}

// HandlerPrompts implementa a interface GerenciadorPrompts do MCPServidor.
// Fornece prompts curados para operações comuns do AgentHub.
type HandlerPrompts struct{}

// NovoHandlerPrompts cria um novo handler de prompts MCP.
func NovoHandlerPrompts() *HandlerPrompts {
	return &HandlerPrompts{}
}

// ListarPrompts retorna a lista de prompts curados disponíveis no servidor.
func (h *HandlerPrompts) ListarPrompts(ctx context.Context) ([]mcp.Prompt, error) {
	return catalogoPrompts, nil
}

// ObterPrompt retorna um prompt renderizado com os argumentos fornecidos.
// Cada prompt tem um template de mensagem que é preenchido com os argumentos.
func (h *HandlerPrompts) ObterPrompt(
	ctx context.Context,
	nome string,
	argumentos map[string]string,
) (*mcp.ResultadoObterPrompt, error) {
	switch nome {
	case "pesquisar-conhecimento":
		return h.renderizarPesquisarConhecimento(argumentos)
	case "executar-skill":
		return h.renderizarExecutarSkill(argumentos)
	default:
		return nil, fmt.Errorf("prompt não encontrado: '%s'", nome)
	}
}

// renderizarPesquisarConhecimento gera o prompt de busca em knowledge base.
func (h *HandlerPrompts) renderizarPesquisarConhecimento(args map[string]string) (*mcp.ResultadoObterPrompt, error) {
	consulta := args["consulta"]
	if consulta == "" {
		return nil, fmt.Errorf("argumento 'consulta' é obrigatório para o prompt 'pesquisar-conhecimento'")
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Pesquise na base de conhecimento por: %s\n\n", consulta))

	if kbID := args["kb_id"]; kbID != "" {
		sb.WriteString(fmt.Sprintf("Base de conhecimento específica: %s\n", kbID))
	} else {
		sb.WriteString("Buscar em todas as bases de conhecimento disponíveis.\n")
	}

	if limite := args["limite"]; limite != "" {
		sb.WriteString(fmt.Sprintf("Retornar no máximo %s documentos.\n", limite))
	} else {
		sb.WriteString("Retornar no máximo 5 documentos.\n")
	}

	sb.WriteString("\nPor favor, use a ferramenta 'document-search' para realizar a busca e apresente os resultados de forma clara e organizada.")

	return &mcp.ResultadoObterPrompt{
		Descricao: "Busca semântica na base de conhecimento do AgentHub",
		Mensagens: []mcp.MensagemPrompt{
			{
				Papel: "user",
				Conteudo: mcp.ItemConteudo{
					Tipo:  "text",
					Texto: sb.String(),
				},
			},
		},
	}, nil
}

// renderizarExecutarSkill gera o prompt para execução de uma skill.
func (h *HandlerPrompts) renderizarExecutarSkill(args map[string]string) (*mcp.ResultadoObterPrompt, error) {
	skillSlug := args["skill_slug"]
	if skillSlug == "" {
		return nil, fmt.Errorf("argumento 'skill_slug' é obrigatório para o prompt 'executar-skill'")
	}

	inputJSON := args["input_json"]
	if inputJSON == "" {
		return nil, fmt.Errorf("argumento 'input_json' é obrigatório para o prompt 'executar-skill'")
	}

	mensagem := fmt.Sprintf(
		"Execute a skill '%s' com os seguintes parâmetros:\n\n%s\n\nUse a ferramenta correspondente no AgentHub e retorne o resultado da execução.",
		skillSlug,
		inputJSON,
	)

	return &mcp.ResultadoObterPrompt{
		Descricao: fmt.Sprintf("Execução da skill '%s' via AgentHub", skillSlug),
		Mensagens: []mcp.MensagemPrompt{
			{
				Papel: "user",
				Conteudo: mcp.ItemConteudo{
					Tipo:  "text",
					Texto: mensagem,
				},
			},
		},
	}, nil
}
