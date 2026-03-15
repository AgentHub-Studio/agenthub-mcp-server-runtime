// Pacote main é o ponto de entrada do agenthub-mcp-server-runtime.
// Inicia o servidor MCP em modo stdio (padrão) e opcionalmente um servidor HTTP.
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/AgentHub-Studio/agenthub-mcp-server-runtime/internal/api"
	"github.com/AgentHub-Studio/agenthub-mcp-server-runtime/internal/backend"
	"github.com/AgentHub-Studio/agenthub-mcp-server-runtime/internal/handler"
	"github.com/AgentHub-Studio/agenthub-mcp-server-runtime/internal/mcp"
	skillruntime "github.com/AgentHub-Studio/agenthub-mcp-server-runtime/internal/skill_runtime"
)

// Configuracao armazena as configurações carregadas de variáveis de ambiente.
type Configuracao struct {
	// BackendURL é a URL base do agenthub-backend (ex: http://agenthub-backend:8080)
	BackendURL string
	// SkillRuntimeURL é a URL base do agenthub-skill-runtime (ex: http://agenthub-skill-runtime:8082)
	SkillRuntimeURL string
	// TenantID é o UUID do tenant para o qual este servidor está configurado
	TenantID string
	// APIToken é o Bearer token para autenticação no backend
	APIToken string
	// ModoStdio define se o servidor deve iniciar em modo stdio (padrão: true)
	ModoStdio bool
	// PortaHTTP é a porta do servidor HTTP (0 = desabilitado)
	PortaHTTP int
}

// carregarConfiguracao carrega as configurações de variáveis de ambiente.
func carregarConfiguracao() *Configuracao {
	portaHTTP := 0
	if porta := obterEnv("HTTP_PORT", ""); porta != "" {
		if p, err := strconv.Atoi(porta); err == nil {
			portaHTTP = p
		}
	}

	modoStdio := true
	if modo := obterEnv("STDIO_MODE", "true"); modo == "false" {
		modoStdio = false
	}

	return &Configuracao{
		BackendURL:      obterEnv("AGENTHUB_BACKEND_URL", "http://agenthub-backend:8080"),
		SkillRuntimeURL: obterEnv("AGENTHUB_SKILL_RUNTIME_URL", "http://agenthub-skill-runtime:8082"),
		TenantID:        obterEnv("AGENTHUB_TENANT_ID", ""),
		APIToken:        obterEnv("AGENTHUB_API_TOKEN", ""),
		ModoStdio:       modoStdio,
		PortaHTTP:       portaHTTP,
	}
}

// obterEnv retorna o valor de uma variável de ambiente ou o valor padrão.
func obterEnv(chave, valorPadrao string) string {
	if valor := os.Getenv(chave); valor != "" {
		return valor
	}
	return valorPadrao
}

func main() {
	log.Println("Iniciando AgentHub MCP Server Runtime...")

	// Carregar configurações
	config := carregarConfiguracao()

	if config.TenantID == "" {
		log.Fatal("AGENTHUB_TENANT_ID é obrigatório — defina a variável de ambiente antes de iniciar")
	}

	log.Printf("Configuração: backendURL=%s, skillRuntimeURL=%s, tenantID=%s, modoStdio=%v, portaHTTP=%d",
		config.BackendURL, config.SkillRuntimeURL, config.TenantID, config.ModoStdio, config.PortaHTTP)

	// Criar clientes HTTP
	clienteBackend := backend.NovoClienteBackend(config.BackendURL, config.TenantID, config.APIToken)
	clienteSkillRuntime := skillruntime.NovoClienteSkillRuntime(config.SkillRuntimeURL, config.TenantID)

	// Criar handlers MCP
	handlerFerramentas := handler.NovoHandlerFerramentas(clienteBackend, clienteSkillRuntime, config.TenantID)
	handlerRecursos := handler.NovoHandlerRecursos(clienteBackend)
	handlerPrompts := handler.NovoHandlerPrompts()

	// Criar servidor MCP
	servidor := mcp.NovoMCPServidor(handlerFerramentas, handlerRecursos, handlerPrompts)

	// Contexto com cancelamento para shutdown graceful
	ctx, cancelar := context.WithCancel(context.Background())
	defer cancelar()

	// Canal para erros dos servidores
	canalErros := make(chan error, 2)

	// Iniciar servidor HTTP se configurado
	if config.PortaHTTP > 0 {
		servidorHTTP := api.NovoServidorHTTP(config.PortaHTTP, servidor)
		go func() {
			log.Printf("Iniciando servidor HTTP na porta %d...", config.PortaHTTP)
			if err := servidorHTTP.Iniciar(); err != nil {
				canalErros <- err
			}
		}()

		// Goroutine para shutdown graceful do HTTP
		go func() {
			<-ctx.Done()
			log.Println("Parando servidor HTTP...")
			ctxParada, cancelarParada := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancelarParada()
			if err := servidorHTTP.Parar(ctxParada); err != nil {
				log.Printf("Erro ao parar servidor HTTP: %v", err)
			}
		}()
	}

	// Iniciar servidor MCP em modo stdio (padrão)
	if config.ModoStdio {
		go func() {
			log.Println("Iniciando servidor MCP em modo stdio...")
			if err := servidor.Iniciar(ctx, os.Stdin, os.Stdout); err != nil {
				canalErros <- err
			}
		}()
	}

	log.Println("AgentHub MCP Server Runtime iniciado com sucesso")
	if config.ModoStdio {
		log.Println("Modo stdio: aguardando mensagens do cliente MCP via stdin")
	}
	if config.PortaHTTP > 0 {
		log.Printf("Servidor HTTP disponível em http://localhost:%d", config.PortaHTTP)
		log.Printf("Health check: GET http://localhost:%d/health", config.PortaHTTP)
		log.Printf("MCP via HTTP: POST http://localhost:%d/mcp", config.PortaHTTP)
	}

	// Aguardar sinal de interrupção ou erro
	canalSinal := make(chan os.Signal, 1)
	signal.Notify(canalSinal, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	select {
	case sig := <-canalSinal:
		log.Printf("Sinal recebido: %v. Iniciando shutdown graceful...", sig)
	case err := <-canalErros:
		log.Printf("Erro fatal em servidor: %v. Encerrando...", err)
	}

	cancelar()
	log.Println("AgentHub MCP Server Runtime encerrado com sucesso")
}
