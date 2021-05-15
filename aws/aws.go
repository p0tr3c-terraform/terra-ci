package aws

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"regexp"
	"sync"
	"text/template"
	"time"

	"github.com/p0tr3c/terra-ci/templates"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs/cloudwatchlogsiface"
	"github.com/aws/aws-sdk-go/service/sfn"
	"github.com/aws/aws-sdk-go/service/sfn/sfniface"
	_ "github.com/briandowns/spinner"
)

var (
	SfnExitEvents = map[string]bool{
		"ExecutionSucceeded": true,
		"ExecutionTimedOut":  true,
		"ExecutionFailed":    true,
		"ExecutionAborted":   true,
	}
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
	Arn  string                   `json:"Arn"`
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

func processEvents(events *ExecutionEventHistory, executionHistory *sfn.GetExecutionHistoryOutput, out io.Writer) (*ExecutionEventHistory, bool) {
	completed := false
	for _, event := range executionHistory.Events {
		if _, ok := events.Events[*event.Id]; ok {
			continue
		}
		var logInformation ExecutionOutput
		switch *event.Type {
		case "ExecutionStarted":
		case "TaskStateEntered":
			fmt.Fprintf(out, "waiting for %s task to complete...\n", *event.StateEnteredEventDetails.Name)
		case "TaskSubmitted":
		case "TaskSubmitFailed":
		case "TaskScheduled":
		case "TaskStarted":
		case "TaskStartFailed":
		case "TaskFailed":
		case "TaskSucceeded":
		case "TaskTimedOut":
		case "TaskStateAborted":
		case "TaskStateExited":
			if err := json.Unmarshal([]byte(*event.StateExitedEventDetails.Output), &logInformation); err != nil {
				fmt.Fprintf(out, "faild to get details: %s\n", err.Error())
			}
			if err := StreamCloudwatchLogs(out, logInformation.Build.Logs.GroupName, logInformation.Build.Logs.StreamName, false); err != nil {
				fmt.Fprintf(out, "failed to stream logs for %s:%s\n", logInformation.Build.Logs.GroupName, logInformation.Build.Logs.StreamName)
			}
			fmt.Fprintf(out, "task %s completed\n", *event.StateExitedEventDetails.Name)
		case "ParallelStateStarted":
		case "ChoiceStateEntered":
		case "FailStateEntered":
		case "MapStateEntered":
		case "PassStateEntered":
		case "SucceedStateEntered":
		case "WaitStateEntered":
		case "ExecutionSucceeded":
			completed = true
		case "ExecutionTimedOut":
			completed = true
		case "ExecutionFailed":
			completed = true
		case "ExecutionAborted":
			completed = true
		}
		events.AddEvent(event)
	}
	return events, completed
}

type ExecutionEventHistory struct {
	Events      map[int64]*sfn.HistoryEvent
	LastEventId int64
}

func (e *ExecutionEventHistory) AddEvent(event *sfn.HistoryEvent) {
	e.Events[*event.Id] = event
	if *event.Id > e.LastEventId {
		e.LastEventId = *event.Id
	}
}

func returnExecutionStatus(events *ExecutionEventHistory) error {
	if len(events.Events) > 0 {
		completionStatus := *events.Events[events.LastEventId].Type
		if completionStatus != "ExecutionSucceeded" {
			return fmt.Errorf("execution completed with %s status", completionStatus)
		}
	}
	return nil
}

func MonitorStateMachineStatus(arn string, refreshRate, executionTimeout time.Duration, isCi bool, out, outErr io.Writer) error {
	sess := session.Must(session.NewSession(&aws.Config{}))
	var wg sync.WaitGroup

	sfnClient := Sfn{
		Client: sfn.New(sess),
	}

	d := time.Now().Add(executionTimeout * time.Minute)
	ctx, cancel := context.WithDeadline(context.Background(), d)
	defer cancel()

	wg.Add(1)
	// Process state machine events
	events := &ExecutionEventHistory{
		Events: make(map[int64]*sfn.HistoryEvent),
	}
	go func(ctx context.Context) {
		defer wg.Done()
		completed := false
		fmt.Fprintf(out, "monitoring execution of %s\n", arn)
		executionEventInput := &sfn.GetExecutionHistoryInput{
			ExecutionArn: aws.String(arn),
		}
		for {
			executionEvents, err := sfnClient.Client.GetExecutionHistory(executionEventInput)
			if err != nil {
				return
			}
			events, completed = processEvents(events, executionEvents, out)
			executionEventInput.NextToken = executionEvents.NextToken
			if completed {
				break
			}
			select {
			case <-ctx.Done():
				fmt.Fprintf(out, "cli execution timed out\n")
				return
			default:
				time.Sleep(time.Second * refreshRate)
			}
		}
		fmt.Fprintf(out, "execution of state machine completed\n")
	}(ctx)

	wg.Wait()
	return returnExecutionStatus(events)
}

func GetCloudwatchLogsReference(executionStatus *sfn.DescribeExecutionOutput) (*ExecutionOutput, error) {
	var logInformation ExecutionOutput
	if err := json.Unmarshal([]byte(*executionStatus.Output), &logInformation); err != nil {
		return nil, err
	}
	return &logInformation, nil
}

func StreamCloudwatchLogs(out io.Writer, groupName, streamName string, verbose bool) error {
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

	gotToken := *resp.NextForwardToken
	excludeInternalLogsPattern, err := regexp.Compile(`^\[Container\]`)
	if err != nil {
		return err
	}
	for {
		for _, event := range resp.Events {
			if !verbose {
				matched := excludeInternalLogsPattern.MatchString(*event.Message)
				if matched {
					continue
				}
			}
			fmt.Fprintf(out, "%s", *event.Message)
		}

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

		gotToken = *resp.NextForwardToken
	}
	return nil
}
