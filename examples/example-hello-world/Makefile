GO ?= go
OUTDIR := output
SVCNAME := helloworld-server
CLINAME := helloworld
SWAG_DOC_PREFIX := HelloWorld
SWAG := $(shell $(GO) env GOPATH)/bin/swag
FRAMEWORK_DIR := $(shell $(GO) list -m -f '{{.Dir}}' github.com/FlyTOmeLight/server-go)
SWAG_SCAN_DIRS := $(shell ls -d internal/app/* | paste -sd ',' -)
SWAG_SDK_DIRS := $(shell ls -d sdk/go/* | paste -sd ',' -)
VERSION_VAR := github.com/FlyTOmeLight/server-go/version
LD_FLAGS = -X $(VERSION_VAR).AppVersion=1.0.0

.PHONY: all build compile run test check package clean
all: build package
compile: build
build:
	mkdir -p $(OUTDIR)/bin
	CGO_ENABLED=1 $(GO) build -ldflags="$(LD_FLAGS)" -o $(OUTDIR)/bin/$(SVCNAME) ./cmd/server
	CGO_ENABLED=0 $(GO) build -ldflags="$(LD_FLAGS)" -o $(OUTDIR)/bin/$(CLINAME) ./cmd/cli
run:
	CGO_ENABLED=1 $(GO) run ./cmd/server
test:
	CGO_ENABLED=1 $(GO) test ./...
check:
	@diff=$$(gorgeous -d ./...) || exit $$?; if [ -n "$$diff" ]; then printf '%s\n' "$$diff"; exit 1; fi
	CGO_ENABLED=1 $(GO) test ./...
	$(GO) vet ./...
	CGO_ENABLED=1 $(GO) test -race ./service ./sdk/go/...
package: build
	mkdir -p $(OUTDIR)/conf/server
	cp conf/server/config.toml $(OUTDIR)/conf/server/
	cp -R deploy $(OUTDIR)/
	mkdir -p $(OUTDIR)/doc/openapi
	cp doc/openapi/*.json doc/openapi/*.yaml $(OUTDIR)/doc/openapi/
clean:
	rm -rf $(OUTDIR)

.PHONY: doc install-doc
install-doc:
	$(GO) install github.com/swaggo/swag/cmd/swag@v1.8.4
doc:
	$(SWAG) init -d $(SWAG_SCAN_DIRS),$(SWAG_SDK_DIRS),$(FRAMEWORK_DIR)/sdk,$(FRAMEWORK_DIR)/errors -g controller.go --instanceName=$(SWAG_DOC_PREFIX) --parseInternal -o doc/openapi
