SHELL := /bin/bash
GOIMPORTS ?= goimports

.PHONY: fmt fmt-check ensure-goimports test test-race vet lint coverage validate validate-ci hooks build release-dry-run

fmt:
	@$(MAKE) ensure-goimports
	@files="$$(find . -type f -name '*.go' -not -path './vendor/*')"; \
	goimports_bin="$$(command -v $(GOIMPORTS) || echo "$$(go env GOPATH)/bin/$(GOIMPORTS)")"; \
	"$$goimports_bin" -w $$files

fmt-check:
	@$(MAKE) ensure-goimports
	@files="$$(find . -type f -name '*.go' -not -path './vendor/*')"; \
	goimports_bin="$$(command -v $(GOIMPORTS) || echo "$$(go env GOPATH)/bin/$(GOIMPORTS)")"; \
	unformatted="$$("$$goimports_bin" -l $$files)"; \
	if [ -n "$$unformatted" ]; then \
		echo "Go files need formatting/import cleanup (goimports):"; \
		echo "$$unformatted"; \
		exit 1; \
	fi

ensure-goimports:
	@command -v $(GOIMPORTS) >/dev/null 2>&1 || [ -x "$$(go env GOPATH)/bin/$(GOIMPORTS)" ] || { \
		echo "goimports is required. Install with:"; \
		echo "  go install golang.org/x/tools/cmd/goimports@latest"; \
		exit 1; \
	}

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
