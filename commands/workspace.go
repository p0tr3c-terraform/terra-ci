package commands

import (
	"fmt"
	"io"
	"path/filepath"

	"github.com/p0tr3c/terra-ci/config"
	"github.com/p0tr3c/terra-ci/logs"
	"github.com/p0tr3c/terra-ci/workspaces"

	"github.com/spf13/cobra"
)

var (
	workspaceFlags = WorkspaceFlags{
		Flags: []string{
			"path",
			"local",
			"source",
			"branch",
			"destroy",
			"out",
			"no-refresh",
			"ci-path",
		},
	}
)

type WorkspaceFlags struct {
	Flags []string
}

func (w WorkspaceFlags) Get(cmd *cobra.Command, args []string, flag string) (interface{}, error) {
	switch flag {
	case "path":
		return cmd.Flags().GetString("path")
	case "branch":
		if cmd.Use == "plan" || cmd.Use == "create" {
			return cmd.Flags().GetString("branch")
		} else {
			return "", nil
		}
	case "local":
		return cmd.Flags().GetBool("local")
	case "source":
		return cmd.Flags().GetString("source")
	case "out":
		return getOutPlan(cmd, args)
	case "destroy":
		destroy := false
		if cmd.Use == "plan" {
			return cmd.Flags().GetBool("destroy")
		}
		return destroy, nil
	case "no-refresh":
		noRefresh := false
		if cmd.Use == "plan" {
			return cmd.Flags().GetBool("no-refresh")
		}
		return noRefresh, nil
	case "ci-path":
		if cmd.Use == "create" {
			return cmd.Flags().GetString("ci-path")
		}
		return "", nil
	default:
		return nil, fmt.Errorf("unsupported flag %s", flag)
	}
}

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
	command.AddCommand(NewWorkspaceCreateCommand(in, out, outErr))
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
	}
	return outPlan, err
}

func getExecutionArn(cmd *cobra.Command, args []string) string {
	switch cmd.Use {
	case "apply":
		return config.Configuration.GetString("apply_sfn_arn")
	case "plan":
		return config.Configuration.GetString("plan_sfn_arn")
	default:
		return ""
	}
}

func getExecutionInput(cmd *cobra.Command, args []string) (*workspaces.WorkspaceExecutionInput, error) {
	inputConfig := make(map[string]interface{})
	var err error
	for _, flag := range workspaceFlags.Flags {
		inputConfig[flag], err = workspaceFlags.Get(cmd, args, flag)
		if err != nil {
			return nil, err
		}
	}

	input := &workspaces.WorkspaceExecutionInput{
		DestroyPlan:         inputConfig["destroy"].(bool),
		DisableRefreshState: inputConfig["no-refresh"].(bool),
		OutPlan:             inputConfig["out"].(string),
		Path:                inputConfig["path"].(string),
		Branch:              inputConfig["branch"].(string),
		Source:              config.Configuration.GetString("repository_url"),
		Location:            config.Configuration.GetString("repository_name"),
		Arn:                 getExecutionArn(cmd, args),
		ExecutionTimeout:    config.Configuration.GetDuration("sfn_execution_timeout"),
		RefreshRate:         config.Configuration.GetDuration("refresh_rate"),
		IsCi:                config.Configuration.GetBool("ci_mode"),
		Local:               inputConfig["local"].(bool),
		LocalModules:        inputConfig["source"].(string),
	}

	return input, nil
}

/*************************** PLAN ***************************************/

