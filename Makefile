GO_IMAGE := golang:1.24
DOCKER_RUN := docker run --rm -v $(PWD):/app -v $(HOME)/go/pkg/mod:/go/pkg/mod -w /app $(GO_IMAGE)

.PHONY: help build test clean run docker-build docker-run deps fmt lint

help: ## Mostrar ajuda
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

build: ## Compilar o projeto
	@echo "Compilando..."
	$(DOCKER_RUN) go build -o bin/mcp-server-runtime ./cmd/server

test: ## Executar testes
	@echo "Executando testes..."
	$(DOCKER_RUN) sh -c "go mod download && go mod tidy && go test -v ./..."

clean: ## Limpar arquivos gerados
	@echo "Limpando..."
	rm -rf bin/

run: build ## Executar o servidor (modo HTTP na porta 8080)
	@echo "Executando servidor..."
	STDIO_MODE=false HTTP_PORT=8080 ./bin/mcp-server-runtime

docker-build: ## Build da imagem Docker
	docker build -t agenthub-mcp-server-runtime:latest .

docker-run: docker-build ## Executar container Docker
	docker run --rm \
		-p 8080:8080 \
		-e AGENTHUB_TENANT_ID=$(TENANT_ID) \
		-e AGENTHUB_BACKEND_URL=$(BACKEND_URL) \
		-e AGENTHUB_SKILL_RUNTIME_URL=$(SKILL_RUNTIME_URL) \
		-e STDIO_MODE=false \
		-e HTTP_PORT=8080 \
		agenthub-mcp-server-runtime:latest

deps: ## Baixar e sincronizar dependências
	@echo "Baixando dependências..."
	$(DOCKER_RUN) sh -c "go mod download && go mod tidy"

fmt: ## Formatar código
	@echo "Formatando código..."
	$(DOCKER_RUN) go fmt ./...

lint: ## Executar linter
	@echo "Executando linter..."
	$(DOCKER_RUN) golangci-lint run

.DEFAULT_GOAL := help
