SHELL := bash
.PHONY: $(MAKECMDGOALS)

install-bd:
	go install github.com/afansv/bd@latest

build:
	go build github.com/afansv/bd .

toolset:
	bd install

lint:
	bd exec golangci-lint run

lint.fix:
	bd exec golangci-lint run --fix