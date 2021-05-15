package commands

import (
	"io"

	"github.com/p0tr3c/terra-ci/config"
	"github.com/p0tr3c/terra-ci/logs"
	"github.com/p0tr3c/terra-ci/workspaces"

	"github.com/spf13/cobra"
)

func NewApplyCommand(in io.Reader, out, outErr io.Writer) *cobra.Command {
	command := &cobra.Command{
		Use:   "apply",
		Short: "Apply terragrunt resources",
		Run:   runHelp,
	}
	SetCommandBuffers(command, in, out, outErr)
	command.AddCommand(NewApplyWorkspaceCommand(in, out, outErr))
	return command
}

func NewApplyWorkspaceCommand(in io.Reader, out, outErr io.Writer) *cobra.Command {
	command := &cobra.Command{
		Use:   "workspace",
		Short: "Apply terragrunt workspace",
		RunE:  runApplyWorkspaceCommand,
	}
	SetCommandBuffers(command, in, out, outErr)

	command.Flags().String("path", "", "Full path to the workspace")
	command.MarkFlagRequired("path") //nolint
	command.Flags().Bool("local", false, "Run apply locally")
	command.Flags().String("modules", "./modules//", "Full path to local modules")
	return command
}

func getApplyExecutionInput(cmd *cobra.Command, args []string) (*workspaces.WorkspaceExecutionInput, error) {
	workspacePath, err := cmd.Flags().GetString("path")
	if err != nil {
		return nil, err
	}
	local, err := cmd.Flags().GetBool("local")
	if err != nil {
		return nil, err
	}
	localModules, err := cmd.Flags().GetString("modules")
	if err != nil {
		return nil, err
	}
	input := &workspaces.WorkspaceExecutionInput{
		Path:             workspacePath,
		Action:           "apply",
		Arn:              config.Configuration.GetString("apply_sfn_arn"),
		ExecutionTimeout: config.Configuration.GetDuration("sfn_execution_timeout"),
		RefreshRate:      config.Configuration.GetDuration("refresh_rate"),
		IsCi:             config.Configuration.GetBool("ci_mode"),
		Local:            local,
		LocalModules:     localModules,
	}
	return input, nil
}

func runApplyWorkspaceCommand(cmd *cobra.Command, args []string) error {
	executionInput, err := getApplyExecutionInput(cmd, args)
	if err != nil {
		logs.Logger.Errorw("error while accessing flags",
			"error", err)
		cmd.PrintErrf("invalid execution input")
		return err
	}

	if err := workspaces.ExecuteWorkspaceWithOutput(executionInput, cmd.InOrStdin(), cmd.OutOrStdout(), cmd.OutOrStderr()); err != nil {
		logs.Logger.Errorw("failed to execute workspace",
			"executionInput", executionInput,
			"error", err)
		cmd.PrintErrf("failed to execute workspace")
		return err
	}
	return nil
}
