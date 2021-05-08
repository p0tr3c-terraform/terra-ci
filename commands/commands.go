package commands

import (
	"github.com/spf13/cobra"
	"io"
	"os"
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
		Use:   "terra-ci",
		Short: "Manages and executes terragrunt remote actions",
		Run:   runHelp,
	}

	command.AddCommand(NewCreateCommand(in, out, outerr))
	return command
}

func runHelp(cmd *cobra.Command, args []string) {
	if err := cmd.Help(); err != nil {
		os.Exit(1)
	}
}
