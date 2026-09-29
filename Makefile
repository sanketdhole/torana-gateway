.PHONY: all build test bench lint vet proto clean docker-build mockplatform fuzz chaos bench-load profile controlplane controlplane-ui run-controlplane test-controlplane

VERSION ?= 1.0.0
BIN_DIR = bin
BINARY = $(BIN_DIR)/gateway-data
CP_BINARY = $(BIN_DIR)/torana-controlplane
GO ?= /usr/local/go/bin/go
PROTOC ?= protoc

all: vet test build controlplane

build:
	mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags "-s -w -X main.Version=$(VERSION)" -o $(BINARY) ./cmd/gateway-data

controlplane-ui:
	cd controlplane/ui && npm run build

controlplane: controlplane-ui
	mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags "-s -w -X main.Version=$(VERSION)" -o $(CP_BINARY) ./controlplane/cmd/server

run-controlplane:
	$(GO) run ./controlplane/cmd/server

test-controlplane:
	$(GO) test -v ./controlplane/pkg/...

mockplatform:
	$(GO) run ./test/mockplatform

test:
	$(GO) test -v -race ./...

bench:
	$(GO) test -bench=. -benchmem -count=1 ./...

bench-load:
	$(GO) test -bench=. -benchmem -count=3 -timeout 300s ./test/bench/...

profile:
	$(GO) test -run TestProfile -count=1 -timeout 120s ./test/bench/...

fuzz:
	@echo "Fuzzing router..."
	$(GO) test -fuzz=FuzzRouterMatch -fuzztime=30s ./test/fuzz/
	@echo "Fuzzing CEL compiler..."
	$(GO) test -fuzz=FuzzCELCompile -fuzztime=30s ./test/fuzz/
	@echo "Fuzzing JWT parser..."
	$(GO) test -fuzz=FuzzJWTValidateToken -fuzztime=30s ./test/fuzz/
	@echo "Fuzzing MCP JSON-RPC parser..."
	$(GO) test -fuzz=FuzzMCPParseRequest -fuzztime=30s ./test/fuzz/

chaos:
	$(GO) test -v -race -timeout 120s ./test/chaos/...

vet:
	$(GO) vet ./...

lint:
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed, running go vet..."; \
		$(GO) vet ./...; \
	fi

proto:
	@mkdir -p api/proto/controlplane/v1 api/proto/plugin/v1
	@if command -v $(PROTOC) >/dev/null 2>&1 && command -v protoc-gen-go >/dev/null 2>&1; then \
		$(PROTOC) --go_out=. --go_opt=paths=source_relative \
			--go-grpc_out=. --go-grpc_opt=paths=source_relative \
			api/proto/controlplane/v1/controlplane.proto \
			api/proto/plugin/v1/plugin.proto; \
		echo "Protobuf code generated successfully."; \
	else \
		echo "protoc / protoc-gen-go not in PATH; proto contracts are stored in api/proto/"; \
	fi

clean:
	rm -rf $(BIN_DIR)
	rm -rf test/bench/out

DOCKER_IMAGE ?= sanku/torana-data
GHCR_IMAGE ?= ghcr.io/sanketdhole/torana-data

docker-build:
	docker build --build-arg VERSION=$(VERSION) \
		-t $(DOCKER_IMAGE):$(VERSION) -t $(DOCKER_IMAGE):latest \
		-t $(GHCR_IMAGE):$(VERSION) -t $(GHCR_IMAGE):latest \
		-t torana-data:$(VERSION) -t torana-data:latest .

docker-push:
	docker push $(DOCKER_IMAGE):$(VERSION)
	docker push $(DOCKER_IMAGE):latest
