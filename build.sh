#!/bin/bash
# Script de build Docker para agenthub-mcp-server-runtime.
# Executa comandos Go dentro de um container para garantir builds reproduzíveis.
GO_IMAGE="golang:1.25.12-alpine"

docker run --rm \
  -v "$(pwd)":/app \
  -v "$HOME/go/pkg/mod":/go/pkg/mod \
  -w /app \
  ${GO_IMAGE} \
  go "$@"
