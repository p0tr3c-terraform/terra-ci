package commands

import (
	"fmt"
	"io"

	"github.com/p0tr3c/terra-ci/config"
	"github.com/p0tr3c/terra-ci/logs"
	"github.com/p0tr3c/terra-ci/modules"

	"github.com/spf13/cobra"
)

var (
	moduleFlags = ModuleFlags{
		Flags: []string{
			"path",
			"local",
			"branch",
			"disable-cgo",
			"timeout",
			"run",
		},
	}
)

type ModuleFlags struct {
	Flags []string
}

func (w ModuleFlags) Get(cmd *cobra.Command, args []string, flag string) (interface{}, error) {
	switch flag {
	case "path":
		return cmd.Flags().GetString("path")
	case "branch":
		return cmd.Flags().GetString("branch")
	case "local":
		return cmd.Flags().GetBool("local")
	case "disable-cgo":
		return cmd.Flags().GetBool("disable-cgo")
	case "timeout":
		return cmd.Flags().GetString("timeout")
	case "run":
		return cmd.Flags().GetString("run")
	default:
		return nil, fmt.Errorf("unsupported flag %s", flag)
	}
}

func NewModuleCommand(in io.Reader, out, outErr io.Writer) *cobra.Command {
	command := &cobra.Command{
		Use:   "module",
		Short: "Manage terraform modules",
		Run:   runHelp,
	}
	SetCommandBuffers(command, in, out, outErr)
	command.PersistentFlags().String("path", "", "Full path to the module")
	command.MarkFlagRequired("path") //nolint

	command.PersistentFlags().Bool("local", false, "Run action with localy")
	command.PersistentFlags().Bool("disable-cgo", false, "Disable CGO")

	command.AddCommand(NewModuleTestCommand(in, out, outErr))
	return command
}

func getModuleExecutionInput(cmd *cobra.Command, args []string) (*modules.ModuleExecutionInput, error) {
	inputConfig := make(map[string]interface{})
	var err error
	for _, flag := range moduleFlags.Flags {
		inputConfig[flag], err = moduleFlags.Get(cmd, args, flag)
		if err != nil {
			return nil, err
		}
	}

	input := &modules.ModuleExecutionInput{
		Path:             inputConfig["path"].(string),
		Source:           config.Configuration.GetString("repository_url"),
		Location:         config.Configuration.GetString("repository_name"),
		Branch:           inputConfig["branch"].(string),
		Arn:              config.Configuration.GetString("test_sfn_arn"),
		TestTimeout:      inputConfig["timeout"].(string),
		Run:              inputConfig["run"].(string),
		ExecutionTimeout: config.Configuration.GetDuration("sfn_execution_timeout"),
		RefreshRate:      config.Configuration.GetDuration("refresh_rate"),
		IsCi:             config.Configuration.GetBool("ci_mode"),
		Local:            inputConfig["local"].(bool),
		DisableCgo:       inputConfig["disable-cgo"].(bool),
	}

	return input, nil
}

/*************************** TEST ***************************************/

func NewModuleTestCommand(in io.Reader, out, outErr io.Writer) *cobra.Command {
	command := &cobra.Command{
		Use:          "test",
		Short:        "Run terratest on module",
		RunE:         runModuleTest,
		SilenceUsage: true,
	}
	SetCommandBuffers(command, in, out, outErr)
	command.Flags().String("branch", "main", "Branch to execute module test on")
	command.Flags().String("timeout", "5m", "Test timeout, default '5m'")
	command.Flags().String("run", "", "Specific test to run")
	return command
}

func runModuleTest(cmd *cobra.Command, args []string) error {
	executionInput, err := getModuleExecutionInput(cmd, args)
	if err != nil {
		logs.Logger.Errorw("error while accessing flags",
			"error", err)
		cmd.PrintErrf("invalid execution input")
		return err
	}

	executionInput.Action = "test"

	if err := modules.ExecuteModuleWithOutput(executionInput, cmd.InOrStdin(), cmd.OutOrStdout(), cmd.OutOrStderr()); err != nil {
		logs.Logger.Errorw("failed to execute workspace",
			"executionInput", executionInput,
			"error", err)
		cmd.PrintErrf("failed to execute workspace")
		return err
	}
	return nil
}
