.PHONY: help
help:
	@echo 'Targets:'
	@echo '  build         Build the archive binary'
	@echo '  run           Run the archive binary locally'
	@echo '  test          Run Go tests'
	@echo '  fmt           Format Go code'
	@echo '  sqlc/generate Generate sqlc code'
	@echo '  migrate/up    Run goose migrations up'
	@echo '  migrate/down  Run goose migrations down'

.PHONY: build
build:
	go build -o ./bin/sleeper-archive ./cmd/sleeper-archive

.PHONY: run
run:
	go run ./cmd/sleeper-archive

.PHONY: test
test:
	go test ./cmd/... ./internal/...

.PHONY: fmt
fmt:
	go fmt ./cmd/... ./internal/...

.PHONY: sqlc/generate
sqlc/generate:
	sqlc generate

.PHONY: migrate/up
migrate/up:
	goose -dir ./migrations postgres "$$DATABASE_URL" up

.PHONY: migrate/down
migrate/down:
	goose -dir ./migrations postgres "$$DATABASE_URL" down
