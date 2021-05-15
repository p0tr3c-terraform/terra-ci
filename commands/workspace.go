package commands

import (
	"fmt"
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
	command.PersistentFlags().String("path", "", "Full path to the workspace")
	command.MarkFlagRequired("path") //nolint

	command.PersistentFlags().Bool("local", false, "Run action with localy")
	command.PersistentFlags().String("source", "", "Full path to local modules")

	command.AddCommand(NewWorkspacePlanCommand(in, out, outErr))
	command.AddCommand(NewWorkspaceApplyCommand(in, out, outErr))
	return command
}

func getOutPlan(cmd *cobra.Command, args []string) (string, error) {
	var outPlan string
	var err error
	switch cmd.Use {
	case "plan":
		outPlan, err = cmd.Flags().GetString("out")
	case "apply":
		if len(args) == 1 {
			outPlan = args[0]
		}
	default:
		err = fmt.Errorf("unsupported command %s", cmd.Use)
	}
	return outPlan, err
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
	localModules, err := cmd.Flags().GetString("source")
	if err != nil {
		return nil, err
	}

	outPlan, err := getOutPlan(cmd, args)
	if err != nil {
		return nil, err
	}

	destroy := false
	if cmd.Use == "plan" {
		destroy, err = cmd.Flags().GetBool("destroy")
		if err != nil {
			return nil, err
		}
	}

	input := &workspaces.WorkspaceExecutionInput{
		DestroyPlan:      destroy,
		OutPlan:          outPlan,
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

/*************************** PLAN ***************************************/

func NewWorkspacePlanCommand(in io.Reader, out, outErr io.Writer) *cobra.Command {
	command := &cobra.Command{
		Use:   "plan",
		Short: "Run terraform plan on workspace",
		RunE:  runWorkspacePlan,
	}
	SetCommandBuffers(command, in, out, outErr)
	command.Flags().String("branch", "main", "Branch to execute workspace action")
	command.Flags().String("out", "", "Name of plan file to generate")
	command.Flags().Bool("destroy", false, "Generate destroy plan")
	return command
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

/*************************** APPLY ***************************************/

func NewWorkspaceApplyCommand(in io.Reader, out, outErr io.Writer) *cobra.Command {
	command := &cobra.Command{
		Use:   "apply",
		Short: "Run terraform apply on workspace",
		RunE:  runWorkspaceApply,
	}
	SetCommandBuffers(command, in, out, outErr)
	return command
}

func runWorkspaceApply(cmd *cobra.Command, args []string) error {
	executionInput, err := getExecutionInput(cmd, args)
	if err != nil {
		logs.Logger.Errorw("error while accessing flags",
			"error", err)
		cmd.PrintErrf("invalid execution input")
		return err
	}

	executionInput.Action = "apply"

	if err := workspaces.ExecuteWorkspaceWithOutput(executionInput, cmd.InOrStdin(), cmd.OutOrStdout(), cmd.OutOrStderr()); err != nil {
		logs.Logger.Errorw("failed to execute workspace",
			"executionInput", executionInput,
			"error", err)
		cmd.PrintErrf("failed to execute workspace")
		return err
	}
	return nil
}
