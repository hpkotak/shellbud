SHELL := /bin/bash

.PHONY: fmt fmt-check test test-race vet lint coverage validate validate-ci hooks build release-dry-run

fmt:
	go fmt ./...

fmt-check:
	@files="$$(find . -type f -name '*.go' -not -path './vendor/*')"; \
	unformatted="$$(gofmt -l $$files)"; \
	if [ -n "$$unformatted" ]; then \
		echo "Go files need formatting:"; \
		echo "$$unformatted"; \
		exit 1; \
	fi

test:
	go test ./...

test-race:
	go test -race ./...

vet:
	go vet ./...

lint:
	golangci-lint run ./...

coverage:
	./scripts/check_coverage.sh

validate: fmt-check vet test test-race lint coverage

validate-ci: validate

hooks:
	git config core.hooksPath .githooks
	chmod +x .githooks/pre-commit

build:
	go build -o sb .

release-dry-run:
	goreleaser release --snapshot --clean
