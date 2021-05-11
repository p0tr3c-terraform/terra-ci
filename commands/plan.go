package commands

import (
	"io"

	"github.com/p0tr3c/terra-ci/config"
	"github.com/p0tr3c/terra-ci/logs"
	"github.com/p0tr3c/terra-ci/workspaces"

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
	command.Flags().String("branch", "main", "Branch to run plan on")
	return command
}

func getPlanFlagValues(cmd *cobra.Command, args []string) (*workspaces.WorkspaceExecutionInput, error) {
	workspacePath, err := cmd.Flags().GetString("path")
	if err != nil {
		return nil, err
	}
	workspaceBranch, err := cmd.Flags().GetString("branch")
	if err != nil {
		return nil, err
	}
	input := &workspaces.WorkspaceExecutionInput{
		Path:             workspacePath,
		Branch:           workspaceBranch,
		Action:           "plan",
		Arn:              config.Configuration.GetString("plan_sfn_arn"),
		ExecutionTimeout: config.Configuration.GetDuration("sfn_execution_timeout"),
		RefreshRate:      config.Configuration.GetDuration("refresh_rate"),
		IsCi:             config.Configuration.GetBool("ci_mode"),
	}
	return input, nil
}

func runPlanWorkspaceCommand(cmd *cobra.Command, args []string) error {
	executionInput, err := getPlanFlagValues(cmd, args)
	if err != nil {
		logs.Logger.Errorw("error while accessing flags",
			"error", err)
		cmd.PrintErrf("invalid execution input")
		return err
	}

	if err := workspaces.ExecuteRemoteWorkspaceWithOutput(executionInput, cmd.OutOrStdout(), cmd.OutOrStderr()); err != nil {
		logs.Logger.Errorw("failed to execute workspace",
			"executionInput", executionInput,
			"error", err)
		cmd.PrintErrf("failed to execute workspace")
		return err
	}
	return nil
}
