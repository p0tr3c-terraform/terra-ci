package commands

import (
	"io"
	"os"

	"github.com/p0tr3c/terra-ci/config"
	"github.com/p0tr3c/terra-ci/fflags"
	"github.com/p0tr3c/terra-ci/logs"

	"github.com/spf13/cobra"
)

func NewDefaultTerraCICommand() *cobra.Command {
	return NewTerraCICommand(os.Stdin, os.Stdout, os.Stderr)
}

func SetCommandBuffers(cmd *cobra.Command, in io.Reader, out, outErr io.Writer) {
	cmd.SetIn(in)
	cmd.SetOut(out)
	cmd.SetErr(outErr)
}

func NewTerraCICommand(in io.Reader, out, outErr io.Writer) *cobra.Command {
	command := &cobra.Command{
		Use:              "terra-ci",
		Short:            "Manages and executes terragrunt remote actions",
		PersistentPreRun: readConfig,
		Run:              runHelp,
	}
	SetCommandBuffers(command, in, out, outErr)

	// Global flags
	config.AddConfigFlags(command)

	// Subcommands
	if fflags.IsEnabled("SFN_MONITOR") {
		command.AddCommand(NewWorkspaceWithMontiorCommand(in, out, outErr))
	} else {
		command.AddCommand(NewWorkspaceCommand(in, out, outErr))
		command.AddCommand(NewModuleCommand(in, out, outErr))
	}
	return command
}

func readConfig(cmd *cobra.Command, args []string) {
	if err := config.LoadConfig(cmd); err != nil {
		logs.Logger.Warn("failed to load configuration file",
			"path", config.ConfigFilePath,
			"error", err)
	}
	logs.UpdateLoggerConfig()
}

func runHelp(cmd *cobra.Command, args []string) {
	if err := cmd.Help(); err != nil {
		os.Exit(1)
	}
}
