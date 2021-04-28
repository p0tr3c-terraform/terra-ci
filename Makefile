PROJECTNAME=$(shell basename "$(PWD)")

MAKEFLAGS += --silent

build:
	@echo " > Building binary"
	@go build -o bin/$(PROJECTNAME) .

compile:
	@echo " > Compiling binary"
	@CGO=0 go build -o bin/$(PROJECTNAME)-linux-amd .

run: build
	@bin/$(PROJECTNAME)

clean:
	@echo " > Cleaning repo"
	@go clean
	@rm -rf ./bin
