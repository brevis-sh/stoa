# stoa: a data lake that is 100% Go. `make up` brings it up, `make demo` writes and reads.
COMPOSE   := docker compose -f docker-compose.yml
LOCAL_ENV := .env.local
BREVIS    := brevissh/brevis:0.41.0
# The steps run inside the scheduler's container, which is Linux on this
# machine's Docker architecture -- arm64 on an Apple Silicon Mac, amd64 elsewhere.
GOARCH    := $(shell docker info --format '{{.Architecture}}' 2>/dev/null | sed 's/aarch64/arm64/;s/x86_64/amd64/')

.PHONY: help build up down seed run land metrics read api demo logs clean
help: ## list the targets
	@grep -E '^[a-z]+:.*##' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*## "}; {printf "  %-8s %s\n", $$1, $$2}'

build: ## build the steps and the API for the scheduler's Linux, into bin/, and fetch the DuckDB CLI there
	@mkdir -p bin
	CGO_ENABLED=0 GOOS=linux GOARCH=$(GOARCH) go build -o bin/ ./cmd/...
	@scripts/duckdb.sh linux-$(GOARCH)-musl bin >/dev/null

# THE LOGIN IS MADE, NOT ASKED FOR: `brevis hash` reads the password from
# standard input, and the published image is enough. The password lives in this
# file, on this machine, never in the repository, which is public.
# `make up STOA_PASSWORD='...'` picks one instead of drawing it.
$(LOCAL_ENV):
	@pw="$${STOA_PASSWORD:-}"; test -n "$$pw" || pw=$$(openssl rand -base64 18); \
	 hash=$$(printf '%s\n' "$$pw" | docker run --rm -i $(BREVIS) hash) || { echo "could not generate the password hash"; exit 1; }; \
	 { \
	   echo "# Made by make up. Never committed: it holds a password."; \
	   echo "BREVIS_AUTH_USER='stoa'"; \
	   echo "BREVIS_AUTH_PASSWORD_HASH='$$hash'"; \
	   echo "BREVIS_AUTH_SECRET='$$(openssl rand -base64 48 | tr -d '\n')'"; \
	   echo "STOA_PASSWORD='$$pw'"; \
	 } > $(LOCAL_ENV); chmod 600 $(LOCAL_ENV); \
	 echo "made $(LOCAL_ENV): this machine's console login"

up: $(LOCAL_ENV) build ## bring up S3, Postgres, engine, console, gateway, Redis and the API; publish the workflow and the gateway
	@set -a; . ./$(LOCAL_ENV); set +a; $(COMPOSE) up -d --build
	@# No -f: with identities in s3.config the S3 root answers 403, and 403 means up.
	@for i in $$(seq 1 40); do curl -s -o /dev/null http://localhost:8333/ && break; sleep 1; done; \
	  curl -s -o /dev/null http://localhost:8333/ || { echo "SeaweedFS did not answer on 8333"; $(COMPOSE) logs seaweedfs; exit 1; }
	@go run ./cmd/bucket
	@for i in $$(seq 1 60); do curl -fs -o /dev/null http://localhost:8081/health && break; sleep 1; done; \
	  curl -fs -o /dev/null http://localhost:8081/health || { echo "the gateway did not answer on 8081"; $(COMPOSE) logs gateway; exit 1; }
	@for i in $$(seq 1 60); do curl -s -o /dev/null http://localhost:8080/ && break; sleep 1; done; \
	  curl -s -o /dev/null http://localhost:8080/ || { echo "the console did not answer on 8080"; $(COMPOSE) logs brevis; exit 1; }
	@for i in $$(seq 1 60); do curl -fs -o /dev/null http://localhost:8082/health && break; sleep 1; done; \
	  curl -fs -o /dev/null http://localhost:8082/health || { echo "the API did not answer on 8082"; $(COMPOSE) logs api; exit 1; }
	@$(COMPOSE) run --rm publish
	@mkdir -p .manifest && $(COMPOSE) run --rm -T describe > .manifest/manifest.json && $(COMPOSE) run --rm publish-gateway
	@. ./$(LOCAL_ENV); echo "the lake is up. Console: http://localhost:8080  user stoa  password $$STOA_PASSWORD"; echo "gateway: POST http://localhost:8081/v1/events   api: GET http://localhost:8082/v1/metrics/daily_revenue"

down: ## stop the containers; the data stays in the volumes
	$(COMPOSE) down

seed: ## 100,000 events in five POSTs to the gateway, which leaves the landing on S3
	go run ./cmd/seed

run: ## trigger a run of the `lake` workflow through the console and wait for the scheduler
	scripts/run.sh lake metrics

land: ## the land step by hand, outside the engine
	go run ./cmd/land

metrics: ## the metrics step by hand, outside the engine
	STOA_DUCKDB=$$(scripts/duckdb.sh) go run ./cmd/metrics

read: ## DuckDB reads bronze.events and gold.daily_revenue by their metadata.json
	scripts/read.sh bronze events
	scripts/read.sh gold daily_revenue

api: ## two reads of the API: the first computes in DuckDB, the second comes from Redis
	@for i in 1 2; do curl -s -D - -o /tmp/stoa-api.json "http://localhost:8082/v1/metrics/daily_revenue?from=2026-11-01&to=2026-11-07" | grep -i "^x-cache\|^x-lake\|^server-timing"; echo; done
	@python3 -c "import json; d=json.load(open('/tmp/stoa-api.json')); print(len(d['rows']), 'rows, snapshot', d['snapshot']); [print('  ', r) for r in d['rows'][:3]]" || cat /tmp/stoa-api.json

demo: seed run read api ## seed through the gateway, run through the engine, read through DuckDB, api with its cache

logs: ## the scheduler's logs, where the steps run
	$(COMPOSE) logs -f scheduler

clean: ## stop everything and drop the volumes: the lake and the engine go back to zero
	$(COMPOSE) down -v
	rm -rf .bin bin
