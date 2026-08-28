.PHONY: build install test clean release-local

VERSION ?= 1.1.0
LDFLAGS = -ldflags="-s -w -X main.version=$(VERSION)"

build:
	go build $(LDFLAGS) -o bin/killport ./cmd/killport

install:
	go build $(LDFLAGS) -o $(HOME)/.local/bin/killport ./cmd/killport
	ln -sf $(HOME)/.local/bin/killport $(HOME)/.local/bin/kport
	@echo "Installed killport & kport to $(HOME)/.local/bin"

test:
	go test -v -race ./...

clean:
	rm -rf bin/ dist/

release-local:
	@mkdir -p dist
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o dist/killport_linux_amd64 ./cmd/killport
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o dist/killport_linux_arm64 ./cmd/killport
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o dist/killport_darwin_amd64 ./cmd/killport
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o dist/killport_darwin_arm64 ./cmd/killport
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o dist/killport_windows_amd64.exe ./cmd/killport
	@echo "Built release binaries in dist/"
