package backend

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/AgentHub-Studio/agenthub-mcp-server-runtime/internal/tenant"
)

// ========== Testes de BackendClient ==========

func TestBackendClient_DeveEnviarHeaderTenantID(t *testing.T) {
	receivedTenantID := ""

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedTenantID = r.Header.Get("X-Tenant-ID")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"content": []SkillDTO{},
		})
	}))
	defer server.Close()

	client := NewBackendClient(server.URL, NewStaticTokenProvider("token-teste"))

	ctx := tenant.WithTenant(context.Background(), "tenant-abc-123", "token-teste")
	_, err := client.ListActiveSkills(ctx)

	if err != nil {
		t.Fatalf("ListActiveSkills retornou erro inesperado: %v", err)
	}

	if receivedTenantID != "tenant-abc-123" {
		t.Errorf("esperava X-Tenant-ID 'tenant-abc-123', obteve '%s'", receivedTenantID)
	}
}

func TestBackendClient_DeveEnviarHeaderAuthorization(t *testing.T) {
	receivedAuth := ""

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"content": []SkillDTO{}})
	}))
	defer server.Close()

	client := NewBackendClient(server.URL, NewStaticTokenProvider("meu-token-secreto"))

	_, err := client.ListActiveSkills(context.Background())

	if err != nil {
		t.Fatalf("ListActiveSkills retornou erro inesperado: %v", err)
	}

	if receivedAuth != "Bearer meu-token-secreto" {
		t.Errorf("esperava 'Bearer meu-token-secreto', obteve '%s'", receivedAuth)
	}
}

func TestBackendClient_DeveDeserializarListaDeSkills(t *testing.T) {
	returnedSkills := []SkillDTO{
		{
			ID:          "id-1",
			Name:        "Busca de Documentos",
			Slug:        "document-search",
			Description: "Busca semântica",
			Status:      "ACTIVE",
		},
		{
			ID:     "id-2",
			Name:   "Consulta SQL",
			Slug:   "sql-query",
			Status: "ACTIVE",
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"content": returnedSkills,
		})
	}))
	defer server.Close()

	client := NewBackendClient(server.URL, nil)

	skills, err := client.ListActiveSkills(context.Background())

	if err != nil {
		t.Fatalf("ListActiveSkills retornou erro inesperado: %v", err)
	}

	if len(skills) != 2 {
		t.Fatalf("esperava 2 skills, obteve %d", len(skills))
	}

	if skills[0].Slug != "document-search" {
		t.Errorf("esperava slug 'document-search', obteve '%s'", skills[0].Slug)
	}
}

func TestBackendClient_DeveRetornarErroEmStatus4xx(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "não autorizado", http.StatusUnauthorized)
	}))
	defer server.Close()

	client := NewBackendClient(server.URL, NewStaticTokenProvider("token-invalido"))

	_, err := client.ListActiveSkills(context.Background())

	if err == nil {
		t.Fatal("esperava erro para resposta 401 do backend")
	}
}

func TestBackendClient_DeveRetornarErroEmStatus5xx(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "erro interno", http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewBackendClient(server.URL, nil)

	_, err := client.ListActiveSkills(context.Background())

	if err == nil {
		t.Fatal("esperava erro para resposta 500 do backend")
	}
}

func TestBackendClient_DeveListarKnowledgeBases(t *testing.T) {
	returnedKBs := []KnowledgeBaseDTO{
		{
			ID:          "kb-id-1",
			Name:        "Documentação Técnica",
			Description: "Docs técnicos",
			Status:      "ACTIVE",
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"content": returnedKBs,
		})
	}))
	defer server.Close()

	client := NewBackendClient(server.URL, nil)

	kbs, err := client.ListKnowledgeBases(context.Background())

	if err != nil {
		t.Fatalf("ListKnowledgeBases retornou erro inesperado: %v", err)
	}

	if len(kbs) != 1 {
		t.Fatalf("esperava 1 knowledge base, obteve %d", len(kbs))
	}

	if kbs[0].Name != "Documentação Técnica" {
		t.Errorf("esperava nome 'Documentação Técnica', obteve '%s'", kbs[0].Name)
	}
}

func TestBackendClient_DeveLerKnowledgeBaseEspecifica(t *testing.T) {
	capturedID := ""

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedID = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(KnowledgeBaseDTO{
			ID:     "kb-uuid-123",
			Name:   "Base de Teste",
			Status: "ACTIVE",
		})
	}))
	defer server.Close()

	client := NewBackendClient(server.URL, nil)

	kb, err := client.GetKnowledgeBase(context.Background(), "kb-uuid-123")

	if err != nil {
		t.Fatalf("GetKnowledgeBase retornou erro inesperado: %v", err)
	}

	if kb.ID != "kb-uuid-123" {
		t.Errorf("esperava ID 'kb-uuid-123', obteve '%s'", kb.ID)
	}

	if capturedID != "/api/knowledge-bases/kb-uuid-123" {
		t.Errorf("URL incorreta chamada: %s", capturedID)
	}
}

func TestBackendClient_DeveRetornarErroParaKBNaoEncontrada(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer server.Close()

	client := NewBackendClient(server.URL, nil)

	_, err := client.GetKnowledgeBase(context.Background(), "kb-inexistente")

	if err == nil {
		t.Fatal("esperava erro para knowledge base não encontrada (404)")
	}
}

func TestBackendClient_DeveBuscarRegistryPelaRotaCanonica(t *testing.T) {
	calledPath := ""
	calledQuery := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calledPath = r.URL.Path
		calledQuery = r.URL.Query().Get("q")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"content": []PackageSearchDTO{{ID: "pkg-1", Slug: "document-search"}},
		})
	}))
	defer server.Close()

	client := NewBackendClient(server.URL, nil)
	packages, err := client.SearchPackages(context.Background(), "document search & retrieval", "SKILL")

	if err != nil {
		t.Fatalf("SearchPackages retornou erro inesperado: %v", err)
	}
	if calledPath != "/api/registry/search" {
		t.Errorf("rota incorreta chamada: %s", calledPath)
	}
	if calledQuery != "document search & retrieval" {
		t.Errorf("query incorreta chamada: %q", calledQuery)
	}
	if len(packages) != 1 || packages[0].Slug != "document-search" {
		t.Errorf("resultado inesperado: %#v", packages)
	}
}
