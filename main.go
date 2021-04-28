package main

import (
	"log"

	"github.com/p0tr3c/terra-ci/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		log.Fatalf("%s\n", err.Error())
	}
}
