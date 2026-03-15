// Pacote mcp define os tipos do protocolo MCP (Model Context Protocol)
// e JSON-RPC 2.0 utilizados pelo servidor AgentHub.
package mcp

import "encoding/json"

// ========== JSON-RPC 2.0 ==========

// RequisicaoJSONRPC representa uma requisição JSON-RPC 2.0 recebida pelo servidor.
type RequisicaoJSONRPC struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Metodo  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// RespostaJSONRPC representa uma resposta JSON-RPC 2.0 enviada pelo servidor.
type RespostaJSONRPC struct {
	JSONRPC   string          `json:"jsonrpc"`
	ID        interface{}     `json:"id,omitempty"`
	Resultado json.RawMessage `json:"result,omitempty"`
	Erro      *ErroJSONRPC    `json:"error,omitempty"`
}

// ErroJSONRPC representa um erro JSON-RPC 2.0.
type ErroJSONRPC struct {
	Codigo   int             `json:"code"`
	Mensagem string          `json:"message"`
	Dados    json.RawMessage `json:"data,omitempty"`
}

// ========== Códigos de Erro JSON-RPC ==========

const (
	// ErroParseamento indica falha ao parsear JSON
	ErroParseamento = -32700
	// ErroRequisicaoInvalida indica requisição malformada
	ErroRequisicaoInvalida = -32600
	// ErroMetodoNaoEncontrado indica método desconhecido
	ErroMetodoNaoEncontrado = -32601
	// ErroParametrosInvalidos indica parâmetros inválidos
	ErroParametrosInvalidos = -32602
	// ErroInterno indica erro interno do servidor
	ErroInterno = -32603
	// ErroServidor indica erro específico do servidor MCP
	ErroServidor = -32000
)

// ========== Capacidades do Servidor MCP ==========

// InformacoesServidor descreve o servidor MCP.
type InformacoesServidor struct {
	Nome            string `json:"name"`
	Versao          string `json:"version"`
	VersaoProtocolo string `json:"protocolVersion"`
}

// InformacoesCliente descreve o cliente MCP conectado.
type InformacoesCliente struct {
	Nome   string `json:"name"`
	Versao string `json:"version"`
}

// CapacidadeFerramentas indica suporte a listagem e chamada de tools.
type CapacidadeFerramentas struct{}

// CapacidadeRecursos indica suporte a listagem e leitura de resources.
type CapacidadeRecursos struct{}

// CapacidadePrompts indica suporte a listagem e obtenção de prompts.
type CapacidadePrompts struct{}

// CapacidadesServidor agrupa todas as capacidades suportadas pelo servidor.
type CapacidadesServidor struct {
	Ferramentas *CapacidadeFerramentas `json:"tools,omitempty"`
	Recursos    *CapacidadeRecursos    `json:"resources,omitempty"`
	Prompts     *CapacidadePrompts     `json:"prompts,omitempty"`
}

// ParamsInicializacao contém os parâmetros enviados pelo cliente no handshake.
type ParamsInicializacao struct {
	VersaoProtocolo string             `json:"protocolVersion"`
	Capacidades     interface{}        `json:"capabilities"`
	InfoCliente     InformacoesCliente `json:"clientInfo"`
}

// ResultadoInicializacao é a resposta do servidor ao handshake de inicialização.
type ResultadoInicializacao struct {
	VersaoProtocolo string              `json:"protocolVersion"`
	Capacidades     CapacidadesServidor `json:"capabilities"`
	InfoServidor    InformacoesServidor `json:"serverInfo"`
}

// ========== Ferramentas (Tools) ==========

