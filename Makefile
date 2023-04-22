VERSION := $(shell git describe --tags)
GIT_HASH := $(shell git rev-parse --short HEAD )

GO_VERSION        ?= $(shell go version)
GO_VERSION_NUMBER ?= $(word 3, $(GO_VERSION))
# TODO: This can be replaced with https://github.com/golang/go/issues/37475
# when Go 1.18 is released
LDFLAGS = -ldflags "-X main.Version=${VERSION} -X main.GitHash=${GIT_HASH} -X main.GoVersion=${GO_VERSION_NUMBER}"

.PHONY: build
build:
	CGO_ENABLED=0 go build ${LDFLAGS} -v -o target/sonic_exporter .

.PHONY: build-container-tarball
build-container-tarball:
	mkdir -p target/
	rm -f target/sonic_exporter.tar* || true
	docker build \
		-t sonic_exporter:$(VERSION) \
		--label='com.azure.sonic.manifest=$(shell cat manifest.json | sed "s/__VERSION__/$(subst v,,$(VERSION))/" | jq -c .)' \
		.
	docker image save sonic_exporter:$(VERSION) -o target/sonic_exporter.tar
	gzip target/sonic_exporter.tar
