# Build multi-stage para agenthub-mcp-server-runtime
# Stage 1: Builder
FROM golang:1.25.12-alpine AS builder

# Instalar dependências de build
RUN apk add --no-cache git make

# Configurar diretório de trabalho
WORKDIR /build

# Copiar código fonte completo
COPY . .

# Baixar dependências e resolver go.sum
RUN go mod download && go mod tidy

# Compilar binário estático
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags='-w -s -extldflags "-static"' \
    -a -installsuffix cgo \
    -o /build/bin/mcp-server-runtime \
    ./cmd/server

# Stage 2: Runtime mínimo
FROM alpine:latest
ARG OCI_CREATED="unknown"
ARG OCI_REVISION="unknown"
ARG OCI_SOURCE="https://github.com/AgentHub-Studio/agenthub-mcp-server-runtime"
ARG OCI_VERSION="local"
LABEL org.opencontainers.image.title="agenthub-mcp-server-runtime" \
    org.opencontainers.image.description="AgentHub MCP server runtime service" \
    org.opencontainers.image.source="${OCI_SOURCE}" \
    org.opencontainers.image.revision="${OCI_REVISION}" \
    org.opencontainers.image.created="${OCI_CREATED}" \
    org.opencontainers.image.version="${OCI_VERSION}" \
    org.opencontainers.image.vendor="AgentHub Studio" \
    org.opencontainers.image.licenses="Proprietary"

# Instalar certificados CA e dados de timezone
RUN apk --no-cache add ca-certificates tzdata

# Criar usuário não-root para segurança
RUN addgroup -g 1000 mcp && \
    adduser -D -u 1000 -G mcp mcp

# Configurar diretório de trabalho
WORKDIR /app

# Copiar binário do builder
COPY --from=builder /build/bin/mcp-server-runtime /app/mcp-server-runtime

# Dar permissão de execução
RUN chmod +x /app/mcp-server-runtime

# Mudar para usuário não-root
USER mcp

# Expor porta HTTP (stdio não precisa de porta)
EXPOSE 8080

# Configurar variáveis de ambiente padrão
ENV AGENTHUB_BACKEND_URL=http://agenthub-backend:8080 \
    AGENTHUB_SKILL_RUNTIME_URL=http://agenthub-skill-runtime:8082 \
    STDIO_MODE=false \
    HTTP_PORT=8080

# Healthcheck via endpoint HTTP
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Comando para executar o servidor
ENTRYPOINT ["/app/mcp-server-runtime"]
