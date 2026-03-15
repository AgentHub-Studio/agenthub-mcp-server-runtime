package mcp

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// ========== Fakes para teste ==========

// fakeDispatcher records received requests for verification in tests.
type fakeDispatcher struct {
	requests []*JSONRPCRequest
	response *JSONRPCResponse
}

func (d *fakeDispatcher) dispatch(req *JSONRPCRequest) *JSONRPCResponse {
	d.requests = append(d.requests, req)
	return d.response
}

// ========== Testes do loop de protocolo ==========

func TestProtocol_DeveLerEProcessarInicializacao(t *testing.T) {
	// Prepare initialization request
	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "initialize",
		Params:  json.RawMessage(`{"protocolVersion":"2024-11-05"}`),
	}
	reqJSON, _ := json.Marshal(req)

	input := bytes.NewReader(append(reqJSON, '\n'))
	output := &bytes.Buffer{}

	expectedResponse := &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      float64(1),
		Result:  json.RawMessage(`{"ok":true}`),
	}

	fake := &fakeDispatcher{response: expectedResponse}
	protocol := NewProtocol(input, output, fake.dispatch)

	err := protocol.StartLoop()

	if err != nil {
		t.Fatalf("StartLoop retornou erro inesperado: %v", err)
	}

	if len(fake.requests) != 1 {
		t.Fatalf("esperava 1 requisição processada, obteve %d", len(fake.requests))
	}

	if fake.requests[0].Method != "initialize" {
		t.Errorf("esperava método 'initialize', obteve '%s'", fake.requests[0].Method)
	}

	// Verify that the response was written to stdout
	outputStr := output.String()
	if !strings.Contains(outputStr, `"jsonrpc":"2.0"`) {
		t.Errorf("resposta não contém versão JSON-RPC: %s", outputStr)
	}
}

func TestProtocol_NaoDeveResponderNotificacao(t *testing.T) {
	// Notifications have no ID and the dispatcher returns nil
	req := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "notifications/initialized",
	}
	reqJSON, _ := json.Marshal(req)

	input := bytes.NewReader(append(reqJSON, '\n'))
	output := &bytes.Buffer{}

	// The dispatcher returns nil for notifications
	fake := &fakeDispatcher{response: nil}
	protocol := NewProtocol(input, output, fake.dispatch)

	err := protocol.StartLoop()

	if err != nil {
		t.Fatalf("StartLoop retornou erro inesperado: %v", err)
	}

	// No response should be written
	if output.Len() != 0 {
		t.Errorf("não deveria ter escrito resposta para notificação, obteve: %s", output.String())
	}

	if len(fake.requests) != 1 {
		t.Fatalf("esperava 1 requisição processada, obteve %d", len(fake.requests))
	}
}

func TestProtocol_DeveRetornarErroParaJSONInvalido(t *testing.T) {
	// Malformed JSON should generate a parse error
	input := bytes.NewReader([]byte("isto nao e json valido\n"))
	output := &bytes.Buffer{}

	fake := &fakeDispatcher{response: nil}
	protocol := NewProtocol(input, output, fake.dispatch)

	err := protocol.StartLoop()

	if err != nil {
		t.Fatalf("StartLoop retornou erro inesperado: %v", err)
	}

	// The dispatcher should not have been called
	if len(fake.requests) != 0 {
		t.Errorf("despachante não deveria ter sido chamado para JSON inválido")
	}

	// Should have written an error response
	outputStr := output.String()
	if !strings.Contains(outputStr, `"code"`) {
		t.Errorf("esperava resposta de erro com 'code', obteve: %s", outputStr)
	}
}

func TestProtocol_DeveProcessarMultiplasRequisicoes(t *testing.T) {
	// Prepare three consecutive requests
	req1, _ := json.Marshal(JSONRPCRequest{JSONRPC: "2.0", ID: 1, Method: "tools/list"})
	req2, _ := json.Marshal(JSONRPCRequest{JSONRPC: "2.0", ID: 2, Method: "resources/list"})
	req3, _ := json.Marshal(JSONRPCRequest{JSONRPC: "2.0", ID: 3, Method: "prompts/list"})

	inputStr := string(req1) + "\n" + string(req2) + "\n" + string(req3) + "\n"
	input := strings.NewReader(inputStr)
	output := &bytes.Buffer{}

	genericResponse := &JSONRPCResponse{
		JSONRPC: "2.0",
		Result:  json.RawMessage(`{}`),
	}

	fake := &fakeDispatcher{response: genericResponse}
	protocol := NewProtocol(input, output, fake.dispatch)

	err := protocol.StartLoop()

	if err != nil {
		t.Fatalf("StartLoop retornou erro inesperado: %v", err)
	}

	if len(fake.requests) != 3 {
		t.Errorf("esperava 3 requisições processadas, obteve %d", len(fake.requests))
	}
}
