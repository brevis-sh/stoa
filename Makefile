# stoa: um lake 100% em Go. `make up` sobe o lake e o engine, `make demo` escreve e lê.
COMPOSE   := docker compose -f docker-compose.yml
LOCAL_ENV := .env.local
BREVIS    := brevissh/brevis:0.41.0
# Os steps rodam dentro do container do scheduler, que é Linux na arquitetura
# do Docker desta máquina -- arm64 num Mac Apple Silicon, amd64 no resto.
GOARCH    := $(shell docker info --format '{{.Architecture}}' 2>/dev/null | sed 's/aarch64/arm64/;s/x86_64/amd64/')

.PHONY: help build up down seed run land read demo logs clean
help: ## lista os alvos
	@grep -E '^[a-z]+:.*##' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*## "}; {printf "  %-8s %s\n", $$1, $$2}'

build: ## compila os steps para o Linux do scheduler, em bin/
	@mkdir -p bin
	CGO_ENABLED=0 GOOS=linux GOARCH=$(GOARCH) go build -o bin/ ./cmd/...

# O LOGIN É FEITO, NÃO PEDIDO: `brevis hash` lê a senha da entrada padrão, e a
# imagem publicada basta. A senha fica neste arquivo, nesta máquina, nunca no
# repositório, que é público. `make up STOA_PASSWORD='...'` escolhe uma.
$(LOCAL_ENV):
	@pw="$${STOA_PASSWORD:-}"; test -n "$$pw" || pw=$$(openssl rand -base64 18); \
	 hash=$$(printf '%s\n' "$$pw" | docker run --rm -i $(BREVIS) hash) || { echo "não consegui gerar o hash da senha"; exit 1; }; \
	 { \
	   echo "# Feito pelo make up. Nunca comitado: tem uma senha."; \
	   echo "BREVIS_AUTH_USER='stoa'"; \
	   echo "BREVIS_AUTH_PASSWORD_HASH='$$hash'"; \
	   echo "BREVIS_AUTH_SECRET='$$(openssl rand -base64 48 | tr -d '\n')'"; \
	   echo "STOA_PASSWORD='$$pw'"; \
	 } > $(LOCAL_ENV); chmod 600 $(LOCAL_ENV); \
	 echo "feito $(LOCAL_ENV): o login do console nesta máquina"

up: $(LOCAL_ENV) build ## sobe S3, Postgres, engine, console e gateway; publica workflow e gateway no console
	@set -a; . ./$(LOCAL_ENV); set +a; $(COMPOSE) up -d
	@# Sem -f: com identidades no s3.config, a raiz do S3 responde 403, e 403 é "de pé".
	@for i in $$(seq 1 40); do curl -s -o /dev/null http://localhost:8333/ && break; sleep 1; done; \
	  curl -s -o /dev/null http://localhost:8333/ || { echo "o SeaweedFS não respondeu em 8333"; $(COMPOSE) logs seaweedfs; exit 1; }
	@go run ./cmd/bucket
	@for i in $$(seq 1 60); do curl -fs -o /dev/null http://localhost:8081/health && break; sleep 1; done; \
	  curl -fs -o /dev/null http://localhost:8081/health || { echo "o gateway não respondeu em 8081"; $(COMPOSE) logs gateway; exit 1; }
	@for i in $$(seq 1 60); do curl -s -o /dev/null http://localhost:8080/ && break; sleep 1; done; \
	  curl -s -o /dev/null http://localhost:8080/ || { echo "o console não respondeu em 8080"; $(COMPOSE) logs brevis; exit 1; }
	@$(COMPOSE) run --rm publish
	@mkdir -p .manifest && $(COMPOSE) run --rm -T describe > .manifest/manifest.json && $(COMPOSE) run --rm publish-gateway
	@. ./$(LOCAL_ENV); echo "lake de pé. Console: http://localhost:8080  usuário stoa  senha $$STOA_PASSWORD"; echo "gateway: POST http://localhost:8081/v1/events"

down: ## derruba os containers; os dados ficam nos volumes
	$(COMPOSE) down

seed: ## 100.000 eventos em cinco POSTs ao gateway, que deixa a landing no S3
	go run ./cmd/seed

run: ## dispara um run do workflow `lake` pelo console e espera o scheduler executar o `land`
	scripts/run.sh lake

land: build ## o mesmo step, na mão, fora do engine
	go run ./cmd/land

read: ## o DuckDB lê a tabela pelo metadata.json: contagem, soma, snapshots
	scripts/read.sh

demo: seed run read ## seed pelo gateway, run pelo engine, read pelo DuckDB

logs: ## os logs do scheduler, onde os steps rodam
	$(COMPOSE) logs -f scheduler

clean: ## derruba e apaga os volumes: o lake e o engine voltam ao zero
	$(COMPOSE) down -v
	rm -rf .bin bin
