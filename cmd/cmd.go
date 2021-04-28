package cmd

import (
	"errors"
	"log"

	_ "github.com/aws/aws-sdk-go/aws"
	_ "github.com/aws/aws-sdk-go/aws/session"
	"github.com/spf13/cobra"
)

var (
	ErrInvalidStatus = errors.New("non-200 status code")
)

const (
	defaultAWSRegion = "eu-west-1"
)

func Execute() error {
	rootCmd := &cobra.Command{
		Use:           "terra-ci",
		RunE:          rootCmdHandler,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	if err := rootCmd.Execute(); err != nil {
		return err
	}
	return nil
}

func rootCmdHandler(cmd *cobra.Command, args []string) error {
	log.Printf("Root command\n")
	return nil
}
