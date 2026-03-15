// Pacote mcp implementa o MCPServidor que gerencia a sessão com um cliente MCP.
// O servidor conecta o protocolo de comunicação aos handlers de ferramentas,
// recursos e prompts do AgentHub.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
)

// GerenciadorFerramentas define a interface para listar e invocar ferramentas MCP.
type GerenciadorFerramentas interface {
	ListarFerramentas(ctx context.Context) ([]Ferramenta, error)
	ChamarFerramenta(ctx context.Context, nome string, argumentos map[string]interface{}) (*ResultadoChamarFerramenta, error)
}

// GerenciadorRecursos define a interface para listar e ler recursos MCP.
type GerenciadorRecursos interface {
	ListarRecursos(ctx context.Context) ([]Recurso, error)
	LerRecurso(ctx context.Context, uri string) (*ConteudoRecurso, error)
}

// GerenciadorPrompts define a interface para listar e obter prompts MCP.
type GerenciadorPrompts interface {
	ListarPrompts(ctx context.Context) ([]Prompt, error)
	ObterPrompt(ctx context.Context, nome string, argumentos map[string]string) (*ResultadoObterPrompt, error)
}

// MCPServidor gerencia a sessão completa com um cliente MCP.
// Recebe requisições via protocolo e despacha para os handlers adequados.
type MCPServidor struct {
	ferramentas  GerenciadorFerramentas
	recursos     GerenciadorRecursos
	prompts      GerenciadorPrompts
	inicializado bool
}

// NovoMCPServidor cria um novo servidor MCP com os handlers fornecidos.
func NovoMCPServidor(
	ferramentas GerenciadorFerramentas,
	recursos GerenciadorRecursos,
	prompts GerenciadorPrompts,
) *MCPServidor {
	return &MCPServidor{
		ferramentas: ferramentas,
		recursos:    recursos,
		prompts:     prompts,
	}
}

// Iniciar inicia o loop de leitura stdio, bloqueando até que o contexto seja cancelado
// ou o cliente feche a conexão.
func (s *MCPServidor) Iniciar(ctx context.Context, leitor io.Reader, escritor io.Writer) error {
	protocolo := NovoProtocolo(leitor, escritor, func(req *RequisicaoJSONRPC) *RespostaJSONRPC {
		return s.processarRequisicao(ctx, req)
	})

	log.Println("MCPServidor: aguardando requisições do cliente...")
	return protocolo.IniciarLoop()
}

// processarRequisicao despacha uma requisição JSON-RPC para o handler correto.
// Retorna nil para notificações (sem resposta esperada).
func (s *MCPServidor) processarRequisicao(ctx context.Context, req *RequisicaoJSONRPC) *RespostaJSONRPC {
	log.Printf("MCPServidor: método=%s id=%v", req.Metodo, req.ID)

	switch req.Metodo {
	case "initialize":
		return s.tratarInicializar(ctx, req)

	case "notifications/initialized":
		// Notificação do cliente confirmando inicialização — sem resposta
		s.inicializado = true
		log.Println("MCPServidor: cliente confirmou inicialização")
		return nil

	case "tools/list":
		return s.tratarListarFerramentas(ctx, req)

	case "tools/call":
		return s.tratarChamarFerramenta(ctx, req)

	case "resources/list":
		return s.tratarListarRecursos(ctx, req)

	case "resources/read":
		return s.tratarLerRecurso(ctx, req)

	case "prompts/list":
		return s.tratarListarPrompts(ctx, req)

	case "prompts/get":
		return s.tratarObterPrompt(ctx, req)

	default:
		log.Printf("MCPServidor: método desconhecido: %s", req.Metodo)
		return NovoErroJSONRPC(req.ID, ErroMetodoNaoEncontrado,
			fmt.Sprintf("método não suportado: %s", req.Metodo))
	}
}

// tratarInicializar responde ao handshake MCP com as capacidades do servidor.
func (s *MCPServidor) tratarInicializar(ctx context.Context, req *RequisicaoJSONRPC) *RespostaJSONRPC {
	resultado := ResultadoInicializacao{
		VersaoProtocolo: "2024-11-05",
		InfoServidor: InformacoesServidor{
			Nome:            "agenthub-mcp-server-runtime",
			Versao:          "1.0.0",
			VersaoProtocolo: "2024-11-05",
		},
		Capacidades: CapacidadesServidor{
			Ferramentas: &CapacidadeFerramentas{},
			Recursos:    &CapacidadeRecursos{},
			Prompts:     &CapacidadePrompts{},
		},
	}

	resp, err := NovaRespostaJSONRPC(req.ID, resultado)
	if err != nil {
		return NovoErroJSONRPC(req.ID, ErroInterno, "erro ao serializar resultado de inicialização")
	}

	log.Println("MCPServidor: handshake de inicialização concluído")
	return resp
}

// tratarListarFerramentas retorna todas as skills ativas do tenant como ferramentas MCP.
func (s *MCPServidor) tratarListarFerramentas(ctx context.Context, req *RequisicaoJSONRPC) *RespostaJSONRPC {
	ferramentas, err := s.ferramentas.ListarFerramentas(ctx)
	if err != nil {
		log.Printf("MCPServidor: erro ao listar ferramentas: %v", err)
		return NovoErroJSONRPC(req.ID, ErroInterno, fmt.Sprintf("erro ao listar ferramentas: %v", err))
	}

	resultado := ResultadoListarFerramentas{Ferramentas: ferramentas}
	resp, err := NovaRespostaJSONRPC(req.ID, resultado)
	if err != nil {
		return NovoErroJSONRPC(req.ID, ErroInterno, "erro ao serializar ferramentas")
	}

	log.Printf("MCPServidor: listadas %d ferramentas", len(ferramentas))
	return resp
}

