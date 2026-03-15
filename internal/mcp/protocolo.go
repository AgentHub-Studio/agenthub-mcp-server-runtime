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

// MessageDispatcher is the function called to process a request and return a response.
// Returns nil for notifications (no response expected).
type MessageDispatcher func(req *JSONRPCRequest) *JSONRPCResponse

// Protocol manages JSON-RPC 2.0 communication in the server role.
// Reads requests from the reader and writes responses to the writer in a thread-safe manner.
type Protocol struct {
	reader     *bufio.Reader
	writer     io.Writer
	writeMu    sync.Mutex
	dispatcher MessageDispatcher
}

// NewProtocol creates a new server protocol handler.
func NewProtocol(reader io.Reader, writer io.Writer, dispatcher MessageDispatcher) *Protocol {
	return &Protocol{
		reader:     bufio.NewReader(reader),
		writer:     writer,
		dispatcher: dispatcher,
	}
}

// StartLoop starts the message reading loop from the client.
// Blocks until the reader is closed or an error occurs.
func (p *Protocol) StartLoop() error {
	for {
		line, err := p.reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("erro ao ler mensagem: %w", err)
		}

		if len(line) == 0 {
			continue
		}

		// Parse as JSON-RPC request
		var req JSONRPCRequest
		if err := json.Unmarshal(line, &req); err != nil {
			// Return parse error
			errResp := NewJSONRPCError(nil, ErrCodeParse, "falha ao parsear JSON")
			if writeErr := p.writeMessage(errResp); writeErr != nil {
				return fmt.Errorf("erro ao escrever resposta de parse: %w", writeErr)
			}
			continue
		}

		// Process request via dispatcher
		response := p.dispatcher(&req)

		// Notifications do not generate responses (null ID)
		if response == nil {
			continue
		}

		if err := p.writeMessage(response); err != nil {
			return fmt.Errorf("erro ao escrever resposta: %w", err)
		}
	}
}

// writeMessage serializes and writes a JSON-RPC message to the writer in a thread-safe manner.
func (p *Protocol) writeMessage(message interface{}) error {
	p.writeMu.Lock()
	defer p.writeMu.Unlock()

	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("erro ao serializar mensagem: %w", err)
	}

	// Each message is followed by a newline
	data = append(data, '\n')

	if _, err := p.writer.Write(data); err != nil {
		return fmt.Errorf("erro ao escrever no stream: %w", err)
	}

	return nil
}
