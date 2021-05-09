package commands

import (
	"io"

	"github.com/p0tr3c/terra-ci/config"
	"github.com/p0tr3c/terra-ci/logs"
	"github.com/p0tr3c/terra-ci/workspaces"

	"github.com/spf13/cobra"
)

func NewRunCommand(in io.Reader, out, outErr io.Writer) *cobra.Command {
	command := &cobra.Command{
		Use:   "run",
		Short: "Runs remote terragrunt actions",
		Run:   runHelp,
	}
	SetCommandBuffers(command, in, out, outErr)
	command.AddCommand(NewRunWorkspaceCommand(in, out, outErr))
	return command
}

func NewRunWorkspaceCommand(in io.Reader, out, outErr io.Writer) *cobra.Command {
	command := &cobra.Command{
		Use:           "workspace",
		Short:         "Runs terragrunt workspace",
		Args:          validateRunWorkspaceArgs,
		RunE:          runRunWorkspace,
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	SetCommandBuffers(command, in, out, outErr)

	command.Flags().String("action", "plan", "Action to trigger on remote worker")
	command.Flags().String("path", "", "Full path to the workspace")
	command.MarkFlagRequired("path") //nolint
	command.Flags().String("module-location", "", "String referencing base of terragrunt module")
	command.Flags().String("branch", "main", "Default git branch to use for deployment")
	return command
}

func validateRunWorkspaceArgs(cmd *cobra.Command, args []string) error {
	// error early if path flag is not present
	val, err := cmd.Flags().GetString("path")
	if err != nil || val == "" {
		logs.Logger.Errorw("error while accessing flag",
			"flag", "path",
			"error", err)
		return UserInputError("path is required")
	}

	// error early if action flag is missing or incorrect
	val, err = cmd.Flags().GetString("action")
	if err != nil {
		logs.Logger.Errorw("error while accessing flag",
			"flag", "action",
			"error", err)
		return UserInputError("action is required")
	}
	switch val {
	case "apply":
	case "plan":
	default:
		logs.Logger.Errorw("invalid action flag",
			"flag", "action",
			"value", val,
			"error", err)
		return UserInputError("action must be one of [apply, plan]")
	}
	return nil
}

func runRunWorkspace(cmd *cobra.Command, args []string) error {
	workspacePath, err := cmd.Flags().GetString("path")
	if err != nil {
		logs.Logger.Errorw("error while accessing flag",
			"flag", "path",
			"error", err)
		cmd.PrintErrf("path flag is required")
		return err
	}
	workspaceBranch, err := cmd.Flags().GetString("branch")
	if err != nil {
		logs.Logger.Errorw("error while accessing flag",
			"flag", "branch",
			"error", err)
		cmd.PrintErrf("branch flag is required")
		return err
	}
	workspaceAction, err := cmd.Flags().GetString("action")
	if err != nil {
		logs.Logger.Errorw("error while accessing flag",
			"flag", "action",
			"error", err)
		cmd.PrintErrf("action flag is required")
		return err
	}

	workspaceArn := config.Configuration.GetString("state_machine_arn")
	executionTimeout := config.Configuration.GetDuration("sfn_execution_timeout")
	refreshRate := config.Configuration.GetDuration("refresh_rate")
	isCi := config.Configuration.GetBool("ci_mode")

	if err := workspaces.ExecuteRemoteWorkspaceWithOutput(workspacePath, workspaceBranch, workspaceAction, workspaceArn, refreshRate, executionTimeout, isCi, cmd.OutOrStdout(), cmd.OutOrStderr()); err != nil {
		logs.Logger.Errorw("failed to execute workspace",
			"path", workspacePath,
			"branch", workspaceBranch,
			"action", workspaceAction,
			"isCi", isCi,
			"refresh_rate", refreshRate,
			"execution_timeout", executionTimeout,
			"error", err)
		cmd.PrintErrf("failed to execute workspace")
		return err
	}
	return nil
}
