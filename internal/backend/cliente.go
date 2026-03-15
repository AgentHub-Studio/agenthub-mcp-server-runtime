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

// ClienteBackend realiza chamadas HTTP ao agenthub-backend para consultar
// skills e knowledge bases do tenant.
type ClienteBackend struct {
	urlBase  string
	tenantID string
	token    string
	http     *http.Client
}

// NovoClienteBackend cria um novo cliente HTTP para o agenthub-backend.
func NovoClienteBackend(urlBase, tenantID, token string) *ClienteBackend {
	return &ClienteBackend{
		urlBase:  urlBase,
		tenantID: tenantID,
		token:    token,
		http: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// ListarSkillsAtivas retorna todas as skills ativas do tenant.
// Chama GET /api/skills?status=ACTIVE no backend.
func (c *ClienteBackend) ListarSkillsAtivas(ctx context.Context) ([]SkillDTO, error) {
	url := fmt.Sprintf("%s/api/skills?status=ACTIVE", c.urlBase)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("erro ao criar requisição: %w", err)
	}

	c.adicionarHeaders(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("erro ao chamar backend: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		corpo, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("backend retornou status %d: %s", resp.StatusCode, string(corpo))
	}

	// O backend retorna um wrapper com lista de conteúdo
	var resultado struct {
		Conteudo []SkillDTO `json:"content"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&resultado); err != nil {
		return nil, fmt.Errorf("erro ao decodificar skills: %w", err)
	}

	return resultado.Conteudo, nil
}

// ListarKnowledgeBases retorna todas as knowledge bases do tenant.
// Chama GET /api/knowledge-bases no backend.
func (c *ClienteBackend) ListarKnowledgeBases(ctx context.Context) ([]KnowledgeBaseDTO, error) {
	url := fmt.Sprintf("%s/api/knowledge-bases", c.urlBase)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("erro ao criar requisição: %w", err)
	}

	c.adicionarHeaders(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("erro ao chamar backend: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		corpo, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("backend retornou status %d: %s", resp.StatusCode, string(corpo))
	}

	var resultado struct {
		Conteudo []KnowledgeBaseDTO `json:"content"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&resultado); err != nil {
		return nil, fmt.Errorf("erro ao decodificar knowledge bases: %w", err)
	}

	return resultado.Conteudo, nil
}

// LerKnowledgeBase retorna os detalhes de uma knowledge base pelo ID.
// Chama GET /api/knowledge-bases/{id} no backend.
func (c *ClienteBackend) LerKnowledgeBase(ctx context.Context, kbID string) (*KnowledgeBaseDTO, error) {
	url := fmt.Sprintf("%s/api/knowledge-bases/%s", c.urlBase, kbID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("erro ao criar requisição: %w", err)
	}

	c.adicionarHeaders(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("erro ao chamar backend: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("knowledge base não encontrada: %s", kbID)
	}

	if resp.StatusCode != http.StatusOK {
		corpo, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("backend retornou status %d: %s", resp.StatusCode, string(corpo))
	}

	var kb KnowledgeBaseDTO
	if err := json.NewDecoder(resp.Body).Decode(&kb); err != nil {
		return nil, fmt.Errorf("erro ao decodificar knowledge base: %w", err)
	}

	return &kb, nil
}

// adicionarHeaders adiciona os headers de autenticação e tenant em um request.
func (c *ClienteBackend) adicionarHeaders(req *http.Request) {
	req.Header.Set("X-Tenant-ID", c.tenantID)
	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
}
