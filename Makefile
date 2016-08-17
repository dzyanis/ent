COMMIT     := $(shell git rev-parse --short HEAD)
VERSION    := 0.3.3

LDFLAGS    := -ldflags \
              "-X main.Commit=$(COMMIT)\
               -X main.Version=$(VERSION)"

GOOS       := $(shell go env GOOS)
GOARCH     := $(shell go env GOARCH)
GO         := GOOS=$(GOOS) GOARCH=$(GOARCH) go

BIN        := ent
ARCHIVE    := $(BIN)-$(VERSION)-$(GOOS)-$(GOARCH).tar.gz


build: $(BIN)

test:
	$(GO) test ./...

release: REMOTE     ?= $(error "can't release, REMOTE not set")
release: REMOTE_DIR ?= $(error "can't release, REMOTE_DIR not set")
release: $(ARCHIVE)
	scp $< $(REMOTE):$(REMOTE_DIR)/

archive: $(ARCHIVE)

clean:
	rm -rf $(BIN) $(ARCHIVE)


.PHONY: build test release archive clean

$(BIN): $(wildcard *.go) Makefile
	$(GO) build -o $@ $(LDFLAGS)

$(ARCHIVE): $(BIN)
	tar -czvf $@ $<
