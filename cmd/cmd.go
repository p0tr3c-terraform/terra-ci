package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"text/template"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go/service/sfn"
	"github.com/spf13/cobra"
)

var (
	StateMachineArn = "arn:aws:states:eu-west-1:719261439472:stateMachine:terra-ci-runner"
	refreshRate     = 10
)

const (
	InputTemplate = `
{
    "Comment": "Run from CLI",
    "build": {
      "environment": {
        "terra_ci_resource": "{{ .Resource }}"
      }
    }
}
`
)

func Execute() error {
	rand.Seed(time.Now().UnixNano())
	rootCmd := &cobra.Command{
		Use:           "terra-ci",
		RunE:          rootCmdHandler,
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	if err := rootCmd.Execute(); err != nil {
		return err
	}
	return nil
}

var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func randSeq(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

type InputParameters struct {
	Resource string
}

type OutputExecutionDetails struct {
	Build ExecutionBuildInformation `json:"Build"`
}

type ExecutionBuildInformation struct {
	Logs ExecutionBuildLogs `json:"Logs"`
}

type ExecutionBuildLogs struct {
	CloudWatchLogsArn string `json:"CloudWatchLogsArn"`
	DeepLink          string `json:"DeepLink"`
	GroupName         string `json:"GroupName"`
	StreamName        string `json:"StreamName"`
}

func rootCmdHandler(cmd *cobra.Command, args []string) error {
	// Session
	sess := session.Must(session.NewSession(&aws.Config{}))

	// Service
	svc := sfn.New(sess)

	// Template input information
	inputParams := &InputParameters{
		Resource: args[0],
	}
	tpl, err := template.New("executionInput").Parse(InputTemplate)
	if err != nil {
		log.Printf("failed to parse template")
		return err
	}
	var templatedInput bytes.Buffer
	if err := tpl.Execute(&templatedInput, inputParams); err != nil {
		log.Printf("failed to prepare input json")
		return err
	}

	// Start state machine
	log.Printf("starting terra-ci-runner with %s", args[0])
	startInput := &sfn.StartExecutionInput{
		Input:           aws.String(templatedInput.String()),
		Name:            aws.String(fmt.Sprintf("terra-ci-runner-plan-%s", randSeq(8))),
		StateMachineArn: aws.String(StateMachineArn),
	}
	executionOutput, err := svc.StartExecution(startInput)
	if err != nil {
		log.Printf("failed to execute %s", StateMachineArn)
		return err
	}

	log.Printf("execution %s", executionOutput)

	// Wait for task to complete
	describeInput := &sfn.DescribeExecutionInput{
		ExecutionArn: executionOutput.ExecutionArn,
	}
	executionStatusChan := make(chan *sfn.DescribeExecutionOutput, 1)
	go func(outputChan chan *sfn.DescribeExecutionOutput) {
		for {
			executionStatus, err := svc.DescribeExecution(describeInput)
			if err != nil {
				log.Printf("failed to describe execution status")
				outputChan <- nil
				return
			}
			log.Printf("execution status %s", *executionStatus.Status)
			switch *executionStatus.Status {
			case "SUCCEEDED":
				log.Printf("execution completed")
				outputChan <- executionStatus
				return
			case "RUNNING":
				time.Sleep(time.Second * time.Duration(refreshRate))
			default:
				log.Printf("execution failed with %s", *executionStatus.Status)
				outputChan <- nil
				return
			}
		}
	}(executionStatusChan)

	executionStatus := <-executionStatusChan
	if executionStatus == nil {
		log.Printf("execution failed")
		return fmt.Errorf("failed to execut")
	}

	log.Printf("execution output")
	log.Printf("%s", executionStatus)

	var logInformation OutputExecutionDetails
	if err := json.Unmarshal([]byte(*executionStatus.Output), &logInformation); err != nil {
		log.Printf("failed to unmarshal logs information")
		return err
	}

	log.Printf("log group name %s", logInformation.Build.Logs.GroupName)
	log.Printf("log stream name %s", logInformation.Build.Logs.StreamName)

	cloudwatchSvc := cloudwatchlogs.New(sess)

	resp, err := cloudwatchSvc.GetLogEvents(&cloudwatchlogs.GetLogEventsInput{
		LogGroupName:  aws.String(logInformation.Build.Logs.GroupName),
		LogStreamName: aws.String(logInformation.Build.Logs.StreamName),
		StartFromHead: aws.Bool(true),
	})
	if err != nil {
		log.Printf("failed to fetch log events")
		return err
	}
	for _, event := range resp.Events {
		log.Printf("  %s", *event.Message)
	}

	gotToken := *resp.NextForwardToken

	for {
		resp, err = cloudwatchSvc.GetLogEvents(&cloudwatchlogs.GetLogEventsInput{
			LogGroupName:  aws.String(logInformation.Build.Logs.GroupName),
			LogStreamName: aws.String(logInformation.Build.Logs.StreamName),
			StartFromHead: aws.Bool(true),
			NextToken:     resp.NextForwardToken,
		})
		if err != nil {
			log.Printf("failed to fetch log events")
			return err
		}
		if gotToken == *resp.NextForwardToken {
			log.Printf("end of stream")
			break
		}
		for _, event := range resp.Events {
			log.Printf("  %s", *event.Message)
		}
		gotToken = *resp.NextForwardToken
	}
	return nil
}
