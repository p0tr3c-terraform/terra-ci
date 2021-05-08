package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"text/template"
	"time"

	"github.com/p0tr3c/terra-ci/config"
	"github.com/p0tr3c/terra-ci/logs"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs/cloudwatchlogsiface"
	"github.com/aws/aws-sdk-go/service/sfn"
	"github.com/aws/aws-sdk-go/service/sfn/sfniface"
	"github.com/briandowns/spinner"
	"github.com/spf13/cobra"
)

const (
	stateMachineInputTpl = `
{
    "Comment": "Run from CLI",
    "build": {
	  "sourceversion": "{{ .Branch }}",
	  "action": "{{ .Action }}",
      "environment": {
        "terra_ci_resource": "{{ .Resource }}"
      }
    }
}
`
)

type InternalError string

func (e InternalError) Error() string {
	return string(e)
}

type SfnInputParameters struct {
	Resource string
	Branch   string
	Action   string
}

type ExecutionOutput struct {
	Build ExecutionOutputBuild `json:"Build"`
}

type ExecutionOutputBuild struct {
	Logs ExecutionOutputBuildLogs `json:"Logs"`
}

type ExecutionOutputBuildLogs struct {
	CloudWatchLogsArn string `json:"CloudWatchLogsArn"`
	DeepLink          string `json:"DeepLink"`
	GroupName         string `json:"GroupName"`
	StreamName        string `json:"StreamName"`
}

// Sfn provides the ability to handle Step Functions requests.
type Sfn struct {
	Client sfniface.SFNAPI
}

// Cloudwatch provides the ability to handle Cloudwatch requests
type Cloudwatch struct {
	Client cloudwatchlogsiface.CloudWatchLogsAPI
}

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
		Use:   "workspace",
		Short: "Runs terragrunt workspace",
		Args:  validateRunWorkspaceArgs,
		Run:   runRunWorkspace,
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

func runRunWorkspace(cmd *cobra.Command, args []string) {
	workspacePath, err := cmd.Flags().GetString("path")
	if err != nil {
		logs.Logger.Errorw("error while accessing flag",
			"flag", "path",
			"error", err)
		cmd.PrintErrf("path flag is required")
		return
	}
	workspaceBranch, err := cmd.Flags().GetString("branch")
	if err != nil {
		logs.Logger.Errorw("error while accessing flag",
			"flag", "branch",
			"error", err)
		cmd.PrintErrf("branch flag is required")
		return
	}
	workspaceAction, err := cmd.Flags().GetString("action")
	if err != nil {
		logs.Logger.Errorw("error while accessing flag",
			"flag", "action",
			"error", err)
		cmd.PrintErrf("action flag is required")
		return
	}

	// Session
	sess := session.Must(session.NewSession(&aws.Config{}))

	// Start step function
	logInformation, err := startStateMachine(cmd, sess, workspacePath, config.Configuration.GetString("state_machine_arn"), workspaceBranch, workspaceAction)
	if err != nil {
		cmd.PrintErrf("failed to start state machine")
		return
	}

	// Stream log content
	if err := streamCloudwatchLogs(cmd, sess, logInformation.Build.Logs.GroupName, logInformation.Build.Logs.StreamName); err != nil {
		cmd.PrintErrf("failed to stream logs")
		return
	}
}

