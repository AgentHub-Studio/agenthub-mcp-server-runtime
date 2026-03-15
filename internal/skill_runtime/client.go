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

// InvokeSkillRequest represents the payload to invoke a skill.
type InvokeSkillRequest struct {
	TenantID  string                 `json:"tenantId"`
	SkillSlug string                 `json:"skillSlug"`
	Input     map[string]interface{} `json:"input"`
	Timeout   int                    `json:"timeout,omitempty"`
}

// SkillResult represents the response from a skill invocation.
type SkillResult struct {
	ExecutionID string                 `json:"executionId"`
	SkillSlug   string                 `json:"skillSlug"`
	Success     bool                   `json:"success"`
	Result      map[string]interface{} `json:"result"`
	Error       string                 `json:"error"`
	LatencyMs   int64                  `json:"latencyMs"`
}

// SkillRuntimeClient performs skill invocations on the agenthub-skill-runtime.
type SkillRuntimeClient struct {
	baseURL  string
	tenantID string
	http     *http.Client
}

// NewSkillRuntimeClient creates a new HTTP client for the agenthub-skill-runtime.
func NewSkillRuntimeClient(baseURL, tenantID string) *SkillRuntimeClient {
	return &SkillRuntimeClient{
		baseURL:  baseURL,
		tenantID: tenantID,
		http: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// InvokeSkill invokes a skill by slug with the provided arguments.
// Calls POST /api/v1/skills/invoke on the skill-runtime.
func (c *SkillRuntimeClient) InvokeSkill(ctx context.Context, req InvokeSkillRequest) (*SkillResult, error) {
	url := fmt.Sprintf("%s/api/v1/skills/invoke", c.baseURL)

	// Ensure tenantId is filled
	if req.TenantID == "" {
		req.TenantID = c.tenantID
	}

	// Default timeout of 30s if not specified
	if req.Timeout == 0 {
		req.Timeout = 30000
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("erro ao serializar requisição: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
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

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("erro ao ler resposta: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("skill-runtime retornou status %d: %s", resp.StatusCode, string(respBody))
	}

	var result SkillResult
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("erro ao decodificar resultado da skill: %w", err)
	}

	return &result, nil
}
