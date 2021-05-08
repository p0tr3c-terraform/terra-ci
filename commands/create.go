package commands

import (
	"github.com/spf13/cobra"
	"io"
)

func NewCreateCommand(in io.Reader, out, outerr io.Writer) *cobra.Command {
	command := &cobra.Command{
		Use:   "create",
		Short: "Creates terra-ci resource type",
		Run:   runHelp,
	}
	command.AddCommand(NewCreateWorkspaceCommand(in, out, outerr))
	return command
}

func NewCreateWorkspaceCommand(in io.Reader, out, outerr io.Writer) *cobra.Command {
	command := &cobra.Command{
		Use:   "workspace",
		Short: "Creates new terragrunt workspace",
		Run:   runCreateWorkspace,
	}
	return command
}

func runCreateWorkspace(cmd *cobra.Command, args []string) {
}
