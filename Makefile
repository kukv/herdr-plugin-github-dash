.PHONY: fmt fmt-check lint test check

# Auto-format the code (gofumpt + goimports via golangci-lint).
fmt:
	golangci-lint fmt ./...

# Fail if the code is not formatted (mirrors the CI format gate).
fmt-check:
	golangci-lint fmt --diff ./...

# Run the linters (mirrors the CI lint step).
lint:
	golangci-lint run ./...

# Run tests with the race detector and coverage (mirrors the CI test step).
test:
	gotestsum --format testdox -- -race -coverprofile=coverage.out -covermode=atomic ./...

# Run everything the CI checks, locally.
check: lint fmt-check test
