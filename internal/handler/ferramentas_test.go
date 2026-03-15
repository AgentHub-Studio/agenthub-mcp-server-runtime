package handler

import (
	"context"
	"errors"
	"testing"

	"github.com/AgentHub-Studio/agenthub-mcp-server-runtime/internal/backend"
	skillruntime "github.com/AgentHub-Studio/agenthub-mcp-server-runtime/internal/skill_runtime"
)

// ========== Fakes para teste ==========

// fakeClienteBackendFerramentas simula o cliente backend para testes de ferramentas.
type fakeClienteBackendFerramentas struct {
	skills []backend.SkillDTO
	erro   error
}

func (f *fakeClienteBackendFerramentas) ListarSkillsAtivas(ctx context.Context) ([]backend.SkillDTO, error) {
	return f.skills, f.erro
}

// fakeClienteSkillRuntime simula o cliente skill-runtime para testes.
type fakeClienteSkillRuntime struct {
	resultado *skillruntime.ResultadoSkill
	erro      error
}

func (f *fakeClienteSkillRuntime) InvocarSkill(ctx context.Context, req skillruntime.RequisicaoInvocarSkill) (*skillruntime.ResultadoSkill, error) {
	return f.resultado, f.erro
}

// ========== Testes ==========

func TestHandlerFerramentas_DeveListarSkillsComoFerramentas(t *testing.T) {
	skills := []backend.SkillDTO{
		{
			ID:        "id-1",
			Nome:      "Busca de Documentos",
			Slug:      "document-search",
			Descricao: "Busca semântica em documentos",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{"type": "string"},
				},
				"required": []string{"query"},
			},
		},
		{
			ID:        "id-2",
			Nome:      "Consulta SQL",
			Slug:      "sql-query",
			Descricao: "Executa queries SQL",
		},
	}

	handler := NovoHandlerFerramentas(
		&fakeClienteBackendFerramentas{skills: skills},
		&fakeClienteSkillRuntime{},
		"tenant-123",
	)

	ferramentas, err := handler.ListarFerramentas(context.Background())

	if err != nil {
		t.Fatalf("ListarFerramentas retornou erro inesperado: %v", err)
	}

	if len(ferramentas) != 2 {
		t.Fatalf("esperava 2 ferramentas, obteve %d", len(ferramentas))
	}

	// O nome da ferramenta deve ser o slug da skill
	if ferramentas[0].Nome != "document-search" {
		t.Errorf("esperava nome 'document-search', obteve '%s'", ferramentas[0].Nome)
	}

	if ferramentas[1].Nome != "sql-query" {
		t.Errorf("esperava nome 'sql-query', obteve '%s'", ferramentas[1].Nome)
	}
}

func TestHandlerFerramentas_DeveUsarSchemaParaoCasoDeSkillSemSchema(t *testing.T) {
	skills := []backend.SkillDTO{
		{
			Slug:        "skill-sem-schema",
			InputSchema: nil, // Schema ausente
		},
	}

	handler := NovoHandlerFerramentas(
		&fakeClienteBackendFerramentas{skills: skills},
		&fakeClienteSkillRuntime{},
		"tenant-123",
	)

	ferramentas, err := handler.ListarFerramentas(context.Background())

	if err != nil {
		t.Fatalf("ListarFerramentas retornou erro inesperado: %v", err)
	}

	if ferramentas[0].InputSchema == nil {
		t.Error("InputSchema não deve ser nil mesmo quando skill não tem schema")
	}
}

func TestHandlerFerramentas_DeveRetornarErroDoBackend(t *testing.T) {
	erroBackend := errors.New("backend indisponível")

	handler := NovoHandlerFerramentas(
		&fakeClienteBackendFerramentas{erro: erroBackend},
		&fakeClienteSkillRuntime{},
		"tenant-123",
	)

	_, err := handler.ListarFerramentas(context.Background())

	if err == nil {
		t.Fatal("esperava erro ao listar ferramentas com backend indisponível")
	}
}

func TestHandlerFerramentas_DeveChamarSkillRuntimeAoChamarFerramenta(t *testing.T) {
	resultadoEsperado := &skillruntime.ResultadoSkill{
		SkillSlug: "document-search",
		Sucesso:   true,
		Resultado: map[string]interface{}{"documentos": []interface{}{}},
	}

	handler := NovoHandlerFerramentas(
		&fakeClienteBackendFerramentas{},
		&fakeClienteSkillRuntime{resultado: resultadoEsperado},
		"tenant-123",
	)

	resultado, err := handler.ChamarFerramenta(context.Background(), "document-search", map[string]interface{}{
		"query": "AgentHub architecture",
	})

	if err != nil {
		t.Fatalf("ChamarFerramenta retornou erro inesperado: %v", err)
	}

	if resultado.EhErro {
		t.Error("resultado não deveria ser erro para execução bem-sucedida")
	}

	if len(resultado.Conteudo) == 0 {
		t.Error("resultado deveria ter pelo menos um item de conteúdo")
	}

	if resultado.Conteudo[0].Tipo != "text" {
		t.Errorf("tipo do conteúdo deveria ser 'text', obteve '%s'", resultado.Conteudo[0].Tipo)
	}
}

func TestHandlerFerramentas_DeveRetornarErroParaNomeVazio(t *testing.T) {
	handler := NovoHandlerFerramentas(
		&fakeClienteBackendFerramentas{},
		&fakeClienteSkillRuntime{},
		"tenant-123",
	)

	_, err := handler.ChamarFerramenta(context.Background(), "", nil)

	if err == nil {
		t.Fatal("esperava erro para nome de ferramenta vazio")
	}
}

func TestHandlerFerramentas_DevePropagareErroDoSkillRuntime(t *testing.T) {
	erroRuntime := errors.New("skill não encontrada")

	handler := NovoHandlerFerramentas(
		&fakeClienteBackendFerramentas{},
		&fakeClienteSkillRuntime{erro: erroRuntime},
		"tenant-123",
	)

	_, err := handler.ChamarFerramenta(context.Background(), "skill-inexistente", nil)

	if err == nil {
		t.Fatal("esperava erro quando skill-runtime retorna erro")
	}
}
