PROJECTNAME=$(shell basename "$(PWD)")

MAKEFLAGS += --silent

test_go_lint:
	@docker run --rm -v $(PWD):/app -w /app golangci/golangci-lint:v1.34.1 golangci-lint run -v

test: test_go_lint

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
