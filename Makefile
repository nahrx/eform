# eForm Backend — Makefile
# Usage: make <target>. Environment comes from the shell or .env (see .env.example).

APP      := eform-backend
BIN      := bin/$(APP)
PKG      := ./...
DB_NAME  ?= eform
PG_USER  ?= postgres

.PHONY: help tidy build run dev vet test clean db-create db-drop dl-deps seed db-backup db-restore

help: ## List the available targets
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | \
	  awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

tidy: ## go mod tidy
	go mod tidy

build: ## Compile to bin/
	@mkdir -p bin
	go build -o $(BIN) .
	@echo "→ $(BIN)"

run: build ## Build, then run
	./$(BIN)

dev: ## Run without producing a build artifact (go run)
	go run .

vet: ## go vet
	go vet $(PKG)

test: ## Run the unit tests
	go test $(PKG)

clean: ## Remove build artifacts
	rm -rf bin

db-create: ## Create the local database ($(DB_NAME))
	createdb -U $(PG_USER) $(DB_NAME) || true

db-drop: ## Drop the local database ($(DB_NAME)) — DESTRUCTIVE
	dropdb -U $(PG_USER) $(DB_NAME) || true

seed: ## Load the region data from data/wilayah_indonesia.csv into the database
	go run ./cmd/seeder -file data/wilayah_indonesia.csv

db-backup: ## Back up the database to backups/eform-YYYYmmdd-HHMMSS.dump (custom format, compressed)
	@mkdir -p backups
	@set -a; [ -f .env ] && . ./.env; set +a; 	 f=backups/eform-$$(date +%Y%m%d-%H%M%S).dump; 	 PGPASSWORD="$$POSTGRES_PASSWORD" pg_dump 	   -h "$${POSTGRES_HOST:-localhost}" -p "$${POSTGRES_PORT:-5432}" 	   -U "$${POSTGRES_USER:-postgres}" -d "$${POSTGRES_DB:-eform}" 	   --format=custom --no-owner --no-privileges -f "$$f"; 	 echo "-> $$f ($$(du -h "$$f" | cut -f1))"

db-restore: ## Restore from a backup: make db-restore FILE=backups/xxx.dump — OVERWRITES existing data
	@[ -n "$(FILE)" ] || { echo "Usage: make db-restore FILE=backups/eform-....dump"; exit 1; }
	@set -a; [ -f .env ] && . ./.env; set +a; 	 PGPASSWORD="$$POSTGRES_PASSWORD" pg_restore 	   -h "$${POSTGRES_HOST:-localhost}" -p "$${POSTGRES_PORT:-5432}" 	   -U "$${POSTGRES_USER:-postgres}" -d "$${POSTGRES_DB:-eform}" 	   --clean --if-exists --no-owner --no-privileges "$(FILE)"
	@echo "restored from $(FILE)"
