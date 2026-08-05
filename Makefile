# eForm Backend — Makefile
# Pakai: make <target>. Env dibaca dari shell / .env (lihat .env.example).

APP      := eform-backend
BIN      := bin/$(APP)
PKG      := ./...
DB_NAME  ?= eform
PG_USER  ?= postgres

.PHONY: help tidy build run dev vet test clean db-create db-drop dl-deps seed db-backup db-restore

help: ## Tampilkan daftar target
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | \
	  awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

tidy: ## go mod tidy
	go mod tidy

build: ## Compile ke bin/
	@mkdir -p bin
	go build -o $(BIN) .
	@echo "→ $(BIN)"

run: build ## Build lalu jalankan
	./$(BIN)

dev: ## Jalankan tanpa build artifact (go run)
	go run .

vet: ## go vet
	go vet $(PKG)

test: ## Jalankan unit test (jika ada)
	go test $(PKG)

clean: ## Hapus artifact
	rm -rf bin

db-create: ## Buat database lokal ($(DB_NAME))
	createdb -U $(PG_USER) $(DB_NAME) || true

db-drop: ## Hapus database lokal ($(DB_NAME)) — HATI-HATI
	dropdb -U $(PG_USER) $(DB_NAME) || true

seed: ## Seed data wilayah dari data/wilayah_indonesia.csv ke database
	go run ./cmd/seeder -file data/wilayah_indonesia.csv

db-backup: ## Backup database ke backups/eform-YYYYmmdd-HHMMSS.dump (format custom, terkompresi)
	@mkdir -p backups
	@set -a; [ -f .env ] && . ./.env; set +a; 	 f=backups/eform-$$(date +%Y%m%d-%H%M%S).dump; 	 PGPASSWORD="$$POSTGRES_PASSWORD" pg_dump 	   -h "$${POSTGRES_HOST:-localhost}" -p "$${POSTGRES_PORT:-5432}" 	   -U "$${POSTGRES_USER:-postgres}" -d "$${POSTGRES_DB:-eform}" 	   --format=custom --no-owner --no-privileges -f "$$f"; 	 echo "-> $$f ($$(du -h "$$f" | cut -f1))"

db-restore: ## Pulihkan dari backup: make db-restore FILE=backups/xxx.dump — MENIMPA data yang ada
	@[ -n "$(FILE)" ] || { echo "Pakai: make db-restore FILE=backups/eform-....dump"; exit 1; }
	@set -a; [ -f .env ] && . ./.env; set +a; 	 PGPASSWORD="$$POSTGRES_PASSWORD" pg_restore 	   -h "$${POSTGRES_HOST:-localhost}" -p "$${POSTGRES_PORT:-5432}" 	   -U "$${POSTGRES_USER:-postgres}" -d "$${POSTGRES_DB:-eform}" 	   --clean --if-exists --no-owner --no-privileges "$(FILE)"
	@echo "pulih dari $(FILE)"
