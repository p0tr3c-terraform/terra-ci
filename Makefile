PROJECTNAME=$(shell basename "$(PWD)")
RELEASE_DIR ?= _release/$(VERSION)

GOLINT_VERSION="v1.40.1"

MAKEFLAGS += --silent

#------------ TESTS -------------#

test_go_lint:
	@echo " > Running Go Lint $(GOLINT_VERSION)"
	@docker run --rm -v $(PWD):/app -w /app golangci/golangci-lint:$(GOLINT_VERSION) golangci-lint run -v

test_go: build
	@echo " > Running go test"
	@go test -v ./...

test: test_go_lint test_go

#------------ BUILD -------------#

build:
	@echo " > Building binary"
	@go build -o bin/$(PROJECTNAME) .

compile: test
	@echo " > Compiling binary"
	@CGO=0 go build -o bin/$(PROJECTNAME)-linux-amd .


#------------ RUN -------------#

run: build
	@bin/$(PROJECTNAME)


#------------ RELEASE -------------#

ensure_release_dir:
	@mkdir -p $(RELEASE_DIR)

release: compile ensure_release_dir
	mv bin/$(PROJECTNAME)-linux-amd $(RELEASE_DIR)/

#------------ CLEAN -------------#

clean:
	@echo " > Cleaning repo"
	@go clean
	@rm -rf ./bin
