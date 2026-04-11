// Pacote backend define os DTOs utilizados para deserializar respostas do agenthub-backend.
package backend

// SkillDTO represents a skill returned by the agenthub-backend.
type SkillDTO struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Slug        string                 `json:"slug"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
	Status      string                 `json:"status"`
}

// KnowledgeBaseDTO represents a knowledge base returned by the agenthub-backend.
type KnowledgeBaseDTO struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Status      string `json:"status"`
}

// PromptTemplateDTO represents a prompt template returned by the agenthub-backend.
type PromptTemplateDTO struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
	Content     string `json:"content"`
	Category    string `json:"category"`
}

// PackageSearchDTO represents a registry package from the search endpoint.
type PackageSearchDTO struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
	Type        string `json:"type"`
}
