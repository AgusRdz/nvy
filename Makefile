.PHONY: build test clean cross dist install tidy release-patch release-minor release-major

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

build:
	docker compose run --rm dev go build -ldflags="$(LDFLAGS)" -o bin/nvy .

tidy:
	docker compose run --rm dev go mod tidy

test:
	docker compose run --rm dev go test ./... -v

clean:
	rm -rf bin/ dist/

UNAME_S := $(shell uname -s)
ifeq ($(findstring MINGW,$(UNAME_S)),MINGW)
  GOOS ?= windows
else ifeq ($(findstring MSYS,$(UNAME_S)),MSYS)
  GOOS ?= windows
else ifeq ($(findstring Darwin,$(UNAME_S)),Darwin)
  GOOS ?= darwin
else
  GOOS ?= linux
endif
GOARCH ?= $(if $(filter arm64 aarch64,$(shell uname -m)),arm64,amd64)
EXT := $(if $(filter windows,$(GOOS)),.exe,)
BINARY := bin/nvy$(EXT)

ifeq ($(GOOS),windows)
  INSTALL_DIR ?= $(LOCALAPPDATA)/Programs/nvy
else
  INSTALL_DIR ?= $(HOME)/.local/bin
endif

install:
	docker compose run --rm dev sh -c "CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build -ldflags='$(LDFLAGS)' -o $(BINARY) ."
	@mkdir -p "$(INSTALL_DIR)"
	cp $(BINARY) "$(INSTALL_DIR)/nvy$(EXT)"
	@echo "installed nvy $(VERSION) ($(GOOS)/$(GOARCH)) to $(INSTALL_DIR)/nvy$(EXT)"

# --- Release helpers ---
CURRENT_TAG := $(shell git describe --tags --abbrev=0 2>/dev/null || echo v0.0.0)
MAJOR := $(shell echo $(CURRENT_TAG) | sed 's/^v//' | cut -d. -f1)
MINOR := $(shell echo $(CURRENT_TAG) | sed 's/^v//' | cut -d. -f2)
PATCH := $(shell echo $(CURRENT_TAG) | sed 's/^v//' | cut -d. -f3)

release-patch:
	@NEXT=v$(MAJOR).$(MINOR).$(shell echo $$(($(PATCH)+1))); \
	echo "$(CURRENT_TAG) -> $$NEXT"; \
	git tag $$NEXT && git push origin $$NEXT && echo "released $$NEXT"

release-minor:
	@NEXT=v$(MAJOR).$(shell echo $$(($(MINOR)+1))).0; \
	echo "$(CURRENT_TAG) -> $$NEXT"; \
	git tag $$NEXT && git push origin $$NEXT && echo "released $$NEXT"

release-major:
	@NEXT=v$(shell echo $$(($(MAJOR)+1))).0.0; \
	echo "$(CURRENT_TAG) -> $$NEXT"; \
	git tag $$NEXT && git push origin $$NEXT && echo "released $$NEXT"

dist: cross
	docker compose run --rm dev sh -c "\
		mkdir -p dist && \
		tar czf dist/nvy-$(VERSION)-linux-amd64.tar.gz  -C bin nvy-linux-amd64  && \
		tar czf dist/nvy-$(VERSION)-linux-arm64.tar.gz  -C bin nvy-linux-arm64  && \
		tar czf dist/nvy-$(VERSION)-darwin-amd64.tar.gz -C bin nvy-darwin-amd64 && \
		tar czf dist/nvy-$(VERSION)-darwin-arm64.tar.gz -C bin nvy-darwin-arm64 && \
		zip dist/nvy-$(VERSION)-windows-amd64.zip bin/nvy-windows-amd64.exe     && \
		zip dist/nvy-$(VERSION)-windows-arm64.zip bin/nvy-windows-arm64.exe     && \
		cd dist && sha256sum * > checksums.sha256"
	@echo "dist:"
	@ls -1 dist/

cross:
	docker compose run --rm dev sh -c "\
		CGO_ENABLED=0 GOOS=linux   GOARCH=amd64 go build -ldflags='$(LDFLAGS)' -o bin/nvy-linux-amd64   . && \
		CGO_ENABLED=0 GOOS=linux   GOARCH=arm64 go build -ldflags='$(LDFLAGS)' -o bin/nvy-linux-arm64   . && \
		CGO_ENABLED=0 GOOS=darwin  GOARCH=amd64 go build -ldflags='$(LDFLAGS)' -o bin/nvy-darwin-amd64  . && \
		CGO_ENABLED=0 GOOS=darwin  GOARCH=arm64 go build -ldflags='$(LDFLAGS)' -o bin/nvy-darwin-arm64  . && \
		CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags='$(LDFLAGS)' -o bin/nvy-windows-amd64.exe . && \
		CGO_ENABLED=0 GOOS=windows GOARCH=arm64 go build -ldflags='$(LDFLAGS)' -o bin/nvy-windows-arm64.exe ."
