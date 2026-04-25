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

// ToolDTO represents a tool returned by the AgentHub backend.
type ToolDTO struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Slug        string                 `json:"slug"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

// SkillToolDTO represents a skill-tool binding returned by the backend.
type SkillToolDTO struct {
	ID       string  `json:"id"`
	SkillID  string  `json:"skillId"`
	Tool     ToolDTO `json:"tool"`
	Priority int     `json:"priority"`
	IsActive bool    `json:"isActive"`
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
