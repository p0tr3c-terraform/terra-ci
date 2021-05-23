package fflags

import (
	"fmt"
	"os"
)

const (
	flagPrefix = "TERRA_CI_FLAG"
)

var (
	flags = map[string]bool{
		"PLAN_SPLIT": false,
	}
)

func IsPlanSplitEnabled() bool {
	return IsEnabled("PLAN_SPLIT")
}

func IsEnabled(flag string) bool {
	ffe := os.Getenv(fmt.Sprintf("%s_%s", flagPrefix, flag))
	// If hardcoded
	if ff, ok := flags[flag]; ok {
		return ff
	} else {
		return ffe != ""
	}
}
