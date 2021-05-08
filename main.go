package main

import (
	"math/rand"
	"os"
	"time"

	"github.com/p0tr3c/terra-ci/commands"
	"github.com/p0tr3c/terra-ci/logs"
)

func main() {
	rand.Seed(time.Now().UnixNano())

	command := commands.NewDefaultTerraCICommand()

	if err := logs.Init(); err != nil {
		os.Exit(1)
	}
	defer logs.Flush()

	if err := command.Execute(); err != nil {
		os.Exit(1)
	}
}
