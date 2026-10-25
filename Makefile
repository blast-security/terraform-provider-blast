BINARY=terraform-provider-blast
OS_ARCH=$(shell go env GOOS)_$(shell go env GOARCH)
VERSION?=0.1.0

build:
	go build -o bin/$(BINARY) .

test:
	go vet ./... && go test ./...

e2e: ## lifecycle test against the in-memory mock API
	scripts/e2e_mock.sh $(or $(TF),terraform)

mock: ## run the mock API on :8089
	go run ./tools/mockserver -addr 127.0.0.1:8089

install: build ## install for local use without dev_overrides
	mkdir -p ~/.terraform.d/plugins/registry.terraform.io/blast-security/blast/$(VERSION)/$(OS_ARCH)
	cp bin/$(BINARY) ~/.terraform.d/plugins/registry.terraform.io/blast-security/blast/$(VERSION)/$(OS_ARCH)/$(BINARY)_v$(VERSION)

.PHONY: build test e2e mock install
