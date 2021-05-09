package commands

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"text/template"

	"github.com/p0tr3c/terra-ci/config"
	"github.com/p0tr3c/terra-ci/logs"
	"github.com/p0tr3c/terra-ci/templates"

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
		Use:           "workspace",
		Short:         "Creates new terragrunt workspace",
		PreRunE:       validateCreateWorkspaceArgs,
		RunE:          runCreateWorkspace,
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	SetCommandBuffers(command, in, out, outErr)

	command.Flags().String("path", "", "Full path to the workspace")
	command.MarkFlagRequired("path") //nolint
	command.Flags().String("module-location", "", "String referencing base of terragrunt module")
	return command
}

type TerragruntConfigParameters struct {
	ModuleLocation string
}

type WorkspaceCIConfigParameters struct {
	WorkspaceName                string
	WorkspacePath                string
	WorkspaceDefaultProdBranch   string
	WorkspaceTerragruntRunnerARN string
}

func validateCreateWorkspaceArgs(cmd *cobra.Command, args []string) error {
	// error early if path flag is not present
	val, err := cmd.Flags().GetString("path")
	if err != nil || val == "" {
		logs.Logger.Errorw("error while accessing flag",
			"flag", "path",
			"error", err)
		return UserInputError("path is required")
	}

	// error early if module location is not present
	val, err = cmd.Flags().GetString("module-location")
	if err != nil || val == "" {
		defaultLocation := config.Configuration.GetString("default_module_location")
		if defaultLocation == "" {
			logs.Logger.Errorw("missing module location to implement",
				"val", val,
				"defualt", defaultLocation,
				"error", err)
			return UserInputError("module-location is required")
		}
	}

	// error early if workspace directory is not present
	val, err = cmd.Flags().GetString("workspace-ci-dir")
	if err != nil || val == "" {
		defaultLocation := config.Configuration.GetString("default_ci_directory")
		if defaultLocation == "" {
			logs.Logger.Errorw("ci directory is missing",
				"val", val,
				"default", defaultLocation,
				"error", err)
			return UserInputError("workspace-ci-dir is required")
		}
	}

	return nil
}

func runCreateWorkspace(cmd *cobra.Command, args []string) error {
	logs.Logger.Debug("start")
	defer logs.Logger.Debug("end")

	workspacePath, err := cmd.Flags().GetString("path")
	if err != nil {
		logs.Logger.Errorw("error while accessing flag",
			"flag", "path",
			"error", err)
		cmd.PrintErrf("path flag is required")
		return err
	}

	workspaceName := filepath.Base(workspacePath)
	moduleLocation, err := cmd.Flags().GetString("module-location")
	if err != nil {
		moduleLocation = config.Configuration.GetString("default_module_location")
	}

	// Create workspace directory
	if err := os.MkdirAll(workspacePath, 0755); err != nil {
		logs.Logger.Errorw("failed to create workspace",
			"name", workspaceName,
			"path", workspacePath,
			"mode", 0755,
			"error", err)
		cmd.PrintErrf("failed to create workspace %s\n", workspaceName)
		return err
	}

	// Template workspace config
	inputParams := &TerragruntConfigParameters{
		ModuleLocation: moduleLocation,
	}
	tpl, err := template.New("terragruntConfig").Parse(templates.TerragruntWorkspaceConfig)
	if err != nil {
		logs.Logger.Errorw("failed to parse template",
			"error", err)
		cmd.PrintErrf("failed to create workspace %s\n", workspaceName)
		return err
	}
	var templatedTerragruntConfig bytes.Buffer
	if err := tpl.Execute(&templatedTerragruntConfig, inputParams); err != nil {
		logs.Logger.Errorw("failed to execute template",
			"error", err)
		cmd.PrintErrf("failed to create workspace %s\n", workspaceName)
		return err
	}
	if err := ioutil.WriteFile(filepath.Join(workspacePath, "terragrunt.hcl"),
		templatedTerragruntConfig.Bytes(), 0644); err != nil {
		logs.Logger.Errorw("failed to write terragrunt config",
			"path", workspacePath,
			"error", err)
		cmd.PrintErrf("failed to create workspace %s\n", workspaceName)
		return err
	}

	// Template workspace CI
	workspaceCiDirectory, _ := cmd.Flags().GetString("workspace-ci-dir")
	if workspaceCiDirectory == "" {
		workspaceCiDirectory = config.Configuration.GetString("default_ci_directory")
	}
	ciInputParams := &WorkspaceCIConfigParameters{
		WorkspaceName:                workspaceName,
		WorkspacePath:                workspacePath,
		WorkspaceDefaultProdBranch:   config.Configuration.GetString("default_workspace_prod_branch"),
		WorkspaceTerragruntRunnerARN: config.Configuration.GetString("state_machine_arn"),
	}
	tpl, err = template.New("ciConfig").Parse(templates.CiWorkspaceConfigTpl)
	if err != nil {
		logs.Logger.Errorw("failed to parse template",
			"name", "ciWorkspaceConfigTpl",
			"error", err)
		cmd.PrintErrf("failed to create workspace %s\n", workspaceName)
		return err
	}
	var templatedCiConfig bytes.Buffer
	if err := tpl.Execute(&templatedCiConfig, ciInputParams); err != nil {
		logs.Logger.Errorw("failed to execute template",
			"name", "ciWorkspaceConfigTpl",
			"error", err)
		cmd.PrintErrf("failed to create workspace %s\n", workspaceName)
		return err
	}
	if err := ioutil.WriteFile(filepath.Join(workspaceCiDirectory, fmt.Sprintf("run-%s.yml", workspaceName)),
		templatedCiConfig.Bytes(), 0644); err != nil {
		logs.Logger.Errorw("failed to write terragrunt config",
			"name", "ciWorkspaceConfigTpl",
			"path", workspaceCiDirectory,
			"error", err)
		cmd.PrintErrf("failed to create workspace %s\n", workspaceName)
		return err
	}

	cmd.Printf("workspace %s setup\n", workspaceName)
	return nil
}
