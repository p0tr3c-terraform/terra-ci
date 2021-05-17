package modules

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

type ModuleExecutionInput struct {
	Path             string
	Branch           string
	Action           string
	Arn              string
	ExecutionTimeout time.Duration
	RefreshRate      time.Duration
	IsCi             bool
	Local            bool
	DisableCgo       bool
}

func ExecuteLocalModuleWithOutput(executionInput *ModuleExecutionInput, in io.Reader, out, outErr io.Writer) error {
	shellCommandArgs := []string{
		executionInput.Action,
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

func ExecuteModuleWithOutput(executionInput *ModuleExecutionInput, in io.Reader, out, outErr io.Writer) error {
	if executionInput.Local {
		if err := ExecuteLocalModuleWithOutput(executionInput, in, out, outErr); err != nil {
			return err
		}
	} else {
		fmt.Fprintf(out, "remote module execution not supported\n")
	}
	return nil
}
