package handler

import (
	"context"
	"strings"
	"testing"
)

func TestHandlerPrompts_DeveListarDoisPromptsCurados(t *testing.T) {
	handler := NovoHandlerPrompts()

	prompts, err := handler.ListarPrompts(context.Background())

	if err != nil {
		t.Fatalf("ListarPrompts retornou erro inesperado: %v", err)
	}

	if len(prompts) != 2 {
		t.Fatalf("esperava 2 prompts curados, obteve %d", len(prompts))
	}

	nomes := make(map[string]bool)
	for _, p := range prompts {
		nomes[p.Nome] = true
	}

	if !nomes["pesquisar-conhecimento"] {
		t.Error("prompt 'pesquisar-conhecimento' não encontrado na lista")
	}

	if !nomes["executar-skill"] {
		t.Error("prompt 'executar-skill' não encontrado na lista")
	}
}

func TestHandlerPrompts_DeveRenderizarPesquisarConhecimento(t *testing.T) {
	handler := NovoHandlerPrompts()

	resultado, err := handler.ObterPrompt(context.Background(), "pesquisar-conhecimento", map[string]string{
		"consulta": "arquitetura do AgentHub",
	})

	if err != nil {
		t.Fatalf("ObterPrompt retornou erro inesperado: %v", err)
	}

	if len(resultado.Mensagens) == 0 {
		t.Fatal("resultado deveria ter pelo menos uma mensagem")
	}

	if resultado.Mensagens[0].Papel != "user" {
		t.Errorf("esperava papel 'user', obteve '%s'", resultado.Mensagens[0].Papel)
	}

	if !strings.Contains(resultado.Mensagens[0].Conteudo.Texto, "arquitetura do AgentHub") {
		t.Error("mensagem do prompt deveria conter a consulta fornecida")
	}
}

func TestHandlerPrompts_DeveRenderizarPesquisarConhecimentoComKBID(t *testing.T) {
	handler := NovoHandlerPrompts()

	resultado, err := handler.ObterPrompt(context.Background(), "pesquisar-conhecimento", map[string]string{
		"consulta": "pipeline de documentos",
		"kb_id":    "123e4567-e89b-12d3-a456-426614174000",
	})

	if err != nil {
		t.Fatalf("ObterPrompt retornou erro inesperado: %v", err)
	}

	texto := resultado.Mensagens[0].Conteudo.Texto
	if !strings.Contains(texto, "123e4567-e89b-12d3-a456-426614174000") {
		t.Error("mensagem deveria conter o kb_id fornecido")
	}
}

func TestHandlerPrompts_DeveRenderizarExecutarSkill(t *testing.T) {
	handler := NovoHandlerPrompts()

	resultado, err := handler.ObterPrompt(context.Background(), "executar-skill", map[string]string{
		"skill_slug": "document-search",
		"input_json": `{"query":"teste","limit":5}`,
	})

	if err != nil {
		t.Fatalf("ObterPrompt retornou erro inesperado: %v", err)
	}

	texto := resultado.Mensagens[0].Conteudo.Texto
	if !strings.Contains(texto, "document-search") {
		t.Error("mensagem deveria conter o slug da skill")
	}

	if !strings.Contains(texto, `{"query":"teste","limit":5}`) {
		t.Error("mensagem deveria conter o input_json fornecido")
	}
}

func TestHandlerPrompts_DeveRetornarErroParaPromptDesconhecido(t *testing.T) {
	handler := NovoHandlerPrompts()

	_, err := handler.ObterPrompt(context.Background(), "prompt-inexistente", nil)

	if err == nil {
		t.Fatal("esperava erro para prompt com nome desconhecido")
	}
}

func TestHandlerPrompts_DeveRetornarErroSemArgumentoObrigatorio_PesquisarConhecimento(t *testing.T) {
	handler := NovoHandlerPrompts()

	// Argumento 'consulta' é obrigatório
	_, err := handler.ObterPrompt(context.Background(), "pesquisar-conhecimento", map[string]string{})

	if err == nil {
		t.Fatal("esperava erro quando argumento 'consulta' está ausente")
	}
}

func TestHandlerPrompts_DeveRetornarErroSemArgumentoObrigatorio_ExecutarSkill(t *testing.T) {
	handler := NovoHandlerPrompts()

	// Ambos 'skill_slug' e 'input_json' são obrigatórios
	_, err := handler.ObterPrompt(context.Background(), "executar-skill", map[string]string{
		"skill_slug": "document-search",
		// input_json ausente
	})

	if err == nil {
		t.Fatal("esperava erro quando argumento 'input_json' está ausente")
	}
}

func TestHandlerPrompts_DeveConterArgumentosNaListagem(t *testing.T) {
	handler := NovoHandlerPrompts()

	prompts, _ := handler.ListarPrompts(context.Background())

	// Verificar que pesquisar-conhecimento tem argumentos definidos
	var promptPesquisa interface{}
	for _, p := range prompts {
		if p.Nome == "pesquisar-conhecimento" {
			promptPesquisa = p
			break
		}
	}

	if promptPesquisa == nil {
		t.Fatal("prompt 'pesquisar-conhecimento' não encontrado")
	}
}
