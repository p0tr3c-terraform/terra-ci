package main

import (
	"math/rand"
	"os"
	"time"

	"github.com/p0tr3c/terra-ci/cmd"
	"github.com/p0tr3c/terra-ci/logs"
)

func main() {
	rand.Seed(time.Now().UnixNano())

	command := cmd.NewDefaultTerraCICommand()

	logs.Init()
	defer logs.Flush()

	if err := command.Execute(); err != nil {
		os.Exit(1)
	}
}
