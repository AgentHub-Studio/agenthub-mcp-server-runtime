package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/AgentHub-Studio/agenthub-mcp-server-runtime/internal/mcp"
	"github.com/AgentHub-Studio/agenthub-mcp-server-runtime/internal/oauth"
)

// fakeProcessor is a test double for MessageProcessor.
type fakeProcessor struct {
	response *mcp.JSONRPCResponse
}

func (f *fakeProcessor) HandleHTTPRequest(_ context.Context, _ *mcp.JSONRPCRequest) *mcp.JSONRPCResponse {
	return f.response
}

// buildTestResponse creates a minimal JSON-RPC success response for tests.
func buildTestResponse(t *testing.T, id interface{}) *mcp.JSONRPCResponse {
	t.Helper()
	resp, err := mcp.NewJSONRPCResponse(id, map[string]string{"status": "ok"})
	if err != nil {
		t.Fatalf("erro ao criar resposta de teste: %v", err)
	}
	return resp
}

// postMCP sends a POST /mcp request to the given server with the specified Accept header.
func postMCP(t *testing.T, srv *httptest.Server, body interface{}, accept string) *http.Response {
	t.Helper()
	bodyJSON, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("erro ao serializar body: %v", err)
	}

	req, err := http.NewRequest(http.MethodPost, srv.URL+"/mcp", bytes.NewReader(bodyJSON))
	if err != nil {
		t.Fatalf("erro ao criar request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if accept != "" {
		req.Header.Set("Accept", accept)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("erro ao enviar request: %v", err)
	}
	return resp
}

func TestHTTPServer_DeveRetornarJSONParaAcceptApplicationJSON(t *testing.T) {
	processor := &fakeProcessor{response: buildTestResponse(t, 1)}
	s := NewHTTPServer(0, processor, oauth.NewValidator("", ""), "", "", nil)

	srv := httptest.NewServer(s.router)
	defer srv.Close()

	reqBody := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1, Method: "tools/list"}
	resp := postMCP(t, srv, reqBody, "application/json")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("esperava 200, obteve %d", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("esperava Content-Type application/json, obteve %s", ct)
	}
}

func TestHTTPServer_DeveRetornarSSEParaAcceptEventStream(t *testing.T) {
	processor := &fakeProcessor{response: buildTestResponse(t, 2)}
	s := NewHTTPServer(0, processor, oauth.NewValidator("", ""), "", "", nil)

	srv := httptest.NewServer(s.router)
	defer srv.Close()

	reqBody := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 2, Method: "tools/list"}
	resp := postMCP(t, srv, reqBody, "text/event-stream")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("esperava 200, obteve %d", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "text/event-stream") {
		t.Errorf("esperava Content-Type text/event-stream, obteve %s", ct)
	}
}

func TestHTTPServer_DeveRetornar204ParaNotificacao(t *testing.T) {
	// Processor returns nil for notifications
	processor := &fakeProcessor{response: nil}
	s := NewHTTPServer(0, processor, oauth.NewValidator("", ""), "", "", nil)

	srv := httptest.NewServer(s.router)
	defer srv.Close()

	reqBody := mcp.JSONRPCRequest{JSONRPC: "2.0", Method: "notifications/initialized"}
	resp := postMCP(t, srv, reqBody, "application/json")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("esperava 204 para notificação, obteve %d", resp.StatusCode)
	}
}

func TestHTTPServer_DeveRetornar401SemTokenQuandoOAuthAtivo(t *testing.T) {
	// Use a validator pointing to a non-existent JWKS — any request without a token should fail at middleware
	processor := &fakeProcessor{response: buildTestResponse(t, 3)}
	validator := oauth.NewValidator("http://jwks.test/certs", "https://issuer.test")
	s := NewHTTPServer(0, processor, validator, "", "", nil)

	srv := httptest.NewServer(s.router)
	defer srv.Close()

	reqBody := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 3, Method: "tools/list"}

	bodyJSON, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/mcp", bytes.NewReader(bodyJSON))
	req.Header.Set("Content-Type", "application/json")
	// No Authorization header

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("erro ao enviar request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("esperava 401 sem token, obteve %d", resp.StatusCode)
	}
}

func TestHTTPServer_HealthDeveRetornar200(t *testing.T) {
	s := NewHTTPServer(0, &fakeProcessor{}, oauth.NewValidator("", ""), "", "", nil)
	srv := httptest.NewServer(s.router)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/health")
	if err != nil {
		t.Fatalf("erro ao chamar /health: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("esperava 200 em /health, obteve %d", resp.StatusCode)
	}
}

func TestHTTPServer_ConfiguresReadHeaderTimeout(t *testing.T) {
	s := NewHTTPServer(8080, &fakeProcessor{}, oauth.NewValidator("", ""), "", "", nil)
	srv := s.newHTTPServer()

	if srv.ReadHeaderTimeout <= 0 {
		t.Fatal("expected ReadHeaderTimeout to be configured")
	}
	if srv.ReadHeaderTimeout != httpReadHeaderTimeout {
		t.Fatalf("expected ReadHeaderTimeout %s, got %s", httpReadHeaderTimeout, srv.ReadHeaderTimeout)
	}
}

func TestHTTPServer_DeveRetornarVersaoProtocolo2025(t *testing.T) {
	// Simulate an initialize response and confirm the protocol version in JSON output
	initResult := mcp.InitializeResult{
		ProtocolVersion: mcp.ProtocolVersion,
	}
	resp, _ := mcp.NewJSONRPCResponse(float64(1), initResult)
	processor := &fakeProcessor{response: resp}
	s := NewHTTPServer(0, processor, oauth.NewValidator("", ""), "", "", nil)

	srv := httptest.NewServer(s.router)
	defer srv.Close()

	reqBody := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: 1, Method: "initialize"}
	httpResp := postMCP(t, srv, reqBody, "application/json")
	defer httpResp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(httpResp.Body).Decode(&result)

	resultMap, _ := result["result"].(map[string]interface{})
	if resultMap == nil {
		t.Fatal("campo 'result' ausente na resposta")
	}
	if resultMap["protocolVersion"] != "2025-03-26" {
		t.Errorf("esperava protocolVersion '2025-03-26', obteve %v", resultMap["protocolVersion"])
	}
}
