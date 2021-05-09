package fflags

import (
	"fmt"
	"os"
)

const (
	flagPrefix = "TERRA_CI_FLAG_"
)

var (
	flags = map[string]bool{
		"PLAN_SPLIT": false,
	}
)

func IsPlanSplitEnabled() bool {
	return IsEnabled(fmt.Sprintf("%sPLAN_SPLIT", flagPrefix))
}

func IsEnabled(flag string) bool {
	ff := os.Getenv(flag)
	return ff != ""
}
