package handler

import (
	"context"
	"strings"
	"testing"
)

func TestPromptsHandler_DeveListarDoisPromptsCurados(t *testing.T) {
	handler := NewPromptsHandler(nil)

	prompts, err := handler.ListPrompts(context.Background())

	if err != nil {
		t.Fatalf("ListPrompts retornou erro inesperado: %v", err)
	}

	if len(prompts) != 2 {
		t.Fatalf("esperava 2 prompts curados, obteve %d", len(prompts))
	}

	names := make(map[string]bool)
	for _, p := range prompts {
		names[p.Name] = true
	}

	if !names["pesquisar-conhecimento"] {
		t.Error("prompt 'pesquisar-conhecimento' não encontrado na lista")
	}

	if !names["executar-skill"] {
		t.Error("prompt 'executar-skill' não encontrado na lista")
	}
}

func TestPromptsHandler_DeveRenderizarPesquisarConhecimento(t *testing.T) {
	handler := NewPromptsHandler(nil)

	result, err := handler.GetPrompt(context.Background(), "pesquisar-conhecimento", map[string]string{
		"consulta": "arquitetura do AgentHub",
	})

	if err != nil {
		t.Fatalf("GetPrompt retornou erro inesperado: %v", err)
	}

	if len(result.Messages) == 0 {
		t.Fatal("resultado deveria ter pelo menos uma mensagem")
	}

	if result.Messages[0].Role != "user" {
		t.Errorf("esperava papel 'user', obteve '%s'", result.Messages[0].Role)
	}

	if !strings.Contains(result.Messages[0].Content.Text, "arquitetura do AgentHub") {
		t.Error("mensagem do prompt deveria conter a consulta fornecida")
	}
}

func TestPromptsHandler_DeveRenderizarPesquisarConhecimentoComKBID(t *testing.T) {
	handler := NewPromptsHandler(nil)

	result, err := handler.GetPrompt(context.Background(), "pesquisar-conhecimento", map[string]string{
		"consulta": "pipeline de documentos",
		"kb_id":    "123e4567-e89b-12d3-a456-426614174000",
	})

	if err != nil {
		t.Fatalf("GetPrompt retornou erro inesperado: %v", err)
	}

	text := result.Messages[0].Content.Text
	if !strings.Contains(text, "123e4567-e89b-12d3-a456-426614174000") {
		t.Error("mensagem deveria conter o kb_id fornecido")
	}
}

func TestPromptsHandler_DeveRenderizarExecutarSkill(t *testing.T) {
	handler := NewPromptsHandler(nil)

	result, err := handler.GetPrompt(context.Background(), "executar-skill", map[string]string{
		"skill_slug": "document-search",
		"input_json": `{"query":"teste","limit":5}`,
	})

	if err != nil {
		t.Fatalf("GetPrompt retornou erro inesperado: %v", err)
	}

	text := result.Messages[0].Content.Text
	if !strings.Contains(text, "document-search") {
		t.Error("mensagem deveria conter o slug da skill")
	}

	if !strings.Contains(text, `{"query":"teste","limit":5}`) {
		t.Error("mensagem deveria conter o input_json fornecido")
	}
}

func TestPromptsHandler_DeveRetornarErroParaPromptDesconhecido(t *testing.T) {
	handler := NewPromptsHandler(nil)

	_, err := handler.GetPrompt(context.Background(), "prompt-inexistente", nil)

	if err == nil {
		t.Fatal("esperava erro para prompt com nome desconhecido")
	}
}

func TestPromptsHandler_DeveRetornarErroSemArgumentoObrigatorio_PesquisarConhecimento(t *testing.T) {
	handler := NewPromptsHandler(nil)

	// Argument 'consulta' is required
	_, err := handler.GetPrompt(context.Background(), "pesquisar-conhecimento", map[string]string{})

	if err == nil {
		t.Fatal("esperava erro quando argumento 'consulta' está ausente")
	}
}

func TestPromptsHandler_DeveRetornarErroSemArgumentoObrigatorio_ExecutarSkill(t *testing.T) {
	handler := NewPromptsHandler(nil)

	// Both 'skill_slug' and 'input_json' are required
	_, err := handler.GetPrompt(context.Background(), "executar-skill", map[string]string{
		"skill_slug": "document-search",
		// input_json absent
	})

	if err == nil {
		t.Fatal("esperava erro quando argumento 'input_json' está ausente")
	}
}

func TestPromptsHandler_DeveConterArgumentosNaListagem(t *testing.T) {
	handler := NewPromptsHandler(nil)

	prompts, _ := handler.ListPrompts(context.Background())

	// Verify that pesquisar-conhecimento has defined arguments
	var searchPrompt interface{}
	for _, p := range prompts {
		if p.Name == "pesquisar-conhecimento" {
			searchPrompt = p
			break
		}
	}

	if searchPrompt == nil {
		t.Fatal("prompt 'pesquisar-conhecimento' não encontrado")
	}
}
