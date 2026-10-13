GO ?= go
OUTDIR := output
SVCNAME := streaming-server
VERSION_VAR := github.com/FlyTOmeLight/server-go/version
LD_FLAGS = -X $(VERSION_VAR).AppVersion=1.0.0

.PHONY: all build compile run test check package clean
all: build package
compile: build
build:
	mkdir -p $(OUTDIR)/bin
	CGO_ENABLED=1 $(GO) build -ldflags="$(LD_FLAGS)" -o $(OUTDIR)/bin/$(SVCNAME) ./cmd/server
run:
	CGO_ENABLED=1 $(GO) run ./cmd/server
test:
	CGO_ENABLED=1 $(GO) test ./...
check:
	@diff=$$(gorgeous -d ./...) || exit $$?; if [ -n "$$diff" ]; then printf '%s\n' "$$diff"; exit 1; fi
	CGO_ENABLED=1 $(GO) test ./...
	$(GO) vet ./...
package: build
	mkdir -p $(OUTDIR)/conf/server
	cp conf/server/config.toml $(OUTDIR)/conf/server/
clean:
	rm -rf $(OUTDIR)

SWAG := $(shell $(GO) env GOPATH)/bin/swag
FRAMEWORK_DIR := $(shell $(GO) list -m -f '{{.Dir}}' github.com/FlyTOmeLight/server-go)

.PHONY: doc install-doc
install-doc:
	$(GO) install github.com/swaggo/swag/cmd/swag@v1.8.4
doc:
	$(SWAG) init -d internal/app/greeting,$(FRAMEWORK_DIR)/errors -g controller.go --instanceName=Streaming --parseInternal --outputTypes json,yaml -o doc/openapi
