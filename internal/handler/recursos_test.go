package handler

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/AgentHub-Studio/agenthub-mcp-server-runtime/internal/backend"
)

// ========== Fakes para teste ==========

// fakeClienteBackendRecursos simula o cliente backend para testes de recursos.
type fakeClienteBackendRecursos struct {
	kbs        []backend.KnowledgeBaseDTO
	kb         *backend.KnowledgeBaseDTO
	erroListar error
	erroLer    error
}

func (f *fakeClienteBackendRecursos) ListarKnowledgeBases(ctx context.Context) ([]backend.KnowledgeBaseDTO, error) {
	return f.kbs, f.erroListar
}

func (f *fakeClienteBackendRecursos) LerKnowledgeBase(ctx context.Context, kbID string) (*backend.KnowledgeBaseDTO, error) {
	return f.kb, f.erroLer
}

// ========== Testes ==========

func TestHandlerRecursos_DeveListarKBsComoRecursos(t *testing.T) {
	kbs := []backend.KnowledgeBaseDTO{
		{
			ID:        "123e4567-e89b-12d3-a456-426614174000",
			Nome:      "Documentação Técnica",
			Descricao: "Base de documentação do AgentHub",
			Status:    "ACTIVE",
		},
		{
			ID:        "987fcdeb-51a2-43f7-9012-abcdef012345",
			Nome:      "FAQ Clientes",
			Descricao: "Perguntas frequentes",
			Status:    "ACTIVE",
		},
	}

	handler := NovoHandlerRecursos(&fakeClienteBackendRecursos{kbs: kbs})

	recursos, err := handler.ListarRecursos(context.Background())

	if err != nil {
		t.Fatalf("ListarRecursos retornou erro inesperado: %v", err)
	}

	if len(recursos) != 2 {
		t.Fatalf("esperava 2 recursos, obteve %d", len(recursos))
	}

	// Verificar formato do URI
	uriEsperado := "agenthub://kb/123e4567-e89b-12d3-a456-426614174000"
	if recursos[0].URI != uriEsperado {
		t.Errorf("esperava URI '%s', obteve '%s'", uriEsperado, recursos[0].URI)
	}

	if recursos[0].Nome != "Documentação Técnica" {
		t.Errorf("esperava nome 'Documentação Técnica', obteve '%s'", recursos[0].Nome)
	}

	if recursos[0].TipoMIME != "application/json" {
		t.Errorf("esperava TipoMIME 'application/json', obteve '%s'", recursos[0].TipoMIME)
	}
}

func TestHandlerRecursos_DeveRetornarErroDoBackendAoListar(t *testing.T) {
	erroBackend := errors.New("backend indisponível")

	handler := NovoHandlerRecursos(&fakeClienteBackendRecursos{erroListar: erroBackend})

	_, err := handler.ListarRecursos(context.Background())

	if err == nil {
		t.Fatal("esperava erro ao listar recursos com backend indisponível")
	}
}

func TestHandlerRecursos_DeveLerRecursoPorURI(t *testing.T) {
	kb := &backend.KnowledgeBaseDTO{
		ID:        "123e4567-e89b-12d3-a456-426614174000",
		Nome:      "Documentação Técnica",
		Descricao: "Base de documentação",
		Status:    "ACTIVE",
	}

	handler := NovoHandlerRecursos(&fakeClienteBackendRecursos{kb: kb})

	conteudo, err := handler.LerRecurso(context.Background(), "agenthub://kb/123e4567-e89b-12d3-a456-426614174000")

	if err != nil {
		t.Fatalf("LerRecurso retornou erro inesperado: %v", err)
	}

	if conteudo.URI != "agenthub://kb/123e4567-e89b-12d3-a456-426614174000" {
		t.Errorf("URI do conteúdo incorreto: %s", conteudo.URI)
	}

	if conteudo.TipoMIME != "application/json" {
		t.Errorf("TipoMIME incorreto: %s", conteudo.TipoMIME)
	}

	if !strings.Contains(conteudo.Texto, "Documentação Técnica") {
		t.Errorf("conteúdo deveria conter o nome da KB, obteve: %s", conteudo.Texto)
	}
}

func TestHandlerRecursos_DeveRetornarErroParaURIInvalido(t *testing.T) {
	handler := NovoHandlerRecursos(&fakeClienteBackendRecursos{})

	_, err := handler.LerRecurso(context.Background(), "uri-invalido")

	if err == nil {
		t.Fatal("esperava erro para URI com formato inválido")
	}
}

func TestHandlerRecursos_DeveRetornarErroParaURISemID(t *testing.T) {
	handler := NovoHandlerRecursos(&fakeClienteBackendRecursos{})

	_, err := handler.LerRecurso(context.Background(), "agenthub://kb/")

	if err == nil {
		t.Fatal("esperava erro para URI sem ID da knowledge base")
	}
}

func TestHandlerRecursos_DeveRetornarErroQuandoKBNaoEncontrada(t *testing.T) {
	erroNaoEncontrado := errors.New("knowledge base não encontrada: uuid-inexistente")

	handler := NovoHandlerRecursos(&fakeClienteBackendRecursos{erroLer: erroNaoEncontrado})

	_, err := handler.LerRecurso(context.Background(), "agenthub://kb/uuid-inexistente")

	if err == nil {
		t.Fatal("esperava erro quando knowledge base não é encontrada")
	}
}
