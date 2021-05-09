package aws

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"text/template"
	"time"

	"github.com/p0tr3c/terra-ci/templates"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs/cloudwatchlogsiface"
	"github.com/aws/aws-sdk-go/service/sfn"
	"github.com/aws/aws-sdk-go/service/sfn/sfniface"
	"github.com/briandowns/spinner"
)

type InternalError string

func (e InternalError) Error() string {
	return string(e)
}

type Sfn struct {
	Client sfniface.SFNAPI
}

type Cloudwatch struct {
	Client cloudwatchlogsiface.CloudWatchLogsAPI
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

var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func randSeq(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func StartStateMachine(target, stateMachineArn, branch, action string) (string, error) {
	sess := session.Must(session.NewSession(&aws.Config{}))

	sfnClient := Sfn{
		Client: sfn.New(sess),
	}

	inputParams := &SfnInputParameters{
		Resource: target,
		Branch:   branch,
		Action:   action,
	}
	tpl, err := template.New("executionInput").Parse(templates.StateMachineInputTpl)
	if err != nil {
		return "", err
	}
	var templatedInput bytes.Buffer
	if err := tpl.Execute(&templatedInput, inputParams); err != nil {
		return "", err
	}

	// Start state machine
	startInput := &sfn.StartExecutionInput{
		Input:           aws.String(templatedInput.String()),
		Name:            aws.String(fmt.Sprintf("terra-ci-runner-%s-%s", action, randSeq(8))),
		StateMachineArn: aws.String(stateMachineArn),
	}
	executionOutput, err := sfnClient.Client.StartExecution(startInput)
	if err != nil {
		return "", err
	}
	return *executionOutput.ExecutionArn, nil
}

func MonitorStateMachineStatus(arn string, refreshRate, executionTimeout time.Duration, ci bool, out, outErr io.Writer) (*sfn.DescribeExecutionOutput, error) {
	sess := session.Must(session.NewSession(&aws.Config{}))

	sfnClient := Sfn{
		Client: sfn.New(sess),
	}

	// Wait for task to complete
	describeInput := &sfn.DescribeExecutionInput{
		ExecutionArn: aws.String(arn),
	}
	executionStatusChan := make(chan *sfn.DescribeExecutionOutput, 1)

	d := time.Now().Add(executionTimeout * time.Minute)
	ctx, cancel := context.WithDeadline(context.Background(), d)
	defer cancel()

	go func(ctx context.Context, outputChan chan *sfn.DescribeExecutionOutput) {
		fmt.Fprintf(out, "waiting for state machine to complete...\n")
		s := spinner.New(spinner.CharSets[9], 100*time.Millisecond)
		if !ci {
			s.Start()
			defer s.Stop()
		}

		for {
			select {
			case <-ctx.Done():
				fmt.Fprintf(outErr, "CLI execution timeout reached\n")
				outputChan <- nil
				return
			default:
			}
			executionStatus, err := sfnClient.Client.DescribeExecution(describeInput)
			if err != nil {
				fmt.Fprintf(outErr, "failed to describe execution status\n")
				fmt.Fprintf(outErr, "%s\n", err)
				outputChan <- nil
				return
			}
			if !ci {
				s.Suffix = fmt.Sprintf("  current state: %s", *executionStatus.Status)
			} else {
				fmt.Fprintf(out, "current state: %s\n", *executionStatus.Status)
			}
			switch *executionStatus.Status {
			case "SUCCEEDED":
				outputChan <- executionStatus
				return
			case "RUNNING":
				time.Sleep(time.Second * refreshRate)
			default:
				fmt.Fprintf(outErr, "execution failed\n")
				fmt.Fprintf(outErr, "%s\n", *executionStatus.Status)
				outputChan <- nil
				return
			}
		}
	}(ctx, executionStatusChan)

	executionStatus := <-executionStatusChan
	if executionStatus == nil {
		return nil, InternalError("state machine execution failed")
	}
	return executionStatus, nil
}

func GetCloudwatchLogsReference(executionStatus *sfn.DescribeExecutionOutput) (*ExecutionOutput, error) {
	var logInformation ExecutionOutput
	if err := json.Unmarshal([]byte(*executionStatus.Output), &logInformation); err != nil {
		return nil, err
	}
	return &logInformation, nil
}

func StreamCloudwatchLogs(out io.Writer, groupName, streamName string) error {
	sess := session.Must(session.NewSession(&aws.Config{}))

	cloudwatchClient := Cloudwatch{
		Client: cloudwatchlogs.New(sess),
	}

	resp, err := cloudwatchClient.Client.GetLogEvents(&cloudwatchlogs.GetLogEventsInput{
		LogGroupName:  aws.String(groupName),
		LogStreamName: aws.String(streamName),
		StartFromHead: aws.Bool(true),
	})
	if err != nil {
		return err
	}
	for _, event := range resp.Events {
		fmt.Fprintf(out, "  %s", *event.Message)
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
			return err
		}
		if gotToken == *resp.NextForwardToken {
			break
		}
		for _, event := range resp.Events {
			fmt.Fprintf(out, "  %s", *event.Message)
		}
		gotToken = *resp.NextForwardToken
	}
	return nil
}
