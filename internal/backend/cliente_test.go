package backend

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// ========== Testes de ClienteBackend ==========

func TestClienteBackend_DeveEnviarHeaderTenantID(t *testing.T) {
	tenantIDRecebido := ""

	servidor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantIDRecebido = r.Header.Get("X-Tenant-ID")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"content": []SkillDTO{},
		})
	}))
	defer servidor.Close()

	cliente := NovoClienteBackend(servidor.URL, "tenant-abc-123", "token-teste")

	_, err := cliente.ListarSkillsAtivas(context.Background())

	if err != nil {
		t.Fatalf("ListarSkillsAtivas retornou erro inesperado: %v", err)
	}

	if tenantIDRecebido != "tenant-abc-123" {
		t.Errorf("esperava X-Tenant-ID 'tenant-abc-123', obteve '%s'", tenantIDRecebido)
	}
}

func TestClienteBackend_DeveEnviarHeaderAuthorization(t *testing.T) {
	authRecebido := ""

	servidor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authRecebido = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"content": []SkillDTO{}})
	}))
	defer servidor.Close()

	cliente := NovoClienteBackend(servidor.URL, "tenant-123", "meu-token-secreto")

	_, err := cliente.ListarSkillsAtivas(context.Background())

	if err != nil {
		t.Fatalf("ListarSkillsAtivas retornou erro inesperado: %v", err)
	}

	if authRecebido != "Bearer meu-token-secreto" {
		t.Errorf("esperava 'Bearer meu-token-secreto', obteve '%s'", authRecebido)
	}
}

func TestClienteBackend_DeveDeserializarListaDeSkills(t *testing.T) {
	skillsRetornadas := []SkillDTO{
		{
			ID:        "id-1",
			Nome:      "Busca de Documentos",
			Slug:      "document-search",
			Descricao: "Busca semântica",
			Status:    "ACTIVE",
		},
		{
			ID:    "id-2",
			Nome:  "Consulta SQL",
			Slug:  "sql-query",
			Status: "ACTIVE",
		},
	}

	servidor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"content": skillsRetornadas,
		})
	}))
	defer servidor.Close()

	cliente := NovoClienteBackend(servidor.URL, "tenant-123", "")

	skills, err := cliente.ListarSkillsAtivas(context.Background())

	if err != nil {
		t.Fatalf("ListarSkillsAtivas retornou erro inesperado: %v", err)
	}

	if len(skills) != 2 {
		t.Fatalf("esperava 2 skills, obteve %d", len(skills))
	}

	if skills[0].Slug != "document-search" {
		t.Errorf("esperava slug 'document-search', obteve '%s'", skills[0].Slug)
	}
}

func TestClienteBackend_DeveRetornarErroEmStatus4xx(t *testing.T) {
	servidor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "não autorizado", http.StatusUnauthorized)
	}))
	defer servidor.Close()

	cliente := NovoClienteBackend(servidor.URL, "tenant-123", "token-invalido")

	_, err := cliente.ListarSkillsAtivas(context.Background())

	if err == nil {
		t.Fatal("esperava erro para resposta 401 do backend")
	}
}

func TestClienteBackend_DeveRetornarErroEmStatus5xx(t *testing.T) {
	servidor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "erro interno", http.StatusInternalServerError)
	}))
	defer servidor.Close()

	cliente := NovoClienteBackend(servidor.URL, "tenant-123", "")

	_, err := cliente.ListarSkillsAtivas(context.Background())

	if err == nil {
		t.Fatal("esperava erro para resposta 500 do backend")
	}
}

func TestClienteBackend_DeveListarKnowledgeBases(t *testing.T) {
	kbsRetornadas := []KnowledgeBaseDTO{
		{
			ID:        "kb-id-1",
			Nome:      "Documentação Técnica",
			Descricao: "Docs técnicos",
			Status:    "ACTIVE",
		},
	}

	servidor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"content": kbsRetornadas,
		})
	}))
	defer servidor.Close()

	cliente := NovoClienteBackend(servidor.URL, "tenant-123", "")

	kbs, err := cliente.ListarKnowledgeBases(context.Background())

	if err != nil {
		t.Fatalf("ListarKnowledgeBases retornou erro inesperado: %v", err)
	}

	if len(kbs) != 1 {
		t.Fatalf("esperava 1 knowledge base, obteve %d", len(kbs))
	}

	if kbs[0].Nome != "Documentação Técnica" {
		t.Errorf("esperava nome 'Documentação Técnica', obteve '%s'", kbs[0].Nome)
	}
}

func TestClienteBackend_DeveLerKnowledgeBaseEspecifica(t *testing.T) {
	idCapturado := ""

	servidor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		idCapturado = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(KnowledgeBaseDTO{
			ID:     "kb-uuid-123",
			Nome:   "Base de Teste",
			Status: "ACTIVE",
		})
	}))
	defer servidor.Close()

	cliente := NovoClienteBackend(servidor.URL, "tenant-123", "")

	kb, err := cliente.LerKnowledgeBase(context.Background(), "kb-uuid-123")

	if err != nil {
		t.Fatalf("LerKnowledgeBase retornou erro inesperado: %v", err)
	}

	if kb.ID != "kb-uuid-123" {
		t.Errorf("esperava ID 'kb-uuid-123', obteve '%s'", kb.ID)
	}

	if idCapturado != "/api/knowledge-bases/kb-uuid-123" {
		t.Errorf("URL incorreta chamada: %s", idCapturado)
	}
}

func TestClienteBackend_DeveRetornarErroParaKBNaoEncontrada(t *testing.T) {
	servidor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer servidor.Close()

	cliente := NovoClienteBackend(servidor.URL, "tenant-123", "")

	_, err := cliente.LerKnowledgeBase(context.Background(), "kb-inexistente")

	if err == nil {
		t.Fatal("esperava erro para knowledge base não encontrada (404)")
	}
}
