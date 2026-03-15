// Pacote backend define os DTOs utilizados para deserializar respostas do agenthub-backend.
package backend

// SkillDTO representa uma skill retornada pelo agenthub-backend.
type SkillDTO struct {
	ID          string                 `json:"id"`
	Nome        string                 `json:"name"`
	Slug        string                 `json:"slug"`
	Descricao   string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
	Status      string                 `json:"status"`
}

// KnowledgeBaseDTO representa uma base de conhecimento retornada pelo agenthub-backend.
type KnowledgeBaseDTO struct {
	ID        string `json:"id"`
	Nome      string `json:"name"`
	Descricao string `json:"description"`
	Status    string `json:"status"`
}
