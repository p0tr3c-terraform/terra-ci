package commands

import (
	"io"

	"github.com/spf13/cobra"
)

func NewPlanCommand(in io.Reader, out, outErr io.Writer) *cobra.Command {
	command := &cobra.Command{
		Use:   "plan",
		Short: "Plan terragrunt resources",
		Run:   runHelp,
	}
	SetCommandBuffers(command, in, out, outErr)
	command.AddCommand(NewPlanWorkspaceCommand(in, out, outErr))
	return command
}

func NewPlanWorkspaceCommand(in io.Reader, out, outErr io.Writer) *cobra.Command {
	command := &cobra.Command{
		Use:   "workspace",
		Short: "Plan terragrunt workspace",
		RunE:  runPlanWorkspaceCommand,
	}
	SetCommandBuffers(command, in, out, outErr)

	command.Flags().String("path", "", "Full path to the workspace")
	command.MarkFlagRequired("path") //nolint
	return command
}

func runPlanWorkspaceCommand(cmd *cobra.Command, args []string) error {
	return nil
}
