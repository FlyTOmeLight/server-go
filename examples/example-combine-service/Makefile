# init project path
HOMEDIR := $(shell pwd)
# build output
OUTDIR  := $(HOMEDIR)/output
# server use server ended name
SVCNAME := example-combine-server
# CLI use short name
CLINAME := example-combine

# init command params
GOROOT    := $(shell go env GOROOT)
GO        := $(GOROOT)/bin/go
GOPATH    := $(shell $(GO) env GOPATH)
GOINSTALL := $(GO) install
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
BUILD_DATE    := $(shell date "+%Y-%m-%d %H:%M:%S")
VERSION_VAR := github.com/FlyTOmeLight/server-go/version
LD_FLAGS    := " \
    -X main.CliName=${CLINAME} \
	-X '$(VERSION_VAR).AppVersion=${APP_VERSION}' \
    -X '$(VERSION_VAR).GitBranch=${GIT_BRANCH}' \
	-X '$(VERSION_VAR).GitCommit=${GIT_COMMIT}' \
	-X '$(VERSION_VAR).BuildDate=${BUILD_DATE}' \
    -X '$(VERSION_VAR).BuildPipeline=${AGILE_PIPELINE_NAME}' \
	-X '$(VERSION_VAR).BuildNumber=${AGILE_PIPELINE_BUILD_NUMBER}' \
    "

# gin controller directories that swag scans for openapi comments
CONTROLLER_DIRS := $(shell ls -d $(HOMEDIR)/internal/app/* | tr '\n' ',' | sed 's/,$$//')

all: clean prepare compile package

prepare: check-var git set-env

check-var:
	$(info GO_1_21_HOME: $(GO_1_21_HOME))
	$(info GOROOT: $(GOROOT))
	$(info GOPATH: $(GOPATH))
	$(info GOBIN: $(GOBIN))
	$(info SWAG: $(SWAG))

compile: build

# disable CGO on CLI build for max compatibility
build: set-env
	$(GOMOD) tidy
	CGO_ENABLED=0 $(GOBUILD) -v -ldflags=$(LD_FLAGS) -o $(HOMEDIR)/${SVCNAME} cmd/server/server.go
	CGO_ENABLED=0 $(GOBUILD) -v -ldflags=$(LD_FLAGS) -o $(HOMEDIR)/${CLINAME} cmd/cli/cli.go

# enable CGO for sqlite integration tests
test:
	CGO_ENABLED=1 $(GOTEST) -v -cover $(GOPKGS)

# package layout like
# 	bin/				 for binary executables
#		example1		 cli program name
#		example1-server  server program name
# 	conf/ 				 server configuration 
#   log/				 server logs
# 	doc/ 				 readme, openapi spec etc.
package: package-bin package-cfg package-log

package-bin:
	mkdir -p $(OUTDIR)/bin
	mv $(HOMEDIR)/${SVCNAME} $(OUTDIR)/bin/${SVCNAME}
	mv $(HOMEDIR)/${CLINAME} $(OUTDIR)/bin/${CLINAME}

package-cfg:
	mkdir -p $(OUTDIR)/conf
	cp -r $(HOMEDIR)/conf/server/* $(OUTDIR)/conf/
	cp -r $(HOMEDIR)/conf/cli/. $(OUTDIR)/bin/

package-log:
	mkdir -p $(OUTDIR)/log

clean:
	$(GO) clean
	rm -rf $(OUTDIR)

# avoid filename conflict and speed up build
.PHONY: all prepare check-var compile test package clean build
