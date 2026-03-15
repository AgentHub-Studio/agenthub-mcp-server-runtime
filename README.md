# AgentHub MCP Server Runtime

**Servidor MCP** que expõe as capacidades do AgentHub (Skills, Knowledge Bases, Prompts) via protocolo [Model Context Protocol](https://spec.modelcontextprotocol.io/), permitindo que sistemas externos como Claude Desktop, IDEs e outros agentes consumam as capacidades da plataforma.

## Visão Geral

```
Claude Desktop / IDE / Agente externo
        │ (stdio ou HTTP)
        ▼
agenthub-mcp-server-runtime
        │
        ├── GET /api/skills?status=ACTIVE ───► agenthub-backend:8080
        ├── GET /api/knowledge-bases ─────────► agenthub-backend:8080
        └── POST /api/v1/skills/invoke ───────► agenthub-skill-runtime:8082
```

## Capacidades Expostas

| Tipo MCP | Origem AgentHub | Descrição |
|----------|----------------|-----------|
| **Tools** | Skills ativas | Cada skill vira uma MCP Tool com seu `inputSchema` |
| **Resources** | Knowledge Bases | URI: `agenthub://kb/{id}` |
| **Prompts** | Curados (hardcoded) | Templates para operações comuns |

### Prompts disponíveis

- `pesquisar-conhecimento` — busca semântica em knowledge bases
- `executar-skill` — executa uma skill com parâmetros JSON

## Configuração

Todas as configurações são via variáveis de ambiente:

| Variável | Descrição | Padrão |
|----------|-----------|--------|
| `AGENTHUB_TENANT_ID` | **Obrigatório.** UUID do tenant | — |
| `AGENTHUB_BACKEND_URL` | URL do agenthub-backend | `http://agenthub-backend:8080` |
| `AGENTHUB_SKILL_RUNTIME_URL` | URL do agenthub-skill-runtime | `http://agenthub-skill-runtime:8082` |
| `AGENTHUB_API_TOKEN` | Bearer token para autenticação | — |
| `STDIO_MODE` | Iniciar em modo stdio | `true` |
| `HTTP_PORT` | Porta do servidor HTTP (0 = desabilitado) | — |

## Modos de Transporte

### Modo stdio (padrão — Claude Desktop)

O servidor lê requisições JSON-RPC do stdin e escreve respostas no stdout.
Ideal para integração com Claude Desktop, que lança o servidor como subprocesso.

```json
// ~/.config/claude/claude_desktop_config.json
{
  "mcpServers": {
    "agenthub": {
      "command": "/caminho/para/mcp-server-runtime",
      "env": {
        "AGENTHUB_TENANT_ID": "seu-tenant-uuid",
        "AGENTHUB_BACKEND_URL": "http://localhost:8080",
        "AGENTHUB_SKILL_RUNTIME_URL": "http://localhost:8082",
        "AGENTHUB_API_TOKEN": "seu-token",
        "STDIO_MODE": "true"
      }
    }
  }
}
```

### Modo HTTP (ambientes containerizados)

Quando `HTTP_PORT` está definido, o servidor também expõe endpoints HTTP:

```bash
# Iniciar em modo HTTP puro
AGENTHUB_TENANT_ID=uuid STDIO_MODE=false HTTP_PORT=8080 ./mcp-server-runtime

# Health check
curl http://localhost:8080/health

# Enviar requisição JSON-RPC
curl -X POST http://localhost:8080/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}'
```

## Exemplos de Uso

### Handshake de inicialização

```json
// Requisição
{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"claude","version":"1.0"}}}

// Resposta
{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2024-11-05","capabilities":{"tools":{},"resources":{},"prompts":{}},"serverInfo":{"name":"agenthub-mcp-server-runtime","version":"1.0.0","protocolVersion":"2024-11-05"}}}
```

### Listar tools (skills)

```json
// Requisição
{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}

// Resposta
{"jsonrpc":"2.0","id":2,"result":{"tools":[{"name":"document-search","description":"Busca semântica em documentos","inputSchema":{"type":"object","properties":{"query":{"type":"string"},"limit":{"type":"integer"}},"required":["query"]}}]}}
```

### Invocar uma tool (skill)

```json
// Requisição
{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"document-search","arguments":{"query":"arquitetura do AgentHub","limit":5}}}

// Resposta
{"jsonrpc":"2.0","id":3,"result":{"content":[{"type":"text","text":"{\"documents\":[...]}"}],"isError":false}}
```

### Listar resources (knowledge bases)

```json
// Requisição
{"jsonrpc":"2.0","id":4,"method":"resources/list","params":{}}

// Resposta
{"jsonrpc":"2.0","id":4,"result":{"resources":[{"uri":"agenthub://kb/123e4567-e89b-12d3-a456-426614174000","name":"Base de Documentação","description":"Documentação técnica do AgentHub","mimeType":"application/json"}]}}
```

## Build

```bash
# Baixar dependências
make deps

# Compilar
make build

# Executar testes
make test

# Build Docker
make docker-build

# Executar Docker (definir variáveis primeiro)
TENANT_ID=uuid BACKEND_URL=http://localhost:8080 SKILL_RUNTIME_URL=http://localhost:8082 make docker-run
```

## Arquitetura Interna

```
cmd/server/main.go              # Entry point, config, shutdown graceful
internal/
├── mcp/
│   ├── tipos.go                # Tipos JSON-RPC 2.0 e MCP (em pt-BR)
│   ├── protocolo.go            # Loop de leitura/escrita stdio
│   └── servidor.go             # Dispatcher central de métodos MCP
├── backend/
│   ├── tipos.go                # DTOs do agenthub-backend
│   └── cliente.go              # HTTP client para o backend
├── skill_runtime/
│   └── cliente.go              # HTTP client para o skill-runtime
├── handler/
│   ├── ferramentas.go          # tools/list + tools/call
│   ├── recursos.go             # resources/list + resources/read
│   └── prompts.go              # prompts/list + prompts/get
└── api/
    └── http.go                 # Servidor HTTP Gin (transporte alternativo)
```

## Métodos MCP Suportados

| Método | Descrição |
|--------|-----------|
| `initialize` | Handshake inicial com negociação de capacidades |
| `notifications/initialized` | Confirmação do cliente (sem resposta) |
| `tools/list` | Lista skills ativas como MCP Tools |
| `tools/call` | Invoca skill via skill-runtime |
| `resources/list` | Lista Knowledge Bases como MCP Resources |
| `resources/read` | Lê metadados de uma Knowledge Base |
| `prompts/list` | Lista prompts curados |
| `prompts/get` | Obtém prompt renderizado com argumentos |

## Tech Stack

- **Go 1.24**
- **Gin v1.9.1** — servidor HTTP
- **net/http** — clientes HTTP para backend e skill-runtime
- Protocolo MCP via JSON-RPC 2.0 sobre stdio ou HTTP

MIT License
