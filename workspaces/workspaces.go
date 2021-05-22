package workspaces

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/p0tr3c/terra-ci/aws"
	"github.com/p0tr3c/terra-ci/templates"
)

const (
	defaultDirectoryPermMode    = 0755
	defaultFilePermMode         = 0644
	defaultTerragruntConfigName = "terragrunt.hcl"
)

type WorkspaceCreateInput struct {
	Name     string
	Path     string
	Module   string
	Branch   string
	PlanArn  string
	ApplyArn string
	CiPath   string
}

func CreateWorkspaceDirecotry(inputConfig *WorkspaceCreateInput) error {
	if err := os.MkdirAll(inputConfig.Path, defaultDirectoryPermMode); err != nil {
		return err
	}
	return nil
}

func CreateWorkspaceConfig(inputConfig *WorkspaceCreateInput) error {
	tpl, err := template.New("terragruntConfig").Parse(templates.TerragruntWorkspaceConfig)
	if err != nil {
		return err
	}
	var templatedTerragruntConfig bytes.Buffer
	if err := tpl.Execute(&templatedTerragruntConfig, inputConfig); err != nil {
		return err
	}
	if err := ioutil.WriteFile(filepath.Join(inputConfig.Path, defaultTerragruntConfigName),
		templatedTerragruntConfig.Bytes(), defaultFilePermMode); err != nil {
		return err
	}
	return nil
}

func CreateWorkspaceCI(inputConfig *WorkspaceCreateInput) error {
	tpl, err := template.New("ciConfig").Parse(templates.CiWorkspaceConfigTpl)
	if err != nil {
		return err
	}
	var templatedCiConfig bytes.Buffer
	if err := tpl.Execute(&templatedCiConfig, inputConfig); err != nil {
		return err
	}
	if err := ioutil.WriteFile(filepath.Join(inputConfig.CiPath, fmt.Sprintf("workspace_%s.yml", strings.Join(strings.Split(inputConfig.Path, "/")[1:], "_"))),
		templatedCiConfig.Bytes(), defaultFilePermMode); err != nil {
		return err
	}
	return nil
}

func CreateWorkspace(input *WorkspaceCreateInput) error {
	if err := CreateWorkspaceDirecotry(input); err != nil {
		return err
	}

	if err := CreateWorkspaceConfig(input); err != nil {
		return err
	}

	if input.CiPath != "" {
		if err := CreateWorkspaceCI(input); err != nil {
			return err
		}
	}
	return nil
}

type WorkspaceExecutionInput struct {
	DestroyPlan         bool
	DisableRefreshState bool
	OutPlan             string
	Path                string
	Branch              string
	Source              string
	Location            string
	Action              string
	Arn                 string
	ExecutionTimeout    time.Duration
	RefreshRate         time.Duration
	IsCi                bool
	Local               bool
	LocalModules        string
}

func ExecuteRemoteWorkspaceWithOutput(executionInput *WorkspaceExecutionInput, out, outErr io.Writer) error {
	executionArn, err := aws.StartStateMachine(executionInput.Path,
		executionInput.Arn,
		executionInput.Source,
		executionInput.Location,
		executionInput.Action)
	if err != nil {
		return err
	}

	fmt.Fprintf(out, "execution %s started\n", executionArn)

	err = aws.MonitorStateMachineStatus(executionArn,
		executionInput.RefreshRate,
		executionInput.ExecutionTimeout,
		executionInput.IsCi, out, outErr)
	if err != nil {
		return err
	}
	return nil
}

func ExecuteLocalWorkspaceWithOutput(executionInput *WorkspaceExecutionInput, in io.Reader, out, outErr io.Writer) error {
	shellCommandArgs := []string{
		executionInput.Action,
	}
	if executionInput.OutPlan != "" {
		switch executionInput.Action {
		case "plan":
			shellCommandArgs = append(shellCommandArgs, []string{
				"-out",
				executionInput.OutPlan,
			}...)
		case "apply":
			shellCommandArgs = append(shellCommandArgs, []string{
				executionInput.OutPlan,
			}...)
		}
	}
	if executionInput.DestroyPlan {
		shellCommandArgs = append(shellCommandArgs, "-destroy")
	}
	if executionInput.DisableRefreshState && executionInput.Action == "plan" {
		shellCommandArgs = append(shellCommandArgs, "-refresh=false")
	}
	if executionInput.IsCi && executionInput.Action == "apply" && executionInput.OutPlan == "" {
		shellCommandArgs = append(shellCommandArgs, "-auto-approve")
	}
	if executionInput.LocalModules != "" {
		shellCommandArgs = append(shellCommandArgs, []string{
			"--terragrunt-source",
			executionInput.LocalModules,
		}...)
	}
	shellCommand := exec.Command("terragrunt", shellCommandArgs...)
	workspaceAbsPath, err := filepath.Abs(executionInput.Path)
	if err != nil {
		return err
	}
	shellCommand.Dir = workspaceAbsPath
	shellCommand.Stdin = in
	shellCommand.Stdout = out
	shellCommand.Stderr = outErr

	if err := shellCommand.Start(); err != nil {
		return err
	}

	if err := shellCommand.Wait(); err != nil {
		return err
	}

	return nil
}

func ExecuteWorkspaceWithOutput(executionInput *WorkspaceExecutionInput, in io.Reader, out, outErr io.Writer) error {
	if executionInput.Local {
		if err := ExecuteLocalWorkspaceWithOutput(executionInput, in, out, outErr); err != nil {
			return err
		}
	} else {
		if err := ExecuteRemoteWorkspaceWithOutput(executionInput, out, outErr); err != nil {
			return err
		}
	}
	return nil
}
