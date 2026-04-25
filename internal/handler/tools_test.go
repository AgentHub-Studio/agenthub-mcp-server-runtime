package handler

import (
	"context"
	"errors"
	"testing"

	"github.com/AgentHub-Studio/agenthub-mcp-server-runtime/internal/backend"
	skillruntime "github.com/AgentHub-Studio/agenthub-mcp-server-runtime/internal/skill_runtime"
)

// ========== Fakes para teste ==========

// fakeBackendClientTools simulates the backend client for tools tests.
type fakeBackendClientTools struct {
	skills   []backend.SkillDTO
	tools    map[string][]backend.SkillToolDTO
	err      error
	toolsErr error
}

func (f *fakeBackendClientTools) ListActiveSkills(ctx context.Context) ([]backend.SkillDTO, error) {
	return f.skills, f.err
}

func (f *fakeBackendClientTools) ListSkillTools(ctx context.Context, skillID string) ([]backend.SkillToolDTO, error) {
	if f.toolsErr != nil {
		return nil, f.toolsErr
	}
	if f.tools == nil {
		return nil, nil
	}
	return f.tools[skillID], nil
}

// fakeSkillRuntimeClient simulates the skill-runtime client for tests.
type fakeSkillRuntimeClient struct {
	result *skillruntime.SkillResult
	err    error
}

func (f *fakeSkillRuntimeClient) InvokeSkill(ctx context.Context, req skillruntime.InvokeSkillRequest) (*skillruntime.SkillResult, error) {
	return f.result, f.err
}

// ========== Testes ==========

func TestToolsHandler_DeveListarSkillsComoFerramentas(t *testing.T) {
	skills := []backend.SkillDTO{
		{
			ID:          "id-1",
			Name:        "Busca de Documentos",
			Slug:        "document-search",
			Description: "Busca semântica em documentos",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{"type": "string"},
				},
				"required": []string{"query"},
			},
		},
		{
			ID:          "id-2",
			Name:        "Consulta SQL",
			Slug:        "sql-query",
			Description: "Executa queries SQL",
		},
	}

	handler := NewToolsHandler(
		&fakeBackendClientTools{skills: skills},
		&fakeSkillRuntimeClient{},
	)

	tools, err := handler.ListTools(context.Background())

	if err != nil {
		t.Fatalf("ListTools retornou erro inesperado: %v", err)
	}

	if len(tools) != 2 {
		t.Fatalf("esperava 2 ferramentas, obteve %d", len(tools))
	}

	// Tool name must be the skill slug
	if tools[0].Name != "document-search" {
		t.Errorf("esperava nome 'document-search', obteve '%s'", tools[0].Name)
	}

	if tools[1].Name != "sql-query" {
		t.Errorf("esperava nome 'sql-query', obteve '%s'", tools[1].Name)
	}
}

func TestToolsHandler_DeveUsarSchemaParaoCasoDeSkillSemSchema(t *testing.T) {
	skills := []backend.SkillDTO{
		{
			Slug:        "skill-sem-schema",
			InputSchema: nil, // Schema ausente
		},
	}

	handler := NewToolsHandler(
		&fakeBackendClientTools{skills: skills},
		&fakeSkillRuntimeClient{},
	)

	tools, err := handler.ListTools(context.Background())

	if err != nil {
		t.Fatalf("ListTools retornou erro inesperado: %v", err)
	}

	if tools[0].InputSchema == nil {
		t.Error("InputSchema não deve ser nil mesmo quando skill não tem schema")
	}
}

func TestToolsHandler_DeveUsarInputSchemaDaToolAtiva(t *testing.T) {
	skills := []backend.SkillDTO{
		{
			ID:          "skill-1",
			Slug:        "viacep_address_lookup",
			Description: "Consulta endereço por CEP",
		},
	}
	toolSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"cep": map[string]interface{}{"type": "string"},
		},
		"required": []interface{}{"cep"},
	}

	handler := NewToolsHandler(
		&fakeBackendClientTools{
			skills: skills,
			tools: map[string][]backend.SkillToolDTO{
				"skill-1": {
					{IsActive: true, Tool: backend.ToolDTO{InputSchema: toolSchema}},
				},
			},
		},
		&fakeSkillRuntimeClient{},
	)

	tools, err := handler.ListTools(context.Background())

	if err != nil {
		t.Fatalf("ListTools retornou erro inesperado: %v", err)
	}
	if len(tools) != 1 {
		t.Fatalf("esperava 1 ferramenta, obteve %d", len(tools))
	}
	properties := tools[0].InputSchema["properties"].(map[string]interface{})
	if _, ok := properties["cep"]; !ok {
		t.Fatalf("esperava schema com propriedade cep, obteve %#v", tools[0].InputSchema)
	}
}

func TestToolsHandler_DeveRetornarErroDoBackend(t *testing.T) {
	backendErr := errors.New("backend indisponível")

	handler := NewToolsHandler(
		&fakeBackendClientTools{err: backendErr},
		&fakeSkillRuntimeClient{},
	)

	_, err := handler.ListTools(context.Background())

	if err == nil {
		t.Fatal("esperava erro ao listar ferramentas com backend indisponível")
	}
}

func TestToolsHandler_DeveChamarSkillRuntimeAoChamarFerramenta(t *testing.T) {
	expectedResult := &skillruntime.SkillResult{
		SkillSlug: "document-search",
		Success:   true,
		Result:    map[string]interface{}{"documentos": []interface{}{}},
	}

	handler := NewToolsHandler(
		&fakeBackendClientTools{},
		&fakeSkillRuntimeClient{result: expectedResult},
	)

	result, err := handler.CallTool(context.Background(), "document-search", map[string]interface{}{
		"query": "AgentHub architecture",
	})

	if err != nil {
		t.Fatalf("CallTool retornou erro inesperado: %v", err)
	}

	if result.IsError {
		t.Error("resultado não deveria ser erro para execução bem-sucedida")
	}

	if len(result.Content) == 0 {
		t.Error("resultado deveria ter pelo menos um item de conteúdo")
	}

	if result.Content[0].Type != "text" {
		t.Errorf("tipo do conteúdo deveria ser 'text', obteve '%s'", result.Content[0].Type)
	}
}

func TestToolsHandler_DeveRetornarErroParaNomeVazio(t *testing.T) {
	handler := NewToolsHandler(
		&fakeBackendClientTools{},
		&fakeSkillRuntimeClient{},
	)

	_, err := handler.CallTool(context.Background(), "", nil)

	if err == nil {
		t.Fatal("esperava erro para nome de ferramenta vazio")
	}
}

func TestToolsHandler_DevePropagareErroDoSkillRuntime(t *testing.T) {
	runtimeErr := errors.New("skill não encontrada")

	handler := NewToolsHandler(
		&fakeBackendClientTools{},
		&fakeSkillRuntimeClient{err: runtimeErr},
	)

	_, err := handler.CallTool(context.Background(), "skill-inexistente", nil)

	if err == nil {
		t.Fatal("esperava erro quando skill-runtime retorna erro")
	}
}
