.PHONY: build check docker-up docker-down

build:
	cd frontend && npm ci --no-audit --no-fund && npm run build
	mkdir -p dist data
	cd backend && CGO_ENABLED=1 go build -trimpath -ldflags="-s -w" -o ../dist/cdk-recharge ./cmd/server

check:
	cd frontend && npm test && npm run typecheck:portal && npx vue-tsc --noEmit -p tsconfig.audit.json
	cd backend && go test -race ./... && go vet ./...

docker-up:
	docker compose up -d --build

docker-down:
	docker compose down
