package modules

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/p0tr3c/terra-ci/aws"
)

type ModuleExecutionInput struct {
	Path             string
	Source           string
	Location         string
	Branch           string
	Action           string
	Arn              string
	Run              string
	TestTimeout      string
	ExecutionTimeout time.Duration
	RefreshRate      time.Duration
	IsCi             bool
	Local            bool
	DisableCgo       bool
}

func ExecuteLocalModuleWithOutput(executionInput *ModuleExecutionInput, in io.Reader, out, outErr io.Writer) error {
	shellCommandArgs := []string{
		executionInput.Action,
		"-timeout",
		executionInput.TestTimeout,
		"-v",
	}
	if executionInput.Run != "" {
		shellCommandArgs = append(shellCommandArgs, []string{
			"-run",
			executionInput.Run,
		}...)
	}
	shellCommand := exec.Command("go", shellCommandArgs...)
	shellCommand.Env = os.Environ()
	if executionInput.DisableCgo {
		shellCommand.Env = append(shellCommand.Env, "CGO_ENABLED=0")
	}
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

func ExecuteRemoteModuleWithOutput(executionInput *ModuleExecutionInput, out, outErr io.Writer) error {
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

func ExecuteModuleWithOutput(executionInput *ModuleExecutionInput, in io.Reader, out, outErr io.Writer) error {
	if executionInput.Local {
		if err := ExecuteLocalModuleWithOutput(executionInput, in, out, outErr); err != nil {
			return err
		}
	} else {
		if err := ExecuteRemoteModuleWithOutput(executionInput, out, outErr); err != nil {
			return err
		}
	}
	return nil
}
