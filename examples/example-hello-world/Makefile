# init project path
HOMEDIR := $(shell pwd)
# build output
OUTDIR  := $(HOMEDIR)/output
# server use server ended name
SVCNAME := helloworld-server
# CLI use short name
CLINAME := helloworld
# Swagger name
SWAG_DOC_PREFIX := HelloWorld

# init command params
GOROOT    := $(shell go env GOROOT)
GO        := $(GOROOT)/bin/go
GOPATH    := $(shell $(GO) env GOPATH)
GOINSTALL := $(GO) install
GOGET     := $(GO) get
GOMOD     := $(GO) mod
GOBUILD   := $(GO) build
GOTEST    := $(GO) test -gcflags="-N -l"
GOPKGS    := $$($(GO) list ./...| grep -vE "vendor")
SWAG      := $(GOPATH)/bin/swag

# test cover files
COVPROF := $(HOMEDIR)/covprof.out  # coverage profile
COVFUNC := $(HOMEDIR)/covfunc.txt  # coverage profile information for each function
COVHTML := $(HOMEDIR)/covhtml.html # HTML representation of coverage profile

# flags to inject into binaries
APP_VERSION := 1.0.0
GIT_BRANCH  := $(shell git rev-parse --abbrev-ref HEAD)
GIT_COMMIT  := $(shell git rev-parse HEAD)
BUILD_DATE  := $(shell date "+%Y-%m-%d %H:%M:%S")
VERSION_VAR := github.com/FlyTOmeLight/server-go/version
GOMODULE    := $(shell $(GO) list -m)

# gin controller directories that swag scans for openapi comments
SWAG_SCAN_DIRS := $(shell ls -d $(HOMEDIR)/internal/app/* | tr '\n' ',' | sed 's/,$$//')
SWAGGER_VAR    := github.com/FlyTOmeLight/server-go/swagger

LD_FLAGS := " \
    -X main.CliName=${CLINAME} \
	-X '$(VERSION_VAR).AppVersion=${APP_VERSION}' \
    -X '$(VERSION_VAR).GitBranch=${GIT_BRANCH}' \
	-X '$(VERSION_VAR).GitCommit=${GIT_COMMIT}' \
	-X '$(VERSION_VAR).BuildDate=${BUILD_DATE}' \
    -X '$(VERSION_VAR).BuildPipeline=${AGILE_PIPELINE_NAME}' \
	-X '$(VERSION_VAR).BuildNumber=${AGILE_PIPELINE_BUILD_NUMBER}' \
	-X '$(SWAGGER_VAR).DocPrefix=${SWAG_DOC_PREFIX}' \
    "

all: clean prepare compile doc package

prepare: check-var install-doc

check-var:
	$(info GOROOT: $(GOROOT))
	$(info GOPATH: $(GOPATH))
	$(info GOBIN: $(GOBIN))
	$(info SWAG: $(SWAG))
	$(info GOMODULE: $(GOMODULE))

install-doc:
	$(GOINSTALL) github.com/swaggo/swag/cmd/swag@v1.8.4

compile: build

# disable CGO on CLI build for max compatibility
build:
	$(GOMOD) tidy
	CGO_ENABLED=0 $(GOBUILD) -v -ldflags=$(LD_FLAGS) -o $(HOMEDIR)/${SVCNAME} cmd/server/server.go
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) -v -ldflags=$(LD_FLAGS) -o $(HOMEDIR)/${SVCNAME}-linux-amd64 cmd/server/server.go
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GOBUILD) -v -ldflags=$(LD_FLAGS) -o $(HOMEDIR)/${SVCNAME}-linux-arm64 cmd/server/server.go

	CGO_ENABLED=0 $(GOBUILD) -v -ldflags=$(LD_FLAGS) -o $(HOMEDIR)/${CLINAME} cmd/cli/cli.go
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) -v -ldflags=$(LD_FLAGS) -o $(HOMEDIR)/${CLINAME}-linux-amd64 cmd/cli/cli.go
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GOBUILD) -v -ldflags=$(LD_FLAGS) -o $(HOMEDIR)/${CLINAME}-linux-arm64 cmd/cli/cli.go

doc:
	$(SWAG) init -d $(SWAG_SCAN_DIRS) -g controller.go --instanceName=${SWAG_DOC_PREFIX} --parseDependency --parseInternal -o $(HOMEDIR)/doc/openapi

upgrade:
	$(GOGET) -u $(shell $(GO) list -u -f '{{if (and (not (or .Main .Indirect)) .Update)}}{{.Path}}{{end}}' -m github.com/FlyTOmeLight/...)

# enable CGO for sqlite integration tests
test:
	CGO_ENABLED=1 $(GOTEST) -cover $(GOPKGS) -coverpkg $(GOMODULE)/...

# package layout like
# 	bin/				  for binary executables
#		helloworld		  cli program name
#		helloworld-server server program name
# 	conf/ 				  server configuration
#   deploy/				  server deploy files
#   log/				  server logs
#   doc/				  readme, openapi spec etc
package: package-bin package-cfg package-deploy package-log package-doc

package-bin:
	mkdir -p $(OUTDIR)/bin
	mv $(HOMEDIR)/${SVCNAME}* $(OUTDIR)/bin
	mv $(HOMEDIR)/${CLINAME}* $(OUTDIR)/bin

package-cfg:
	mkdir -p $(OUTDIR)/conf/server
	cp -r $(HOMEDIR)/conf/server/* $(OUTDIR)/conf/server
	cp -r $(HOMEDIR)/conf/cli/. $(OUTDIR)/bin/

package-deploy:
	mkdir -p $(OUTDIR)/deploy
	cp -r $(HOMEDIR)/deploy/* $(OUTDIR)/deploy

package-log:
	mkdir -p $(OUTDIR)/log

package-doc:
	mkdir -p $(OUTDIR)/doc
	cp $(HOMEDIR)/doc/openapi/* $(OUTDIR)/doc
	rm $(OUTDIR)/doc/*.go

clean:
	$(GO) clean
	rm -rf $(OUTDIR)

# avoid filename conflict and speed up build
.PHONY: all prepare check-var git set-env compile build doc upgrade test package clean
