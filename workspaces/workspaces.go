package workspaces

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
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

func CreateWorkspaceDirecotry(name, path string) error {
	if err := os.MkdirAll(path, defaultDirectoryPermMode); err != nil {
		return err
	}
	return nil
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

func CreateWorkspaceConfig(workspacePath, modulePath string) error {
	inputParams := &TerragruntConfigParameters{
		ModuleLocation: modulePath,
	}
	tpl, err := template.New("terragruntConfig").Parse(templates.TerragruntWorkspaceConfig)
	if err != nil {
		return err
	}
	var templatedTerragruntConfig bytes.Buffer
	if err := tpl.Execute(&templatedTerragruntConfig, inputParams); err != nil {
		return err
	}
	if err := ioutil.WriteFile(filepath.Join(workspacePath, defaultTerragruntConfigName),
		templatedTerragruntConfig.Bytes(), defaultFilePermMode); err != nil {
		return err
	}
	return nil
}

func CreateWorkspaceCI(name, workspacePath, branch, arn, ciPath string) error {
	ciInputParams := &WorkspaceCIConfigParameters{
		WorkspaceName:                name,
		WorkspacePath:                workspacePath,
		WorkspaceDefaultProdBranch:   branch,
		WorkspaceTerragruntRunnerARN: arn,
	}
	tpl, err := template.New("ciConfig").Parse(templates.CiWorkspaceConfigTpl)
	if err != nil {
		return err
	}
	var templatedCiConfig bytes.Buffer
	if err := tpl.Execute(&templatedCiConfig, ciInputParams); err != nil {
		return err
	}
	if err := ioutil.WriteFile(filepath.Join(ciPath, fmt.Sprintf("%s.yml", strings.ReplaceAll(workspacePath, "/", "_"))),
		templatedCiConfig.Bytes(), defaultFilePermMode); err != nil {
		return err
	}
	return nil
}

func ExecuteRemoteWorkspaceWithOutput(path, branch, action, arn string, refreshRate, executionTimeout time.Duration, isCi bool, out, outErr io.Writer) error {
	executionArn, err := aws.StartStateMachine(path, arn, branch, action)
	if err != nil {
		return err
	}

	executionStatus, err := aws.MonitorStateMachineStatus(executionArn, refreshRate, executionTimeout, isCi, out, outErr)
	if err != nil {
		return err
	}

	logInformation, err := aws.GetCloudwatchLogsReference(executionStatus)
	if err != nil {
		return err
	}

	if err := aws.StreamCloudwatchLogs(out, logInformation.Build.Logs.GroupName, logInformation.Build.Logs.StreamName); err != nil {
		return err
	}
	return nil
}
