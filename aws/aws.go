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
	Resource       string
	Action         string
	RepositoryUrl  string
	RepositoryName string
	Run            string
}

type ExecutionOutput struct {
	TaskResults TaskResultOutput `json:"taskresult"`
}

type TaskResultOutput struct {
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

func StartStateMachine(target, stateMachineArn, repoUrl, repoName, run, action string) (string, error) {
	sess := session.Must(session.NewSession(&aws.Config{}))

	sfnClient := Sfn{
		Client: sfn.New(sess),
	}

	inputParams := &SfnInputParameters{
		Resource:       target,
		Action:         action,
		RepositoryUrl:  repoUrl,
		RepositoryName: repoName,
		Run:            run,
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
			if err := json.Unmarshal([]byte(*event.TaskFailedEventDetails.Cause), &logInformation); err != nil {
				fmt.Fprintf(out, "faild to get details: %s\n", err.Error())
			}
			if err := StreamCloudwatchLogs(out, logInformation.TaskResults.Build.Logs.GroupName, logInformation.TaskResults.Build.Logs.StreamName, false); err != nil {
				fmt.Fprintf(out, "failed to stream logs for %s:%s\n", logInformation.TaskResults.Build.Logs.GroupName, logInformation.TaskResults.Build.Logs.StreamName)
				fmt.Fprintf(out, "error: %s\n", err.Error())
			}
			fmt.Fprintf(out, "task failed\n")
		case "TaskSucceeded":
		case "TaskTimedOut":
		case "TaskStateAborted":
		case "TaskStateExited":
			if err := json.Unmarshal([]byte(*event.StateExitedEventDetails.Output), &logInformation); err != nil {
				fmt.Fprintf(out, "faild to get details: %s\n", err.Error())
			}
			if err := StreamCloudwatchLogs(out, logInformation.TaskResults.Build.Logs.GroupName, logInformation.TaskResults.Build.Logs.StreamName, false); err != nil {
				fmt.Fprintf(out, "failed to stream logs for %s:%s\n", logInformation.TaskResults.Build.Logs.GroupName, logInformation.TaskResults.Build.Logs.StreamName)
				fmt.Fprintf(out, "error: %s\n", err.Error())
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

type ExecutionMonitorExitDetails struct {
	Type   string
	Error  error
	Output string
}

func MonitorStateMachineStatus(arn string, refreshRate, executionTimeout time.Duration, isCi bool, out, outErr io.Writer) error {
	sess := session.Must(session.NewSession(&aws.Config{}))
	sfnClient := Sfn{
		Client: sfn.New(sess),
	}

	d := time.Now().Add(executionTimeout * time.Minute)
	ctx, cancel := context.WithDeadline(context.Background(), d)
	defer cancel()

	// Process state machine events
	events := &ExecutionEventHistory{
		Events: make(map[int64]*sfn.HistoryEvent),
	}
	monitorExitStatus := make(chan *ExecutionMonitorExitDetails)
	go func(ctx context.Context, exitStatus chan *ExecutionMonitorExitDetails) {
		completed := false
		fmt.Fprintf(out, "monitoring execution of %s\n", arn)
		executionEventInput := &sfn.GetExecutionHistoryInput{
			ExecutionArn: aws.String(arn),
		}
		for {
			executionEvents, err := sfnClient.Client.GetExecutionHistory(executionEventInput)
			if err != nil {
				exitStatus <- &ExecutionMonitorExitDetails{
					Error: err,
				}
				return
			}
			events, completed = processEvents(events, executionEvents, out)
			executionEventInput.NextToken = executionEvents.NextToken
			if completed {
				break
			}
			select {
			case <-ctx.Done():
				exitStatus <- &ExecutionMonitorExitDetails{
					Error: fmt.Errorf("cli execution timed out"),
				}
				return
			default:
				time.Sleep(time.Second * refreshRate)
			}
		}
		fmt.Fprintf(out, "execution of state machine completed\n")
		exitStatus <- &ExecutionMonitorExitDetails{}
	}(ctx, monitorExitStatus)

	exitStatus := <-monitorExitStatus
	if exitStatus.Error != nil {
		return exitStatus.Error
	}
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

/*************************** FF SFN_MONITOR ***************************************/

func FFMonitorStateMachineStatus(arn string, refreshRate, executionTimeout time.Duration, isCi bool, out, outErr io.Writer) error {
	sess := session.Must(session.NewSession(&aws.Config{}))
	sfnClient := Sfn{
		Client: sfn.New(sess),
	}

	d := time.Now().Add(executionTimeout * time.Minute)
	ctx, cancel := context.WithDeadline(context.Background(), d)
	defer cancel()

	// Process state machine events
	events := &ExecutionEventHistory{
		Events: make(map[int64]*sfn.HistoryEvent),
	}
	monitorExitStatus := make(chan *ExecutionMonitorExitDetails)

	monitorInput := &ExecutionMonitorInput{
		Client:      sfnClient,
		Arn:         arn,
		Events:      events,
		RefreshRate: refreshRate,
		Out:         out,
	}
	go EventHistoryMonitor(ctx, monitorInput, monitorExitStatus)

	exitStatus := <-monitorExitStatus
	if exitStatus.Error != nil {
		return exitStatus.Error
	}
	return returnExecutionStatus(events)
}

type ExecutionMonitorInput struct {
	Arn         string
	Events      *ExecutionEventHistory
	Client      Sfn
	Out         io.Writer
	RefreshRate time.Duration
}

func NewTaskEventSubscriber(eb *SfnEventBus, out io.Writer, done *sync.WaitGroup) {
	defer done.Done()
	var wg sync.WaitGroup
	taskEventChan := make(chan SfnEvent)
	eb.Subscribe("TaskStateEntered", taskEventChan)
	eb.Subscribe("TaskSubmitted", taskEventChan)
	eb.Subscribe("TaskSubmitFailed", taskEventChan)
	eb.Subscribe("TaskScheduled", taskEventChan)
	eb.Subscribe("TaskStarted", taskEventChan)
	eb.Subscribe("TaskStartFailed", taskEventChan)
	eb.Subscribe("TaskFailed", taskEventChan)
	eb.Subscribe("TaskSucceeded", taskEventChan)
	eb.Subscribe("TaskTimedOut", taskEventChan)
	eb.Subscribe("TaskStateAborted", taskEventChan)
	eb.Subscribe("TaskStateExited", taskEventChan)

	wg.Add(1)
	go func(ch chan SfnEvent) {
		defer wg.Done()
		var logInformation ExecutionOutput
		for {
			select {
			case d := <-ch:
				fmt.Fprintf(out, "[SUBSCRIBER]Event: %s\n", *d.Data.Type)
				switch *d.Data.Type {
				case "TaskStateEntered":
					fmt.Fprintf(out, "waiting for %s task to complete...\n", *d.Data.StateEnteredEventDetails.Name)
				case "TaskSubmitted":
				case "TaskSubmitFailed":
				case "TaskScheduled":
				case "TaskStarted":
				case "TaskStartFailed":
				case "TaskFailed":
					if err := json.Unmarshal([]byte(*d.Data.TaskFailedEventDetails.Cause), &logInformation); err != nil {
						fmt.Fprintf(out, "faild to get details: %s\n", err.Error())
					}
					if err := StreamCloudwatchLogs(out, logInformation.TaskResults.Build.Logs.GroupName, logInformation.TaskResults.Build.Logs.StreamName, false); err != nil {
						fmt.Fprintf(out, "failed to stream logs for %s:%s\n", logInformation.TaskResults.Build.Logs.GroupName, logInformation.TaskResults.Build.Logs.StreamName)
						fmt.Fprintf(out, "error: %s\n", err.Error())
					}
					fmt.Fprintf(out, "task failed\n")
					return
				case "TaskSucceeded":
				case "TaskTimedOut":
				case "TaskStateAborted":
				case "TaskStateExited":
					if err := json.Unmarshal([]byte(*d.Data.StateExitedEventDetails.Output), &logInformation); err != nil {
						fmt.Fprintf(out, "faild to get details: %s\n", err.Error())
					}
					if err := StreamCloudwatchLogs(out, logInformation.TaskResults.Build.Logs.GroupName, logInformation.TaskResults.Build.Logs.StreamName, false); err != nil {
						fmt.Fprintf(out, "failed to stream logs for %s:%s\n", logInformation.TaskResults.Build.Logs.GroupName, logInformation.TaskResults.Build.Logs.StreamName)
						fmt.Fprintf(out, "error: %s\n", err.Error())
					}
					fmt.Fprintf(out, "task %s completed\n", *d.Data.StateExitedEventDetails.Name)
					return
				}
			}
		}
	}(taskEventChan)
	wg.Wait()
}

func EventHistoryMonitor(ctx context.Context, monitorInput *ExecutionMonitorInput, exitStatus chan *ExecutionMonitorExitDetails) {
	completed := false
	executionEventInput := &sfn.GetExecutionHistoryInput{
		ExecutionArn: aws.String(monitorInput.Arn),
	}
	eb := &SfnEventBus{
		subscribers: map[string]SfnEventChannelSlice{},
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go NewTaskEventSubscriber(eb, monitorInput.Out, &wg)

	for {
		executionEvents, err := monitorInput.Client.Client.GetExecutionHistory(executionEventInput)
		if err != nil {
			exitStatus <- &ExecutionMonitorExitDetails{
				Error: err,
			}
			return
		}
		monitorInput.Events, completed = FFprocessEvents(eb, monitorInput.Events, executionEvents, monitorInput.Out)
		executionEventInput.NextToken = executionEvents.NextToken
		if completed {
			break
		}
		select {
		case <-ctx.Done():
			exitStatus <- &ExecutionMonitorExitDetails{
				Error: fmt.Errorf("cli execution timed out"),
			}
			return
		default:
			time.Sleep(time.Second * monitorInput.RefreshRate)
		}
	}
	wg.Wait()
	exitStatus <- &ExecutionMonitorExitDetails{}
}

func FFprocessEvents(eb *SfnEventBus, events *ExecutionEventHistory, executionHistory *sfn.GetExecutionHistoryOutput, out io.Writer) (*ExecutionEventHistory, bool) {
	completed := false
	for _, event := range executionHistory.Events {
		if _, ok := events.Events[*event.Id]; ok {
			continue
		}
		eb.Publish(*event.Type, SfnEvent{Data: event})
		switch *event.Type {
		case "ExecutionStarted":
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

type SfnEvent struct {
	Data *sfn.HistoryEvent
}

type SfnEventChannel chan SfnEvent
type SfnEventChannelSlice []SfnEventChannel

type SfnEventBus struct {
	subscribers map[string]SfnEventChannelSlice
	rm          sync.RWMutex
}

func (eb *SfnEventBus) Subscribe(eventType string, ch SfnEventChannel) {
	eb.rm.Lock()
	defer eb.rm.Unlock()
	if prev, found := eb.subscribers[eventType]; found {
		eb.subscribers[eventType] = append(prev, ch)
	} else {
		eb.subscribers[eventType] = append([]SfnEventChannel{}, ch)
	}
}

func (eb *SfnEventBus) Publish(eventType string, data SfnEvent) {
	eb.rm.RLock()
	defer eb.rm.RUnlock()
	if chans, found := eb.subscribers[eventType]; found {
		// this is done because the slices refer to same array even though they are passed by value
		// thus we are creating a new slice with our elements thus preserve locking correctly.
		channels := append(SfnEventChannelSlice{}, chans...)
		go func(data SfnEvent, dataChannelSlices SfnEventChannelSlice) {
			for _, ch := range dataChannelSlices {
				ch <- data
			}
		}(data, channels)
	}
}

/*************************** FF SFN_MONITOR - END  ***************************************/
