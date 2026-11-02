# stoa: um lake 100% em Go. `make up` sobe o lake, `make demo` escreve e lê.
COMPOSE := docker compose -f docker-compose.yml

.PHONY: help up down seed land read demo clean
help: ## lista os alvos
	@grep -E '^[a-z]+:.*##' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*## "}; {printf "  %-8s %s\n", $$1, $$2}'

up: ## sobe SeaweedFS (S3) e Postgres (catálogo) e espera os dois responderem
	$(COMPOSE) up -d
	@# Sem -f: com identidades no s3.config, a raiz responde 403, e 403 é "de pé".
	@for i in $$(seq 1 40); do curl -s -o /dev/null http://localhost:8333/ && break; sleep 1; done; \
	  curl -s -o /dev/null http://localhost:8333/ || { echo "o SeaweedFS não respondeu em 8333"; $(COMPOSE) logs seaweedfs; exit 1; }
	@for i in $$(seq 1 40); do [ "$$($(COMPOSE) ps postgres --format '{{.Health}}')" = healthy ] && break; sleep 1; done; \
	  [ "$$($(COMPOSE) ps postgres --format '{{.Health}}')" = healthy ] || { echo "o Postgres não ficou saudável"; $(COMPOSE) logs postgres; exit 1; }
	@echo "lake de pé: S3 em http://localhost:8333, catálogo em localhost:55439"

down: ## derruba os containers; os dados ficam no volume
	$(COMPOSE) down

seed: ## 100.000 eventos NDJSON na landing do S3, em cinco objetos
	go run ./cmd/seed

land: ## a landing vira a tabela Iceberg bronze.events: cinco escritas, cinco snapshots
	go run ./cmd/land

read: ## o DuckDB lê a tabela pelo metadata.json: contagem, soma, snapshots
	scripts/read.sh

demo: seed land read ## seed, land, read

clean: down ## derruba e apaga o volume: o lake volta ao zero
	$(COMPOSE) down -v
	rm -rf .bin
