package mcp

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// ========== Fakes para teste ==========

// despachanteFake registra as requisições recebidas para verificação nos testes.
type despachanteFake struct {
	requisicoes []*RequisicaoJSONRPC
	resposta    *RespostaJSONRPC
}

func (d *despachanteFake) processar(req *RequisicaoJSONRPC) *RespostaJSONRPC {
	d.requisicoes = append(d.requisicoes, req)
	return d.resposta
}

// ========== Testes do loop de protocolo ==========

func TestProtocolo_DeveLerEProcessarInicializacao(t *testing.T) {
	// Preparar requisição de inicialização
	req := RequisicaoJSONRPC{
		JSONRPC: "2.0",
		ID:      float64(1),
		Metodo:  "initialize",
		Params:  json.RawMessage(`{"protocolVersion":"2024-11-05"}`),
	}
	reqJSON, _ := json.Marshal(req)

	entrada := bytes.NewReader(append(reqJSON, '\n'))
	saida := &bytes.Buffer{}

	respostaEsperada := &RespostaJSONRPC{
		JSONRPC:   "2.0",
		ID:        float64(1),
		Resultado: json.RawMessage(`{"ok":true}`),
	}

	fake := &despachanteFake{resposta: respostaEsperada}
	protocolo := NovoProtocolo(entrada, saida, fake.processar)

	err := protocolo.IniciarLoop()

	if err != nil {
		t.Fatalf("IniciarLoop retornou erro inesperado: %v", err)
	}

	if len(fake.requisicoes) != 1 {
		t.Fatalf("esperava 1 requisição processada, obteve %d", len(fake.requisicoes))
	}

	if fake.requisicoes[0].Metodo != "initialize" {
		t.Errorf("esperava método 'initialize', obteve '%s'", fake.requisicoes[0].Metodo)
	}

	// Verificar que a resposta foi escrita no stdout
	saída := saida.String()
	if !strings.Contains(saída, `"jsonrpc":"2.0"`) {
		t.Errorf("resposta não contém versão JSON-RPC: %s", saída)
	}
}

func TestProtocolo_NaoDeveResponderNotificacao(t *testing.T) {
	// Notificações não têm ID e o despachante retorna nil
	req := RequisicaoJSONRPC{
		JSONRPC: "2.0",
		Metodo:  "notifications/initialized",
	}
	reqJSON, _ := json.Marshal(req)

	entrada := bytes.NewReader(append(reqJSON, '\n'))
	saida := &bytes.Buffer{}

	// O despachante retorna nil para notificações
	fake := &despachanteFake{resposta: nil}
	protocolo := NovoProtocolo(entrada, saida, fake.processar)

	err := protocolo.IniciarLoop()

	if err != nil {
		t.Fatalf("IniciarLoop retornou erro inesperado: %v", err)
	}

	// Nenhuma resposta deve ser escrita
	if saida.Len() != 0 {
		t.Errorf("não deveria ter escrito resposta para notificação, obteve: %s", saida.String())
	}

	if len(fake.requisicoes) != 1 {
		t.Fatalf("esperava 1 requisição processada, obteve %d", len(fake.requisicoes))
	}
}

func TestProtocolo_DeveRetornarErroParaJSONInvalido(t *testing.T) {
	// JSON malformado deve gerar erro de parse
	entrada := bytes.NewReader([]byte("isto nao e json valido\n"))
	saida := &bytes.Buffer{}

	fake := &despachanteFake{resposta: nil}
	protocolo := NovoProtocolo(entrada, saida, fake.processar)

	err := protocolo.IniciarLoop()

	if err != nil {
		t.Fatalf("IniciarLoop retornou erro inesperado: %v", err)
	}

	// O despachante não deve ter sido chamado
	if len(fake.requisicoes) != 0 {
		t.Errorf("despachante não deveria ter sido chamado para JSON inválido")
	}

	// Deve ter escrito uma resposta de erro
	saída := saida.String()
	if !strings.Contains(saída, `"code"`) {
		t.Errorf("esperava resposta de erro com 'code', obteve: %s", saída)
	}
}

func TestProtocolo_DeveProcessarMultiplasRequisicoes(t *testing.T) {
	// Preparar três requisições consecutivas
	req1, _ := json.Marshal(RequisicaoJSONRPC{JSONRPC: "2.0", ID: 1, Metodo: "tools/list"})
	req2, _ := json.Marshal(RequisicaoJSONRPC{JSONRPC: "2.0", ID: 2, Metodo: "resources/list"})
	req3, _ := json.Marshal(RequisicaoJSONRPC{JSONRPC: "2.0", ID: 3, Metodo: "prompts/list"})

	entradaStr := string(req1) + "\n" + string(req2) + "\n" + string(req3) + "\n"
	entrada := strings.NewReader(entradaStr)
	saida := &bytes.Buffer{}

	respostaGenerica := &RespostaJSONRPC{
		JSONRPC:   "2.0",
		Resultado: json.RawMessage(`{}`),
	}

	fake := &despachanteFake{resposta: respostaGenerica}
	protocolo := NovoProtocolo(entrada, saida, fake.processar)

	err := protocolo.IniciarLoop()

	if err != nil {
		t.Fatalf("IniciarLoop retornou erro inesperado: %v", err)
	}

	if len(fake.requisicoes) != 3 {
		t.Errorf("esperava 3 requisições processadas, obteve %d", len(fake.requisicoes))
	}
}
