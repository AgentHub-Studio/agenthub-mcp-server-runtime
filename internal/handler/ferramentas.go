// Pacote handler implementa os handlers MCP para ferramentas, recursos e prompts.
// Este arquivo contém o handler de ferramentas (tools), que expõe as skills
// ativas do AgentHub como MCP Tools.
package handler

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/AgentHub-Studio/agenthub-mcp-server-runtime/internal/backend"
	"github.com/AgentHub-Studio/agenthub-mcp-server-runtime/internal/mcp"
	skillruntime "github.com/AgentHub-Studio/agenthub-mcp-server-runtime/internal/skill_runtime"
)

// InterfaceClienteBackendFerramentas define as operações de backend usadas pelo handler de ferramentas.
type InterfaceClienteBackendFerramentas interface {
	ListarSkillsAtivas(ctx context.Context) ([]backend.SkillDTO, error)
}

// InterfaceClienteSkillRuntime define as operações do skill-runtime usadas pelo handler de ferramentas.
type InterfaceClienteSkillRuntime interface {
	InvocarSkill(ctx context.Context, req skillruntime.RequisicaoInvocarSkill) (*skillruntime.ResultadoSkill, error)
}

// HandlerFerramentas implementa a interface GerenciadorFerramentas do MCPServidor.
// Converte skills do AgentHub para o formato MCP Tool e delega execuções ao skill-runtime.
type HandlerFerramentas struct {
	clienteBackend      InterfaceClienteBackendFerramentas
	clienteSkillRuntime InterfaceClienteSkillRuntime
	tenantID            string
}

// NovoHandlerFerramentas cria um novo handler de ferramentas MCP.
func NovoHandlerFerramentas(
	clienteBackend InterfaceClienteBackendFerramentas,
	clienteSkillRuntime InterfaceClienteSkillRuntime,
	tenantID string,
) *HandlerFerramentas {
	return &HandlerFerramentas{
		clienteBackend:      clienteBackend,
		clienteSkillRuntime: clienteSkillRuntime,
		tenantID:            tenantID,
	}
}

// ListarFerramentas consulta as skills ativas do tenant e as converte para MCP Tools.
// Cada skill é exposta com name=slug, description e inputSchema original.
func (h *HandlerFerramentas) ListarFerramentas(ctx context.Context) ([]mcp.Ferramenta, error) {
	skills, err := h.clienteBackend.ListarSkillsAtivas(ctx)
	if err != nil {
		return nil, fmt.Errorf("erro ao buscar skills do backend: %w", err)
	}

	ferramentas := make([]mcp.Ferramenta, 0, len(skills))
	for _, skill := range skills {
		ferramenta := converterSkillParaFerramenta(skill)
		ferramentas = append(ferramentas, ferramenta)
	}

	return ferramentas, nil
}

// ChamarFerramenta invoca uma skill pelo slug com os argumentos fornecidos.
// Delega a execução ao agenthub-skill-runtime e retorna o resultado como conteúdo MCP.
func (h *HandlerFerramentas) ChamarFerramenta(
	ctx context.Context,
	nome string,
	argumentos map[string]interface{},
) (*mcp.ResultadoChamarFerramenta, error) {
	if nome == "" {
		return nil, fmt.Errorf("nome da ferramenta é obrigatório")
	}

	req := skillruntime.RequisicaoInvocarSkill{
		TenantID:  h.tenantID,
		SkillSlug: nome,
		Input:     argumentos,
	}

	resultado, err := h.clienteSkillRuntime.InvocarSkill(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("erro ao invocar skill '%s': %w", nome, err)
	}

	// Serializar o resultado como JSON para o conteúdo de texto
	resultadoJSON, err := json.Marshal(resultado.Resultado)
	if err != nil {
		return nil, fmt.Errorf("erro ao serializar resultado da skill: %w", err)
	}

	return &mcp.ResultadoChamarFerramenta{
		EhErro: !resultado.Sucesso,
		Conteudo: []mcp.ItemConteudo{
			{Tipo: "text", Texto: string(resultadoJSON)},
		},
	}, nil
}

// converterSkillParaFerramenta converte um SkillDTO do backend para uma Ferramenta MCP.
// O slug da skill se torna o nome da ferramenta para identificação única.
func converterSkillParaFerramenta(skill backend.SkillDTO) mcp.Ferramenta {
	// Schema padrão caso a skill não tenha um definido
	inputSchema := skill.InputSchema
	if inputSchema == nil {
		inputSchema = map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		}
	}

	return mcp.Ferramenta{
		Nome:        skill.Slug,
		Descricao:   skill.Descricao,
		InputSchema: inputSchema,
	}
}
