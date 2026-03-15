// Pacote backend implementa o cliente HTTP para comunicação com o agenthub-backend.
// Todos os requests incluem o header X-Tenant-ID para isolamento multi-tenant
// e Authorization: Bearer para autenticação.
package backend

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// BackendClient performs HTTP calls to the agenthub-backend to query
// skills and knowledge bases for the tenant.
type BackendClient struct {
	baseURL  string
	tenantID string
	token    string
	http     *http.Client
}

// NewBackendClient creates a new HTTP client for the agenthub-backend.
func NewBackendClient(baseURL, tenantID, token string) *BackendClient {
	return &BackendClient{
		baseURL:  baseURL,
		tenantID: tenantID,
		token:    token,
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
		return nil, fmt.Errorf("erro ao criar requisição: %w", err)
	}

	c.addHeaders(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("erro ao chamar backend: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("backend retornou status %d: %s", resp.StatusCode, string(body))
	}

	// The backend returns a wrapper with a content list
	var result struct {
		Content []SkillDTO `json:"content"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("erro ao decodificar skills: %w", err)
	}

	return result.Content, nil
}

// ListKnowledgeBases returns all knowledge bases for the tenant.
// Calls GET /api/knowledge-bases on the backend.
func (c *BackendClient) ListKnowledgeBases(ctx context.Context) ([]KnowledgeBaseDTO, error) {
	url := fmt.Sprintf("%s/api/knowledge-bases", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("erro ao criar requisição: %w", err)
	}

	c.addHeaders(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("erro ao chamar backend: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("backend retornou status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Content []KnowledgeBaseDTO `json:"content"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("erro ao decodificar knowledge bases: %w", err)
	}

	return result.Content, nil
}

// GetKnowledgeBase returns the details of a knowledge base by ID.
// Calls GET /api/knowledge-bases/{id} on the backend.
func (c *BackendClient) GetKnowledgeBase(ctx context.Context, kbID string) (*KnowledgeBaseDTO, error) {
	url := fmt.Sprintf("%s/api/knowledge-bases/%s", c.baseURL, kbID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("erro ao criar requisição: %w", err)
	}

	c.addHeaders(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("erro ao chamar backend: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("knowledge base não encontrada: %s", kbID)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("backend retornou status %d: %s", resp.StatusCode, string(body))
	}

	var kb KnowledgeBaseDTO
	if err := json.NewDecoder(resp.Body).Decode(&kb); err != nil {
		return nil, fmt.Errorf("erro ao decodificar knowledge base: %w", err)
	}

	return &kb, nil
}

// addHeaders adds authentication and tenant headers to a request.
func (c *BackendClient) addHeaders(req *http.Request) {
	req.Header.Set("X-Tenant-ID", c.tenantID)
	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
}
