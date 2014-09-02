COMMIT     := $(shell git rev-parse --short HEAD)
VERSION    := 0.1.0

LDFLAGS    := -ldflags \
              "-X main.Commit $(COMMIT)\
               -X main.Version $(VERSION)"

GOOS       := $(shell go env GOOS)
GOARCH     := $(shell go env GOARCH)
GOBUILD    := GOOS=$(GOOS) GOARCH=$(GOARCH) go build $(LDFLAGS)

ARCHIVE    := ent-$(VERSION)-$(GOOS)-$(GOARCH).tar.gz
DISTDIR    := dist/$(GOOS)_$(GOARCH)

.PHONY: default release archive clean

default: *.go
	$(GOBUILD)

release: REMOTE     ?= $(error "can't release, REMOTE not set")
release: REMOTE_DIR ?= $(error "can't release, REMOTE_DIR not set")
release: dist/$(ARCHIVE)
	scp $< $(REMOTE):$(REMOTE_DIR)/

archive: dist/$(ARCHIVE)

clean:
	git clean -f -x -d

dist/$(ARCHIVE): $(DISTDIR)/ent
	tar -C $(DISTDIR) -czvf $@ .

$(DISTDIR)/ent: *.go
	$(GOBUILD) -o $@
