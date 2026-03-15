package handler

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/AgentHub-Studio/agenthub-mcp-server-runtime/internal/backend"
)

// ========== Fakes para teste ==========

// fakeBackendClientResources simulates the backend client for resources tests.
type fakeBackendClientResources struct {
	kbs       []backend.KnowledgeBaseDTO
	kb        *backend.KnowledgeBaseDTO
	errList   error
	errGet    error
}

func (f *fakeBackendClientResources) ListKnowledgeBases(ctx context.Context) ([]backend.KnowledgeBaseDTO, error) {
	return f.kbs, f.errList
}

func (f *fakeBackendClientResources) GetKnowledgeBase(ctx context.Context, kbID string) (*backend.KnowledgeBaseDTO, error) {
	return f.kb, f.errGet
}

// ========== Testes ==========

func TestResourcesHandler_DeveListarKBsComoRecursos(t *testing.T) {
	kbs := []backend.KnowledgeBaseDTO{
		{
			ID:          "123e4567-e89b-12d3-a456-426614174000",
			Name:        "Documentação Técnica",
			Description: "Base de documentação do AgentHub",
			Status:      "ACTIVE",
		},
		{
			ID:          "987fcdeb-51a2-43f7-9012-abcdef012345",
			Name:        "FAQ Clientes",
			Description: "Perguntas frequentes",
			Status:      "ACTIVE",
		},
	}

	handler := NewResourcesHandler(&fakeBackendClientResources{kbs: kbs})

	resources, err := handler.ListResources(context.Background())

	if err != nil {
		t.Fatalf("ListResources retornou erro inesperado: %v", err)
	}

	if len(resources) != 2 {
		t.Fatalf("esperava 2 recursos, obteve %d", len(resources))
	}

	// Verify URI format
	expectedURI := "agenthub://kb/123e4567-e89b-12d3-a456-426614174000"
	if resources[0].URI != expectedURI {
		t.Errorf("esperava URI '%s', obteve '%s'", expectedURI, resources[0].URI)
	}

	if resources[0].Name != "Documentação Técnica" {
		t.Errorf("esperava nome 'Documentação Técnica', obteve '%s'", resources[0].Name)
	}

	if resources[0].MIMEType != "application/json" {
		t.Errorf("esperava MIMEType 'application/json', obteve '%s'", resources[0].MIMEType)
	}
}

func TestResourcesHandler_DeveRetornarErroDoBackendAoListar(t *testing.T) {
	backendErr := errors.New("backend indisponível")

	handler := NewResourcesHandler(&fakeBackendClientResources{errList: backendErr})

	_, err := handler.ListResources(context.Background())

	if err == nil {
		t.Fatal("esperava erro ao listar recursos com backend indisponível")
	}
}

func TestResourcesHandler_DeveLerRecursoPorURI(t *testing.T) {
	kb := &backend.KnowledgeBaseDTO{
		ID:          "123e4567-e89b-12d3-a456-426614174000",
		Name:        "Documentação Técnica",
		Description: "Base de documentação",
		Status:      "ACTIVE",
	}

	handler := NewResourcesHandler(&fakeBackendClientResources{kb: kb})

	content, err := handler.ReadResource(context.Background(), "agenthub://kb/123e4567-e89b-12d3-a456-426614174000")

	if err != nil {
		t.Fatalf("ReadResource retornou erro inesperado: %v", err)
	}

	if content.URI != "agenthub://kb/123e4567-e89b-12d3-a456-426614174000" {
		t.Errorf("URI do conteúdo incorreto: %s", content.URI)
	}

	if content.MIMEType != "application/json" {
		t.Errorf("MIMEType incorreto: %s", content.MIMEType)
	}

	if !strings.Contains(content.Text, "Documentação Técnica") {
		t.Errorf("conteúdo deveria conter o nome da KB, obteve: %s", content.Text)
	}
}

func TestResourcesHandler_DeveRetornarErroParaURIInvalido(t *testing.T) {
	handler := NewResourcesHandler(&fakeBackendClientResources{})

	_, err := handler.ReadResource(context.Background(), "uri-invalido")

	if err == nil {
		t.Fatal("esperava erro para URI com formato inválido")
	}
}

func TestResourcesHandler_DeveRetornarErroParaURISemID(t *testing.T) {
	handler := NewResourcesHandler(&fakeBackendClientResources{})

	_, err := handler.ReadResource(context.Background(), "agenthub://kb/")

	if err == nil {
		t.Fatal("esperava erro para URI sem ID da knowledge base")
	}
}

func TestResourcesHandler_DeveRetornarErroQuandoKBNaoEncontrada(t *testing.T) {
	notFoundErr := errors.New("knowledge base não encontrada: uuid-inexistente")

	handler := NewResourcesHandler(&fakeBackendClientResources{errGet: notFoundErr})

	_, err := handler.ReadResource(context.Background(), "agenthub://kb/uuid-inexistente")

	if err == nil {
		t.Fatal("esperava erro quando knowledge base não é encontrada")
	}
}
