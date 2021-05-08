package commands

import (
	"io"
	"os"

	"github.com/p0tr3c/terra-ci/config"

	"github.com/spf13/cobra"
)

var ()

const ()

func init() {
}

func NewDefaultTerraCICommand() *cobra.Command {
	return NewTerraCICommand(os.Stdin, os.Stdout, os.Stderr)
}

func NewTerraCICommand(in io.Reader, out, outerr io.Writer) *cobra.Command {
	command := &cobra.Command{
		Use:              "terra-ci",
		Short:            "Manages and executes terragrunt remote actions",
		PersistentPreRun: readConfig,
		Run:              runHelp,
	}

	// Global flags
	config.AddConfigFlags(command)

	// Subcommands
	command.AddCommand(NewCreateCommand(in, out, outerr))

	return command
}

func readConfig(cmd *cobra.Command, args []string) {
	config.LoadConfig(cmd)
}

func runHelp(cmd *cobra.Command, args []string) {
	if err := cmd.Help(); err != nil {
		os.Exit(1)
	}
}