func NewWorkspacePlanCommand(in io.Reader, out, outErr io.Writer) *cobra.Command {
	command := &cobra.Command{
		Use:          "plan",
		Short:        "Run terraform plan on workspace",
		RunE:         runWorkspacePlan,
		SilenceUsage: true,
	}
	SetCommandBuffers(command, in, out, outErr)
	command.Flags().String("branch", "main", "Branch to execute workspace action")
	command.Flags().String("out", "", "Name of plan file to generate")
	command.Flags().Bool("destroy", false, "Generate destroy plan")
	command.Flags().Bool("no-refresh", false, "Disable state synchronization")
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
		Use:          "apply",
		Short:        "Run terraform apply on workspace",
		RunE:         runWorkspaceApply,
		SilenceUsage: true,
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

/*************************** CREATE ***************************************/

func NewWorkspaceCreateCommand(in io.Reader, out, outErr io.Writer) *cobra.Command {
	command := &cobra.Command{
		Use:   "create",
		Short: "Creates new terragrunt workspace",
		RunE:  runWorkspaceCreate,
	}
	SetCommandBuffers(command, in, out, outErr)

	command.Flags().String("path", "", "Full path to the workspace")
	command.MarkFlagRequired("path") //nolint
	command.Flags().String("branch", "main", "Branch to execute workspace action")
	command.Flags().String("ci-path", ".github/workflows", "Path to create github action")
	return command
}

func getCreateInput(cmd *cobra.Command, args []string) (*workspaces.WorkspaceCreateInput, error) {
	inputConfig := make(map[string]interface{})
	var err error
	for _, flag := range workspaceFlags.Flags {
		inputConfig[flag], err = workspaceFlags.Get(cmd, args, flag)
		if err != nil {
			return nil, err
		}
	}

	input := &workspaces.WorkspaceCreateInput{
		Name:     filepath.Base(inputConfig["path"].(string)),
		Path:     inputConfig["path"].(string),
		Branch:   inputConfig["branch"].(string),
		PlanArn:  config.Configuration.GetString("plan_sfn_arn"),
		ApplyArn: config.Configuration.GetString("apply_sfn_arn"),
		Module:   inputConfig["source"].(string),
		CiPath:   inputConfig["ci-path"].(string),
	}

	return input, nil
}

func runWorkspaceCreate(cmd *cobra.Command, args []string) error {
	createInput, err := getCreateInput(cmd, args)
	if err != nil {
		logs.Logger.Errorw("error while accessing flags",
			"error", err)
		cmd.PrintErrf("invalid create input")
		return err
	}
	if err := workspaces.CreateWorkspace(createInput); err != nil {
		logs.Logger.Errorw("failed to create workspace",
			"error", err)
		cmd.PrintErrf("failed to create workspace")
		return err
	}
	return nil
}

/*************************** FF SFN_MONITOR ***************************************/

func NewWorkspaceWithMontiorCommand(in io.Reader, out, outErr io.Writer) *cobra.Command {
	command := &cobra.Command{
		Use:          "workspace",
		Short:        "Manage terraform workspace",
		RunE:         workspaceMonitorPoC,
		SilenceUsage: true,
	}
	SetCommandBuffers(command, in, out, outErr)
	command.PersistentFlags().String("path", "", "Full path to the workspace")
	command.MarkFlagRequired("path") //nolint

	command.PersistentFlags().Bool("local", false, "Run action with localy")
	command.PersistentFlags().String("source", "", "Full path to local modules")
	command.Flags().String("branch", "main", "Branch to execute workspace action")
	command.Flags().String("out", "", "Name of plan file to generate")
	command.Flags().Bool("destroy", false, "Generate destroy plan")
	command.Flags().Bool("no-refresh", false, "Disable state synchronization")
	return command
}

func workspaceMonitorPoC(cmd *cobra.Command, args []string) error {
	executionInput, err := getExecutionInput(cmd, args)
	if err != nil {
		logs.Logger.Errorw("error while accessing flags",
			"error", err)
		cmd.PrintErrf("invalid execution input")
		return err
	}

	executionInput.Arn = config.Configuration.GetString("plan_sfn_arn")
	executionInput.Action = "plan"

	if err := workspaces.FFExecuteWorkspaceWithOutput(executionInput, cmd.InOrStdin(), cmd.OutOrStdout(), cmd.OutOrStderr()); err != nil {
		logs.Logger.Errorw("failed to execute workspace",
			"executionInput", executionInput,
			"error", err)
		cmd.PrintErrf("failed to execute workspace")
		return err
	}
	return nil
}

/*************************** FF SFN_MONITOR - END  ***************************************/
