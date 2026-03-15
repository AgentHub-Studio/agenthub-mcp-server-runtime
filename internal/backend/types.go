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
