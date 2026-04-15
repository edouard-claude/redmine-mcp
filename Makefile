BINARY := redmine-mcp
VERSION := 2.0.0

.PHONY: build install clean

build:
	go build -ldflags "-X main.version=$(VERSION)" -o $(BINARY) ./cmd/redmine-mcp/

install:
	go build -ldflags "-X main.version=$(VERSION)" -o /usr/local/bin/$(BINARY) ./cmd/redmine-mcp/

clean:
	rm -f $(BINARY)
