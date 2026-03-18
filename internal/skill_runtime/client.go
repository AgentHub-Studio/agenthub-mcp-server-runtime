// Package skill_runtime implements the HTTP client for the agenthub-skill-runtime.
// Responsible for invoking AgentHub skills via POST /api/v1/skills/invoke.
package skill_runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/AgentHub-Studio/agenthub-mcp-server-runtime/internal/tenant"
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
	baseURL string
	http    *http.Client
}

// NewSkillRuntimeClient creates a new HTTP client for the agenthub-skill-runtime.
func NewSkillRuntimeClient(baseURL string) *SkillRuntimeClient {
	return &SkillRuntimeClient{
		baseURL: baseURL,
		http: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// InvokeSkill invokes a skill by slug with the provided arguments.
// Calls POST /api/v1/skills/invoke on the skill-runtime.
// Tenant ID and Bearer token are read from the context.
func (c *SkillRuntimeClient) InvokeSkill(ctx context.Context, req InvokeSkillRequest) (*SkillResult, error) {
	url := fmt.Sprintf("%s/api/v1/skills/invoke", c.baseURL)

	// Populate tenant ID from context when not set in the request
	if req.TenantID == "" {
		req.TenantID = tenant.IDFromContext(ctx)
	}

	if req.Timeout == 0 {
		req.Timeout = 30000
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("error serializing request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("error creating HTTP request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	if id := tenant.IDFromContext(ctx); id != "" {
		httpReq.Header.Set("X-Tenant-ID", id)
	}

	if tok := tenant.TokenFromContext(ctx); tok != "" {
		httpReq.Header.Set("Authorization", "Bearer "+tok)
	}

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("error calling skill-runtime: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("skill-runtime returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var result SkillResult
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("error decoding skill result: %w", err)
	}

	return &result, nil
}