// Ferramenta representa uma skill do AgentHub exposta como MCP Tool.
type Ferramenta struct {
	Nome        string                 `json:"name"`
	Descricao   string                 `json:"description,omitempty"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

// ResultadoListarFerramentas é o resultado do método tools/list.
type ResultadoListarFerramentas struct {
	Ferramentas []Ferramenta `json:"tools"`
}

// ParamsChamarFerramenta contém os parâmetros do método tools/call.
type ParamsChamarFerramenta struct {
	Nome      string                 `json:"name"`
	Argumentos map[string]interface{} `json:"arguments,omitempty"`
}

// ItemConteudo representa um item de conteúdo retornado por uma ferramenta.
type ItemConteudo struct {
	Tipo  string `json:"type"` // "text", "image", "resource"
	Texto string `json:"text,omitempty"`
	Dados string `json:"data,omitempty"`
	URI   string `json:"uri,omitempty"`
}

// ResultadoChamarFerramenta é o resultado do método tools/call.
type ResultadoChamarFerramenta struct {
	Conteudo []ItemConteudo `json:"content"`
	EhErro   bool           `json:"isError,omitempty"`
}

// ========== Recursos (Resources) ==========

// Recurso representa uma Knowledge Base do AgentHub exposta como MCP Resource.
type Recurso struct {
	URI       string `json:"uri"`
	Nome      string `json:"name"`
	Descricao string `json:"description,omitempty"`
	TipoMIME  string `json:"mimeType,omitempty"`
}

// ResultadoListarRecursos é o resultado do método resources/list.
type ResultadoListarRecursos struct {
	Recursos []Recurso `json:"resources"`
}

// ParamsLerRecurso contém os parâmetros do método resources/read.
type ParamsLerRecurso struct {
	URI string `json:"uri"`
}

// ConteudoRecurso representa o conteúdo de um recurso lido.
type ConteudoRecurso struct {
	URI      string `json:"uri"`
	TipoMIME string `json:"mimeType,omitempty"`
	Texto    string `json:"text,omitempty"`
	Blob     string `json:"blob,omitempty"` // Base64 codificado
}

// ResultadoLerRecurso é o resultado do método resources/read.
type ResultadoLerRecurso struct {
	Conteudos []ConteudoRecurso `json:"contents"`
}

// ========== Prompts ==========

// ArgumentoPrompt descreve um argumento aceito por um prompt.
type ArgumentoPrompt struct {
	Nome      string `json:"name"`
	Descricao string `json:"description,omitempty"`
	Obrigatorio bool  `json:"required,omitempty"`
}

// Prompt representa um prompt curado exposto pelo servidor MCP.
type Prompt struct {
	Nome      string            `json:"name"`
	Descricao string            `json:"description,omitempty"`
	Argumentos []ArgumentoPrompt `json:"arguments,omitempty"`
}

// ResultadoListarPrompts é o resultado do método prompts/list.
type ResultadoListarPrompts struct {
	Prompts []Prompt `json:"prompts"`
}

// ParamsObterPrompt contém os parâmetros do método prompts/get.
type ParamsObterPrompt struct {
	Nome      string            `json:"name"`
	Argumentos map[string]string `json:"arguments,omitempty"`
}

// MensagemPrompt representa uma mensagem no resultado de um prompt.
type MensagemPrompt struct {
	Papel    string       `json:"role"` // "user", "assistant"
	Conteudo ItemConteudo `json:"content"`
}

// ResultadoObterPrompt é o resultado do método prompts/get.
type ResultadoObterPrompt struct {
	Descricao string           `json:"description,omitempty"`
	Mensagens []MensagemPrompt `json:"messages"`
}

// ========== Funções auxiliares ==========

// NovaRespostaJSONRPC cria uma resposta JSON-RPC 2.0 de sucesso.
func NovaRespostaJSONRPC(id interface{}, resultado interface{}) (*RespostaJSONRPC, error) {
	resultadoJSON, err := json.Marshal(resultado)
	if err != nil {
		return nil, err
	}
	return &RespostaJSONRPC{
		JSONRPC:   "2.0",
		ID:        id,
		Resultado: resultadoJSON,
	}, nil
}

// NovoErroJSONRPC cria uma resposta JSON-RPC 2.0 de erro.
func NovoErroJSONRPC(id interface{}, codigo int, mensagem string) *RespostaJSONRPC {
	return &RespostaJSONRPC{
		JSONRPC: "2.0",
		ID:      id,
		Erro: &ErroJSONRPC{
			Codigo:   codigo,
			Mensagem: mensagem,
		},
	}
}
