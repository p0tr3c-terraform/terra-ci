package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"text/template"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs/cloudwatchlogsiface"
	"github.com/aws/aws-sdk-go/service/sfn"
	"github.com/aws/aws-sdk-go/service/sfn/sfniface"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	rootCmd = &cobra.Command{
		Use:           "terra-ci",
		RunE:          rootCmdHandler,
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cfg           = viper.New()
	configOptions = []ConfigOptions{
		{"state-machine-arn", "", "aws state machine arn to run", "string"},
		{"refresh-rate", 10, "refresh rate to fetch cloudwatch events in seconds", "duration"},
		{"execution-input", defaultTemplate, "JSON context for state machine input", "string"},
		{"execution-timeout", 30, "CLI execution timeout in minutes", "duration"},
		{"branch", "main", "Branch reference to execute on", "string"},
		{"action", "plan", "Terraform action to perform", "string"},
	}
)

const (
	defaultTemplate = `
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

func init() {
	rand.Seed(time.Now().UnixNano())
}

type ConfigOptions struct {
	Name        string
	Default     interface{}
	Description string
	Type        string
}

func Execute() error {
	cfg.SetEnvPrefix("TERRA_CI")
	cfg.AutomaticEnv()

	for _, option := range configOptions {
		switch option.Type {
		case "string":
			rootCmd.PersistentFlags().String(option.Name, option.Default.(string), option.Description)
		case "int":
			rootCmd.PersistentFlags().Int(option.Name, option.Default.(int), option.Description)
		case "duration":
			rootCmd.PersistentFlags().Duration(option.Name, time.Duration(option.Default.(int)), option.Description)
		}
		if err := cfg.BindPFlag(strings.ReplaceAll(option.Name, "-", "_"), rootCmd.PersistentFlags().Lookup(option.Name)); err != nil {
			return err
		}
	}

	// Read config from file
	cfg.SetConfigName(".config")
	cfg.SetConfigType("yaml")
	cfg.AddConfigPath(".")
	if err := cfg.ReadInConfig(); err != nil {
		// Ignore errors if config file does not exist
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			log.Printf("failed to load configuration file")
			return err
		}
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

func startStateMachine(sess *session.Session, target string, stateMachineArn string, branch string) (*ExecutionOutput, error) {
	// Client
	sfnClient := Sfn{
		Client: sfn.New(sess),
	}

	// Template input information
	inputParams := &InputParameters{
		Resource: target,
		Branch:   branch,
		Action:   cfg.GetString("action"),
	}
	tpl, err := template.New("executionInput").Parse(cfg.GetString("execution_input"))
	if err != nil {
		log.Printf("failed to parse template")
		return nil, err
	}
	var templatedInput bytes.Buffer
	if err := tpl.Execute(&templatedInput, inputParams); err != nil {
		log.Printf("failed to prepare input json")
		return nil, err
	}

	// Start state machine
	log.Printf("running terragrunt for %s", stateMachineArn)
	log.Printf("executing %s", stateMachineArn)
	startInput := &sfn.StartExecutionInput{
		Input:           aws.String(templatedInput.String()),
		Name:            aws.String(fmt.Sprintf("terra-ci-runner-plan-%s", randSeq(8))),
		StateMachineArn: aws.String(stateMachineArn),
	}
	executionOutput, err := sfnClient.Client.StartExecution(startInput)
	if err != nil {
		log.Printf("failed to execute %s", stateMachineArn)
		return nil, err
	}

	// Wait for task to complete
	describeInput := &sfn.DescribeExecutionInput{
		ExecutionArn: executionOutput.ExecutionArn,
	}
	log.Printf("cloudwatch refresh rate %ds", cfg.GetDuration("refresh_rate"))
	executionStatusChan := make(chan *sfn.DescribeExecutionOutput, 1)

	log.Printf("CLI timeout %dm", cfg.GetDuration("execution_timeout"))
	d := time.Now().Add(cfg.GetDuration("execution_timeout") * time.Minute)
	ctx, cancel := context.WithDeadline(context.Background(), d)
	defer cancel()

	go func(ctx context.Context, outputChan chan *sfn.DescribeExecutionOutput) {
		for {
			select {
			case <-ctx.Done():
				log.Printf("CLI execution timeout reached")
				outputChan <- nil
				return
			default:
			}
			executionStatus, err := sfnClient.Client.DescribeExecution(describeInput)
			if err != nil {
				log.Printf("failed to describe execution status")
				outputChan <- nil
				return
			}
			log.Printf("execution status %s", *executionStatus.Status)
			switch *executionStatus.Status {
			case "SUCCEEDED":
				outputChan <- executionStatus
				return
			case "RUNNING":
				time.Sleep(time.Second * cfg.GetDuration("refresh_rate"))
			default:
				log.Printf("execution failed with %s", *executionStatus.Status)
				outputChan <- nil
				return
			}
		}
	}(ctx, executionStatusChan)

	executionStatus := <-executionStatusChan
	if executionStatus == nil {
		return nil, fmt.Errorf("state machine execution failed")
	}

	var logInformation ExecutionOutput
	if err := json.Unmarshal([]byte(*executionStatus.Output), &logInformation); err != nil {
		log.Printf("failed to unmarshal logs information")
		return nil, err
	}
	return &logInformation, nil
}

func streamCloudwatchLogs(sess *session.Session, groupName string, streamName string) error {
	log.Printf("log group name %s", groupName)
	log.Printf("log stream name %s", streamName)

	cloudwatchClient := Cloudwatch{
		Client: cloudwatchlogs.New(sess),
	}

	resp, err := cloudwatchClient.Client.GetLogEvents(&cloudwatchlogs.GetLogEventsInput{
		LogGroupName:  aws.String(groupName),
		LogStreamName: aws.String(streamName),
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
		resp, err = cloudwatchClient.Client.GetLogEvents(&cloudwatchlogs.GetLogEventsInput{
			LogGroupName:  aws.String(groupName),
			LogStreamName: aws.String(streamName),
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

func rootCmdHandler(cmd *cobra.Command, args []string) error {
	// Session
	sess := session.Must(session.NewSession(&aws.Config{}))

	// Start step function
	logInformation, err := startStateMachine(sess, args[0], cfg.GetString("state_machine_arn"), cfg.GetString("branch"))
	if err != nil {
		return err
	}

	// Stream log content
	if err := streamCloudwatchLogs(sess, logInformation.Build.Logs.GroupName, logInformation.Build.Logs.StreamName); err != nil {
		return err
	}

	return nil
}
