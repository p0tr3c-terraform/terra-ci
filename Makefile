PROJECTNAME=$(shell basename "$(PWD)")

GOLINT_VERSION="v1.34.1"

MAKEFLAGS += --silent

test_go_lint:
	@echo " > Running Go Lint $(GOLINT_VERSION)"
	@docker run --rm -v $(PWD):/app -w /app golangci/golangci-lint:$(GOLINT_VERSION) golangci-lint run -v

test_go: build
	@echo " > Running go test"
	@go test -v ./...

test: test_go_lint test_go

build:
	@echo " > Building binary"
	@go build -o bin/$(PROJECTNAME) .

compile: test
	@echo " > Compiling binary"
	@CGO=0 go build -o bin/$(PROJECTNAME)-linux-amd .

run: build
	@bin/$(PROJECTNAME)

clean:
	@echo " > Cleaning repo"
	@go clean
	@rm -rf ./bin
