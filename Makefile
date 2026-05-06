.PHONY: dev run web web-deps web-ensure web-build build tidy test infra infra-down up down logs

# One-shot dev command: infra (docker) + Go API + React UI, all in this terminal.
# Output is prefixed with [api] / [web] so you can tell them apart.
# Ctrl+C cleanly stops both processes.
dev: web-ensure
	@echo ""
	@echo "  →  Admin UI   http://localhost:5173"
	@echo "  →  Go API     http://localhost:8080"
	@echo ""
	@echo "Press Ctrl+C to stop everything."
	@echo ""
	@trap 'kill 0' INT TERM; \
		( while true; do \
			go run ./backend/cmd/api --config backend/configs/config.yaml 2>&1 | sed -u 's/^/[api] /'; \
			echo "[api] exited, restarting in 1s..."; \
			sleep 1; \
		done ) & \
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
	docker compose -f deploy/docker-compose.yml up -d --wait postgres redis adminer

infra-down:
	docker compose -f deploy/docker-compose.yml stop postgres redis adminer

# Run only the Go API (if you want the frontend in its own terminal).
run:
	go run ./backend/cmd/api --config backend/configs/config.yaml

# Run only the React UI.
web:
	cd web/admin && npm run dev

# Full stack including the api container (production-ish local run).
up:
	docker compose -f deploy/docker-compose.yml --profile full up -d --build

down:
	docker compose -f deploy/docker-compose.yml --profile full down

logs:
	docker compose -f deploy/docker-compose.yml logs -f api

# Build dist only if it doesn't exist yet (first run). Dev uses Vite HMR at :5173;
# dist is only needed so `go run` can embed a fallback SPA for :8080.
web-ensure: web-deps
	@if [ ! -f web/admin/dist/index.html ]; then \
		echo "→ building frontend dist (first run only)..."; \
		cd web/admin && npm run build; \
	fi

# Build the frontend bundle into web/admin/dist for go:embed to pick up.
web-build: web-deps
	cd web/admin && npm run build

# Single-binary production build: bundles the admin SPA into the Go binary
# via go:embed. After `make build`, running `./bin/easypay` serves both the
# API and the admin UI on one port — no separate Vite process needed.
build: web-build
	CGO_ENABLED=0 go build -o bin/easypay ./backend/cmd/api

tidy:
	go mod tidy

test:
	go test ./...
