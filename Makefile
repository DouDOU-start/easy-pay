.PHONY: dev run web web-deps build tidy test infra infra-down up down logs

# One-shot dev command: infra (docker) + Go API + React UI, all in this terminal.
# Output is prefixed with [api] / [web] so you can tell them apart.
# Ctrl+C cleanly stops both processes.
dev: infra web-deps
	@echo ""
	@echo "  →  Admin UI   http://localhost:5173"
	@echo "  →  Go API     http://localhost:8080"
	@echo "  →  Adminer    http://localhost:8081"
	@echo ""
	@echo "Press Ctrl+C to stop everything."
	@echo ""
	@trap 'kill 0' INT TERM; \
		( go run ./cmd/api --config configs/config.yaml 2>&1 | sed -u 's/^/[api] /' ) & \
		( cd web/admin && npm run dev 2>&1 | sed -u 's/^/[web] /' ) & \
		wait

# First-run convenience: install npm deps only if node_modules is missing.
web-deps:
	@if [ ! -d web/admin/node_modules ]; then \
		echo "→ installing frontend dependencies (first run only)..."; \
		cd web/admin && npm install; \
	fi

# Start infrastructure only and block until healthy.
infra:
	docker compose up -d --wait postgres redis adminer

infra-down:
	docker compose stop postgres redis adminer

# Run only the Go API (if you want the frontend in its own terminal).
run:
	go run ./cmd/api --config configs/config.yaml

# Run only the React UI.
web:
	cd web/admin && npm run dev

# Full stack including the api container (production-ish local run).
up:
	docker compose --profile full up -d --build

down:
	docker compose --profile full down

logs:
	docker compose logs -f api

build:
	CGO_ENABLED=0 go build -o bin/easypay ./cmd/api

tidy:
	go mod tidy

test:
	go test ./...
