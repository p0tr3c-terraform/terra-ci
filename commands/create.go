package commands

import (
	"io"
	"os"

	"github.com/p0tr3c/terra-ci/logs"

	"github.com/spf13/cobra"
)

type UserInputError string

func (e UserInputError) Error() string {
	return string(e)
}

const (
	ErrPositionalArgument UserInputError = "invalid positional argument"
)

func NewCreateCommand(in io.Reader, out, outErr io.Writer) *cobra.Command {
	command := &cobra.Command{
		Use:   "create",
		Short: "Creates terra-ci resource type",
		Run:   runHelp,
	}
	SetCommandBuffers(command, in, out, outErr)
	command.AddCommand(NewCreateWorkspaceCommand(in, out, outErr))
	return command
}

func NewCreateWorkspaceCommand(in io.Reader, out, outErr io.Writer) *cobra.Command {
	command := &cobra.Command{
		Use:   "workspace",
		Short: "Creates new terragrunt workspace",
		Run:   runCreateWorkspace,
	}
	SetCommandBuffers(command, in, out, outErr)

	command.Flags().String("path", "", "Full path to the workspace")
	command.MarkFlagRequired("path")
	return command
}

func runCreateWorkspace(cmd *cobra.Command, args []string) {
	logs.Logger.Debug("start")
	defer logs.Logger.Debug("end")

	// Create workspace directory
	workspacePath, err := cmd.Flags().GetString("path")
	if err != nil {
		logs.Logger.Error("path flag is missing")
		return
	}
	if err := os.MkdirAll(workspacePath, 0755); err != nil {
		logs.Logger.Debug("failed to create workspace",
			"path", workspacePath,
			"mode", 0755,
			"error", err)
		cmd.Printf("failed to create workspace %s\n", args[0])
		return
	}
	// Template workspace config
	// Template workspace CI
}
