package cmd

import (
	"github.com/spf13/cobra"
	"io"
)

func NewCreateCommand(in io.Reader, out, outerr io.Writer) *cobra.Command {
	command := &cobra.Command{
		Use:   "create",
		Short: "Creates terra-ci resource type",
		Run:   runHelp,
	}
	return command
}
