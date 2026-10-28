# =============================================================================
# Sportss Colombia - tareas de desarrollo
# =============================================================================
SHELL := /bin/sh
APP := sportss
BIN_DIR := bin
BINARY := $(BIN_DIR)/$(APP)

.PHONY: help run dev build tidy fmt vet test redis-up redis-down redis-logs clean

help: ## Muestra esta ayuda
	@echo "Sportss Colombia - comandos disponibles:"
	@grep -E '^[a-zA-Z_-]+:.*## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*## "}; {printf "  %-14s %s\n", $$1, $$2}'

run: ## Ejecuta el servidor con la configuración de .env
	go run ./cmd/server

dev: ## Ejecuta en modo desarrollo (reinicio manual, GIN en modo debug)
	APP_ENV=development go run ./cmd/server

build: ## Compila el binario en bin/sportss
	mkdir -p $(BIN_DIR)
	go build -trimpath -ldflags="-s -w" -o $(BINARY) ./cmd/server

tidy: ## Ordena y descarga dependencias
	go mod tidy

fmt: ## Formatea el código Go
	gofmt -w $$(find . -name '*.go' -not -path './vendor/*')

vet: ## Ejecuta go vet
	go vet ./...

test: ## Ejecuta las pruebas (el proyecto no incluye pruebas automatizadas)
	go test ./...

redis-up: ## Levanta Redis con Docker Compose
	docker compose up -d redis

redis-down: ## Detiene Redis
	docker compose down

redis-logs: ## Muestra logs de Redis
	docker compose logs -f redis

clean: ## Elimina el binario compilado
	rm -rf $(BIN_DIR)