var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func randSeq(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func startStateMachine(cmd *cobra.Command, sess *session.Session, target, stateMachineArn, branch, action string) (*ExecutionOutput, error) {
	logs.Logger.Debug("start")
	defer logs.Logger.Debug("end")

	// Client
	sfnClient := Sfn{
		Client: sfn.New(sess),
	}

	// Template input information
	inputParams := &SfnInputParameters{
		Resource: target,
		Branch:   branch,
		Action:   action,
	}
	tpl, err := template.New("executionInput").Parse(stateMachineInputTpl)
	if err != nil {
		logs.Logger.Errorw("failed to parse template",
			"name", "stateMachineInputTpl",
			"error", err)
		return nil, err
	}
	var templatedInput bytes.Buffer
	if err := tpl.Execute(&templatedInput, inputParams); err != nil {
		logs.Logger.Errorw("failed to prepare input json",
			"name", "stateMachineInputTpl",
			"error", err)
		return nil, err
	}

	// Start state machine
	startInput := &sfn.StartExecutionInput{
		Input:           aws.String(templatedInput.String()),
		Name:            aws.String(fmt.Sprintf("terra-ci-runner-%s-%s", action, randSeq(8))),
		StateMachineArn: aws.String(stateMachineArn),
	}
	executionOutput, err := sfnClient.Client.StartExecution(startInput)
	if err != nil {
		logs.Logger.Errorw("failed to execute state machine",
			"arn", stateMachineArn,
			"error", err)
		return nil, err
	}

	// Wait for task to complete
	describeInput := &sfn.DescribeExecutionInput{
		ExecutionArn: executionOutput.ExecutionArn,
	}
	executionStatusChan := make(chan *sfn.DescribeExecutionOutput, 1)

	d := time.Now().Add(config.Configuration.GetDuration("sfn_execution_timeout") * time.Minute)
	ctx, cancel := context.WithDeadline(context.Background(), d)
	defer cancel()

	go func(ctx context.Context, outputChan chan *sfn.DescribeExecutionOutput) {
		cmd.Printf("waiting for state machine to complete...\n")

		s := spinner.New(spinner.CharSets[9], 100*time.Millisecond)
		s.Start()
		defer s.Stop()

		for {
			select {
			case <-ctx.Done():
				logs.Logger.Errorw("CLI execution timeout reached")
				outputChan <- nil
				return
			default:
			}
			executionStatus, err := sfnClient.Client.DescribeExecution(describeInput)
			if err != nil {
				logs.Logger.Errorw("failed to describe execution status",
					"error", err)
				outputChan <- nil
				return
			}
			s.Suffix = fmt.Sprintf("  current state: %s", *executionStatus.Status)
			switch *executionStatus.Status {
			case "SUCCEEDED":
				outputChan <- executionStatus
				return
			case "RUNNING":
				time.Sleep(time.Second * config.Configuration.GetDuration("refresh_rate"))
			default:
				logs.Logger.Errorw("execution failed",
					"status", *executionStatus.Status)
				outputChan <- nil
				return
			}
		}
	}(ctx, executionStatusChan)

	executionStatus := <-executionStatusChan
	if executionStatus == nil {
		return nil, InternalError("state machine execution failed")
	}

	var logInformation ExecutionOutput
	if err := json.Unmarshal([]byte(*executionStatus.Output), &logInformation); err != nil {
		logs.Logger.Errorw("failed to unmarshal logs information",
			"error", err)
		return nil, err
	}
	return &logInformation, nil
}

func streamCloudwatchLogs(cmd *cobra.Command, sess *session.Session, groupName string, streamName string) error {
	logs.Logger.Debugw("start")
	defer logs.Logger.Debugw("end")

	cloudwatchClient := Cloudwatch{
		Client: cloudwatchlogs.New(sess),
	}

	resp, err := cloudwatchClient.Client.GetLogEvents(&cloudwatchlogs.GetLogEventsInput{
		LogGroupName:  aws.String(groupName),
		LogStreamName: aws.String(streamName),
		StartFromHead: aws.Bool(true),
	})
	if err != nil {
		logs.Logger.Errorw("failed to fetch log events",
			"error", err)
		return err
	}
	for _, event := range resp.Events {
		cmd.Printf("  %s", *event.Message)
	}

	gotToken := *resp.NextForwardToken

	for {
		resp, err = cloudwatchClient.Client.GetLogEvents(&cloudwatchlogs.GetLogEventsInput{
			LogGroupName:  aws.String(groupName),
			LogStreamName: aws.String(streamName),
			StartFromHead: aws.Bool(true),
			NextToken:     resp.NextForwardToken,
		})
		if err != nil {
			logs.Logger.Errorw("failed to fetch log events",
				"error", err)
			return err
		}
		if gotToken == *resp.NextForwardToken {
			break
		}
		for _, event := range resp.Events {
			cmd.Printf("  %s", *event.Message)
		}
		gotToken = *resp.NextForwardToken
	}
	return nil
}
