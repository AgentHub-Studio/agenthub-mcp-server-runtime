// Package backend implements the HTTP client for communication with agenthub-backend.
// Tenant identity (X-Tenant-ID header and Bearer token) is read from the
// request context on every call, so a single client instance serves all tenants.
package backend

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/AgentHub-Studio/agenthub-mcp-server-runtime/internal/tenant"
)

// BackendClient performs HTTP calls to the agenthub-backend to query
// skills and knowledge bases for the tenant.
type BackendClient struct {
	baseURL       string
	tokenProvider TokenProvider // fallback token provider when context carries no Bearer token
	http          *http.Client
}

// NewBackendClient creates a new HTTP client for the agenthub-backend.
// tokenProvider is optional; used only when the request context has no Bearer token.
func NewBackendClient(baseURL string, tokenProvider TokenProvider) *BackendClient {
	return &BackendClient{
		baseURL:       baseURL,
		tokenProvider: tokenProvider,
		http: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// ListActiveSkills returns all active skills for the tenant.
// Calls GET /api/skills?status=ACTIVE on the backend.
func (c *BackendClient) ListActiveSkills(ctx context.Context) ([]SkillDTO, error) {
	url := fmt.Sprintf("%s/api/skills?status=ACTIVE", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	c.addHeaders(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error calling backend: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("backend returned status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Content []SkillDTO `json:"content"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("error decoding skills: %w", err)
	}

	return result.Content, nil
}

// ListSkillTools returns the tools bound to a skill.
// Calls GET /api/skills/{id}/tools on the backend.
func (c *BackendClient) ListSkillTools(ctx context.Context, skillID string) ([]SkillToolDTO, error) {
	url := fmt.Sprintf("%s/api/skills/%s/tools", c.baseURL, skillID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	c.addHeaders(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error calling backend: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("backend returned status %d: %s", resp.StatusCode, string(body))
	}

	var result []SkillToolDTO
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("error decoding skill tools: %w", err)
	}

	return result, nil
}

// ListKnowledgeBases returns all knowledge bases for the tenant.
// Calls GET /api/knowledge-bases on the backend.
func (c *BackendClient) ListKnowledgeBases(ctx context.Context) ([]KnowledgeBaseDTO, error) {
	url := fmt.Sprintf("%s/api/knowledge-bases", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	c.addHeaders(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error calling backend: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("backend returned status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Content []KnowledgeBaseDTO `json:"content"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("error decoding knowledge bases: %w", err)
	}

	return result.Content, nil
}

// GetKnowledgeBase returns the details of a knowledge base by ID.
// Calls GET /api/knowledge-bases/{id} on the backend.
func (c *BackendClient) GetKnowledgeBase(ctx context.Context, kbID string) (*KnowledgeBaseDTO, error) {
	url := fmt.Sprintf("%s/api/knowledge-bases/%s", c.baseURL, kbID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	c.addHeaders(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error calling backend: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("knowledge base not found: %s", kbID)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("backend returned status %d: %s", resp.StatusCode, string(body))
	}

	var kb KnowledgeBaseDTO
	if err := json.NewDecoder(resp.Body).Decode(&kb); err != nil {
		return nil, fmt.Errorf("error decoding knowledge base: %w", err)
	}

	return &kb, nil
}

// ListPromptTemplates returns all global prompt templates from the backend.
// Calls GET /api/prompt-templates on the backend.
func (c *BackendClient) ListPromptTemplates(ctx context.Context) ([]PromptTemplateDTO, error) {
	url := fmt.Sprintf("%s/api/prompt-templates?size=100", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}
	c.addHeaders(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error calling backend: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("backend returned status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Content []PromptTemplateDTO `json:"content"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("error decoding prompt templates: %w", err)
	}
	return result.Content, nil
}

// SearchPackages performs a registry search across PUBLIC packages.
// Calls GET /api/registry/search?q=...&type=... on the backend.
func (c *BackendClient) SearchPackages(ctx context.Context, query string, pkgType string) ([]PackageSearchDTO, error) {
	params := url.Values{}
	params.Set("q", query)
	params.Set("size", "20")
	if pkgType != "" {
		params.Set("type", pkgType)
	}
	u := fmt.Sprintf("%s/api/registry/search?%s", c.baseURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}
	c.addHeaders(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error calling backend: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("backend returned status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Content []PackageSearchDTO `json:"content"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("error decoding search results: %w", err)
	}
	return result.Content, nil
}

// addHeaders sets authentication and tenant headers on the outgoing request.
// Tenant ID and Bearer token are read from the request context; the token
// provider (static or Keycloak client credentials) is used as a fallback.
func (c *BackendClient) addHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")

	if id := tenant.IDFromContext(req.Context()); id != "" {
		req.Header.Set("X-Tenant-ID", id)
	}

	if tok := tenant.TokenFromContext(req.Context()); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	} else if c.tokenProvider != nil {
		if tok, err := c.tokenProvider.Token(); err == nil && tok != "" {
			req.Header.Set("Authorization", "Bearer "+tok)
		}
	}
}