// tratarChamarFerramenta invoca uma skill do AgentHub e retorna o resultado.
func (s *MCPServidor) tratarChamarFerramenta(ctx context.Context, req *RequisicaoJSONRPC) *RespostaJSONRPC {
	var params ParamsChamarFerramenta
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NovoErroJSONRPC(req.ID, ErroParametrosInvalidos, "parâmetros inválidos para tools/call")
	}

	resultado, err := s.ferramentas.ChamarFerramenta(ctx, params.Nome, params.Argumentos)
	if err != nil {
		log.Printf("MCPServidor: erro ao chamar ferramenta '%s': %v", params.Nome, err)
		// Retornar como resultado de erro MCP (não erro JSON-RPC)
		erroResultado := &ResultadoChamarFerramenta{
			EhErro: true,
			Conteudo: []ItemConteudo{
				{Tipo: "text", Texto: fmt.Sprintf("erro ao executar ferramenta: %v", err)},
			},
		}
		resp, _ := NovaRespostaJSONRPC(req.ID, erroResultado)
		return resp
	}

	resp, err := NovaRespostaJSONRPC(req.ID, resultado)
	if err != nil {
		return NovoErroJSONRPC(req.ID, ErroInterno, "erro ao serializar resultado da ferramenta")
	}

	log.Printf("MCPServidor: ferramenta '%s' executada com sucesso", params.Nome)
	return resp
}

// tratarListarRecursos retorna todas as Knowledge Bases ativas como recursos MCP.
func (s *MCPServidor) tratarListarRecursos(ctx context.Context, req *RequisicaoJSONRPC) *RespostaJSONRPC {
	recursos, err := s.recursos.ListarRecursos(ctx)
	if err != nil {
		log.Printf("MCPServidor: erro ao listar recursos: %v", err)
		return NovoErroJSONRPC(req.ID, ErroInterno, fmt.Sprintf("erro ao listar recursos: %v", err))
	}

	resultado := ResultadoListarRecursos{Recursos: recursos}
	resp, err := NovaRespostaJSONRPC(req.ID, resultado)
	if err != nil {
		return NovoErroJSONRPC(req.ID, ErroInterno, "erro ao serializar recursos")
	}

	log.Printf("MCPServidor: listados %d recursos", len(recursos))
	return resp
}

// tratarLerRecurso retorna o conteúdo de uma Knowledge Base pelo URI.
func (s *MCPServidor) tratarLerRecurso(ctx context.Context, req *RequisicaoJSONRPC) *RespostaJSONRPC {
	var params ParamsLerRecurso
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NovoErroJSONRPC(req.ID, ErroParametrosInvalidos, "parâmetros inválidos para resources/read")
	}

	conteudo, err := s.recursos.LerRecurso(ctx, params.URI)
	if err != nil {
		log.Printf("MCPServidor: erro ao ler recurso '%s': %v", params.URI, err)
		return NovoErroJSONRPC(req.ID, ErroInterno, fmt.Sprintf("erro ao ler recurso: %v", err))
	}

	resultado := ResultadoLerRecurso{Conteudos: []ConteudoRecurso{*conteudo}}
	resp, err := NovaRespostaJSONRPC(req.ID, resultado)
	if err != nil {
		return NovoErroJSONRPC(req.ID, ErroInterno, "erro ao serializar conteúdo do recurso")
	}

	log.Printf("MCPServidor: recurso '%s' lido com sucesso", params.URI)
	return resp
}

// tratarListarPrompts retorna os prompts curados do servidor.
func (s *MCPServidor) tratarListarPrompts(ctx context.Context, req *RequisicaoJSONRPC) *RespostaJSONRPC {
	prompts, err := s.prompts.ListarPrompts(ctx)
	if err != nil {
		log.Printf("MCPServidor: erro ao listar prompts: %v", err)
		return NovoErroJSONRPC(req.ID, ErroInterno, fmt.Sprintf("erro ao listar prompts: %v", err))
	}

	resultado := ResultadoListarPrompts{Prompts: prompts}
	resp, err := NovaRespostaJSONRPC(req.ID, resultado)
	if err != nil {
		return NovoErroJSONRPC(req.ID, ErroInterno, "erro ao serializar prompts")
	}

	log.Printf("MCPServidor: listados %d prompts", len(prompts))
	return resp
}

// ProcessarRequisicaoHTTP é o ponto de entrada para requisições vindas do transporte HTTP.
// Permite que o ServidorHTTP delegue o processamento JSON-RPC ao MCPServidor.
func (s *MCPServidor) ProcessarRequisicaoHTTP(ctx context.Context, req *RequisicaoJSONRPC) *RespostaJSONRPC {
	return s.processarRequisicao(ctx, req)
}

// tratarObterPrompt retorna um prompt renderizado com os argumentos fornecidos.
func (s *MCPServidor) tratarObterPrompt(ctx context.Context, req *RequisicaoJSONRPC) *RespostaJSONRPC {
	var params ParamsObterPrompt
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return NovoErroJSONRPC(req.ID, ErroParametrosInvalidos, "parâmetros inválidos para prompts/get")
	}

	resultado, err := s.prompts.ObterPrompt(ctx, params.Nome, params.Argumentos)
	if err != nil {
		log.Printf("MCPServidor: erro ao obter prompt '%s': %v", params.Nome, err)
		return NovoErroJSONRPC(req.ID, ErroInterno, fmt.Sprintf("erro ao obter prompt: %v", err))
	}

	resp, err := NovaRespostaJSONRPC(req.ID, resultado)
	if err != nil {
		return NovoErroJSONRPC(req.ID, ErroInterno, "erro ao serializar resultado do prompt")
	}

	log.Printf("MCPServidor: prompt '%s' obtido com sucesso", params.Nome)
	return resp
}
