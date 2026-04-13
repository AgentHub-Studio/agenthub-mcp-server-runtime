// Package skill_runtime implements the HTTP client for the agenthub-skill-runtime.
// Invokes AgentHub skills via POST /api/skills/{slug}/execute.
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

// skillRuntimeBody mirrors the executeBody accepted by the agenthub-skill-runtime:
// POST /api/skills/{slug}/execute expects {"input": {...}, "context": {...}}.
type skillRuntimeBody struct {
	Input   map[string]interface{} `json:"input"`
	Context struct {
		TenantID string `json:"tenantId"`
	} `json:"context"`
}

// TokenFunc is a function that returns a valid Bearer token.
// Used as a fallback when the request context carries no token.
type TokenFunc func() (string, error)

// SkillRuntimeClient performs skill invocations on the agenthub-skill-runtime.
type SkillRuntimeClient struct {
	baseURL         string
	fallbackToken   TokenFunc // optional; used when context has no token
	fallbackTenant  string    // optional default tenant ID when context has none
	http            *http.Client
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

// WithFallbackToken sets a token provider function used when the request context
// carries no Bearer token (e.g. when OAuth is disabled on the MCP server).
func (c *SkillRuntimeClient) WithFallbackToken(fn TokenFunc) *SkillRuntimeClient {
	c.fallbackToken = fn
	return c
}

// WithFallbackTenant sets a default tenant ID used when the request context
// carries none (e.g. when OAuth is disabled on the MCP server).
func (c *SkillRuntimeClient) WithFallbackTenant(tenantID string) *SkillRuntimeClient {
	c.fallbackTenant = tenantID
	return c
}

// InvokeSkill invokes a skill by slug with the provided arguments.
// Calls POST /api/skills/{skillSlug}/execute on the agenthub-skill-runtime.
// The tenant is identified from the Bearer JWT in the request context.
func (c *SkillRuntimeClient) InvokeSkill(ctx context.Context, req InvokeSkillRequest) (*SkillResult, error) {
	// Resolve tenant: explicit > context > configured fallback.
	tenantID := req.TenantID
	if tenantID == "" {
		tenantID = tenant.IDFromContext(ctx)
	}
	if tenantID == "" {
		tenantID = c.fallbackTenant
	}

	url := fmt.Sprintf("%s/api/skills/%s/execute", c.baseURL, req.SkillSlug)

	input := req.Input
	if input == nil {
		input = map[string]interface{}{}
	}

	payload := skillRuntimeBody{Input: input}
	payload.Context.TenantID = tenantID

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("error serializing request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("error creating HTTP request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	// Forward tenant and auth headers so the skill-runtime can resolve the schema.
	if tenantID != "" {
		httpReq.Header.Set("X-Tenant-ID", tenantID)
	}
	if tok := tenant.TokenFromContext(ctx); tok != "" {
		httpReq.Header.Set("Authorization", "Bearer "+tok)
	} else if c.fallbackToken != nil {
		if tok, err := c.fallbackToken(); err == nil && tok != "" {
			httpReq.Header.Set("Authorization", "Bearer "+tok)
		}
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

	// The skill-runtime may return a raw JSON object. Wrap it in a SkillResult
	// so callers always get a consistent struct regardless of what the skill returns.
	var raw interface{}
	if err := json.Unmarshal(respBody, &raw); err != nil {
		return &SkillResult{
			SkillSlug: req.SkillSlug,
			Success:   true,
			Result:    map[string]interface{}{"raw": string(respBody)},
		}, nil
	}

	// If the response looks like our SkillResult shape, unmarshal directly.
	var result SkillResult
	if err := json.Unmarshal(respBody, &result); err == nil && (result.SkillSlug != "" || result.ExecutionID != "") {
		return &result, nil
	}

	// Otherwise, wrap the raw response.
	rawMap, _ := raw.(map[string]interface{})
	if rawMap == nil {
		rawMap = map[string]interface{}{"output": raw}
	}
	return &SkillResult{
		SkillSlug: req.SkillSlug,
		Success:   true,
		Result:    rawMap,
	}, nil
}
