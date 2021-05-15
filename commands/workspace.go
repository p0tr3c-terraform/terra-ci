package commands

import (
	"io"

	"github.com/p0tr3c/terra-ci/config"
	"github.com/p0tr3c/terra-ci/logs"
	"github.com/p0tr3c/terra-ci/workspaces"

	"github.com/spf13/cobra"
)

func NewWorkspaceCommand(in io.Reader, out, outErr io.Writer) *cobra.Command {
	command := &cobra.Command{
		Use:   "workspace",
		Short: "Manage terraform workspace",
		Run:   runHelp,
	}
	SetCommandBuffers(command, in, out, outErr)
	command.Flags().String("path", "", "Full path to the workspace")
	command.MarkFlagRequired("path") //nolint

	command.Flags().Bool("local", false, "Run action with localy")
	command.Flags().String("modules", "./modules//", "Full path to local modules")
	command.AddCommand(NewWorkspacePlanCommand(in, out, outErr))
	return command
}

func NewWorkspacePlanCommand(in io.Reader, out, outErr io.Writer) *cobra.Command {
	command := &cobra.Command{
		Use:   "plan",
		Short: "Run terraform plan on workspace",
		RunE:  runWorkspacePlan,
	}
	SetCommandBuffers(command, in, out, outErr)
	command.Flags().String("branch", "main", "Branch to execute workspace action")
	return command
}

func getExecutionInput(cmd *cobra.Command, args []string) (*workspaces.WorkspaceExecutionInput, error) {
	workspacePath, err := cmd.Flags().GetString("path")
	if err != nil {
		return nil, err
	}
	workspaceBranch, err := cmd.Flags().GetString("branch")
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
		Branch:           workspaceBranch,
		Arn:              config.Configuration.GetString("plan_sfn_arn"),
		ExecutionTimeout: config.Configuration.GetDuration("sfn_execution_timeout"),
		RefreshRate:      config.Configuration.GetDuration("refresh_rate"),
		IsCi:             config.Configuration.GetBool("ci_mode"),
		Local:            local,
		LocalModules:     localModules,
	}
	return input, nil
}

func runWorkspacePlan(cmd *cobra.Command, args []string) error {
	executionInput, err := getExecutionInput(cmd, args)
	if err != nil {
		logs.Logger.Errorw("error while accessing flags",
			"error", err)
		cmd.PrintErrf("invalid execution input")
		return err
	}

	executionInput.Action = "plan"

	if err := workspaces.ExecuteWorkspaceWithOutput(executionInput, cmd.InOrStdin(), cmd.OutOrStdout(), cmd.OutOrStderr()); err != nil {
		logs.Logger.Errorw("failed to execute workspace",
			"executionInput", executionInput,
			"error", err)
		cmd.PrintErrf("failed to execute workspace")
		return err
	}
	return nil
}