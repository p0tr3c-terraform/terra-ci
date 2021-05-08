package commands

import (
	"io"

	"github.com/p0tr3c/terra-ci/config"
	"github.com/p0tr3c/terra-ci/logs"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

func NewConfigCommand(in io.Reader, out, outErr io.Writer) *cobra.Command {
	command := &cobra.Command{
		Use:   "config",
		Short: "Manage configuration file",
		Run:   runHelp,
	}
	SetCommandBuffers(command, in, out, outErr)
	command.AddCommand(NewConfigViewCommand(in, out, outErr))
	return command
}

func NewConfigViewCommand(in io.Reader, out, outErr io.Writer) *cobra.Command {
	command := &cobra.Command{
		Use:   "view",
		Short: "View current configuration",
		Run:   runViewConfigCommand,
	}
	SetCommandBuffers(command, in, out, outErr)
	return command
}

func runViewConfigCommand(cmd *cobra.Command, args []string) {
	allSettings := config.Configuration.AllSettings()
	output, err := yaml.Marshal(allSettings)
	if err != nil {
		logs.Logger.Error("failed to marshal current settings",
			"error", err)
		cmd.PrintErrf("failed to print configuration\n")
		return
	}
	cmd.Printf("%s", output)
}
