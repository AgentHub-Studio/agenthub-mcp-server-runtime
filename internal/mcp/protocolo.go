// Pacote mcp implementa o handler de protocolo JSON-RPC 2.0 no papel de servidor.
// O servidor lê requisições do stdin (ou de um io.Reader) e escreve respostas
// no stdout (ou em um io.Writer), diferente do cliente que envia e aguarda.
package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"sync"
)

// DespachanteMensagem é a função chamada para processar uma requisição e retornar uma resposta.
// Retorna nil para notificações (sem resposta esperada).
type DespachanteMensagem func(req *RequisicaoJSONRPC) *RespostaJSONRPC

// Protocolo gerencia a comunicação JSON-RPC 2.0 no papel de servidor.
// Lê requisições do leitor e escreve respostas no escritor de forma thread-safe.
type Protocolo struct {
	leitor       *bufio.Reader
	escritor     io.Writer
	mutexEscrita sync.Mutex
	despachante  DespachanteMensagem
}

// NovoProtocolo cria um novo handler de protocolo servidor.
func NovoProtocolo(leitor io.Reader, escritor io.Writer, despachante DespachanteMensagem) *Protocolo {
	return &Protocolo{
		leitor:      bufio.NewReader(leitor),
		escritor:    escritor,
		despachante: despachante,
	}
}

// IniciarLoop inicia o loop de leitura de mensagens do cliente.
// Bloqueia até que o leitor seja fechado ou ocorra erro.
func (p *Protocolo) IniciarLoop() error {
	for {
		linha, err := p.leitor.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("erro ao ler mensagem: %w", err)
		}

		if len(linha) == 0 {
			continue
		}

		// Parsear como requisição JSON-RPC
		var req RequisicaoJSONRPC
		if err := json.Unmarshal(linha, &req); err != nil {
			// Retornar erro de parse
			errResp := NovoErroJSONRPC(nil, ErroParseamento, "falha ao parsear JSON")
			if writeErr := p.escreverMensagem(errResp); writeErr != nil {
				return fmt.Errorf("erro ao escrever resposta de parse: %w", writeErr)
			}
			continue
		}

		// Processar a requisição via despachante
		resposta := p.despachante(&req)

		// Notificações não geram resposta (ID nulo)
		if resposta == nil {
			continue
		}

		if err := p.escreverMensagem(resposta); err != nil {
			return fmt.Errorf("erro ao escrever resposta: %w", err)
		}
	}
}

// escreverMensagem serializa e escreve uma mensagem JSON-RPC no escritor de forma thread-safe.
func (p *Protocolo) escreverMensagem(mensagem interface{}) error {
	p.mutexEscrita.Lock()
	defer p.mutexEscrita.Unlock()

	dados, err := json.Marshal(mensagem)
	if err != nil {
		return fmt.Errorf("erro ao serializar mensagem: %w", err)
	}

	// Cada mensagem é seguida de newline
	dados = append(dados, '\n')

	if _, err := p.escritor.Write(dados); err != nil {
		return fmt.Errorf("erro ao escrever no stream: %w", err)
	}

	return nil
}
