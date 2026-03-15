// Pacote skill_runtime implementa o cliente HTTP para o agenthub-skill-runtime.
// Responsável por invocar skills do AgentHub via POST /api/v1/skills/invoke.
package skill_runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// RequisicaoInvocarSkill representa o payload para invocar uma skill.
type RequisicaoInvocarSkill struct {
	TenantID  string                 `json:"tenantId"`
	SkillSlug string                 `json:"skillSlug"`
	Input     map[string]interface{} `json:"input"`
	Timeout   int                    `json:"timeout,omitempty"`
}

// ResultadoSkill representa a resposta da invocação de uma skill.
type ResultadoSkill struct {
	ExecutionID string                 `json:"executionId"`
	SkillSlug   string                 `json:"skillSlug"`
	Sucesso     bool                   `json:"success"`
	Resultado   map[string]interface{} `json:"result"`
	Erro        string                 `json:"error"`
	LatenciaMs  int64                  `json:"latencyMs"`
}

// ClienteSkillRuntime realiza invocações de skills no agenthub-skill-runtime.
type ClienteSkillRuntime struct {
	urlBase  string
	tenantID string
	http     *http.Client
}

// NovoClienteSkillRuntime cria um novo cliente HTTP para o agenthub-skill-runtime.
func NovoClienteSkillRuntime(urlBase, tenantID string) *ClienteSkillRuntime {
	return &ClienteSkillRuntime{
		urlBase:  urlBase,
		tenantID: tenantID,
		http: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// InvocarSkill invoca uma skill pelo slug com os argumentos fornecidos.
// Chama POST /api/v1/skills/invoke no skill-runtime.
func (c *ClienteSkillRuntime) InvocarSkill(ctx context.Context, req RequisicaoInvocarSkill) (*ResultadoSkill, error) {
	url := fmt.Sprintf("%s/api/v1/skills/invoke", c.urlBase)

	// Garantir que o tenantId esteja preenchido
	if req.TenantID == "" {
		req.TenantID = c.tenantID
	}

	// Timeout padrão de 30s se não especificado
	if req.Timeout == 0 {
		req.Timeout = 30000
	}

	corpo, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("erro ao serializar requisição: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(corpo))
	if err != nil {
		return nil, fmt.Errorf("erro ao criar requisição HTTP: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Tenant-ID", c.tenantID)

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("erro ao chamar skill-runtime: %w", err)
	}
	defer resp.Body.Close()

	corpoResp, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("erro ao ler resposta: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("skill-runtime retornou status %d: %s", resp.StatusCode, string(corpoResp))
	}

	var resultado ResultadoSkill
	if err := json.Unmarshal(corpoResp, &resultado); err != nil {
		return nil, fmt.Errorf("erro ao decodificar resultado da skill: %w", err)
	}

	return &resultado, nil
}
