.PHONY: clean check build.local build.linux build.osx

BINARY        ?= kube-nodes-downscaler
IMAGE         ?= x0rg/kube-nodes-downscaler
VERSION       ?= v0.0.4
TAG           ?= $(VERSION)
GITHEAD       = $(shell git rev-parse --short HEAD)
GITURL        = $(shell git config --get remote.origin.url)
GITSTATUS     = $(shell git status --porcelain || echo "no changes")
SOURCES       = $(shell find . -name '*.go')
GOPKGS        = $(shell go list ./... | grep -v /vendor/)
BUILD_FLAGS   ?= -v
LDFLAGS       ?= -w -s
default: build.local

clean:
	rm -rf build

test:
	go test -v -race -cover $(GOPKGS)

fmt:
	go fmt $(GOPKGS)

check:
	golint $(GOPKGS)
	go vet -v $(GOPKGS)

build.local: build/$(BINARY)
build.linux: build/linux/$(BINARY)
build.osx: build/osx/$(BINARY)

build/$(BINARY): $(SOURCES)
	go build -o build/$(BINARY) $(BUILD_FLAGS) -ldflags "$(LDFLAGS)" .

build/linux/$(BINARY): $(SOURCES)
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build $(BUILD_FLAGS) -o build/linux/$(BINARY) -ldflags "$(LDFLAGS)" .

build.docker:
	docker build --rm --tag "$(IMAGE):$(VERSION)" .

build.push: build.docker
	docker push "$(IMAGE):$(VERSION)"