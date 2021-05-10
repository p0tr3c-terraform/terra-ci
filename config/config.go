package config

import (
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	Configuration              *viper.Viper
	ConfigFilePath             = "config.yaml"
	LogLevel                   = "ERROR"
	DefaultModuleLocation      = ""
	DefaultWorkspaceProdBranch = "main"
	DefaultCiDirectory         = ".github/workflows/"
	StateMachineArn            = ""
	PlanStateMachineArn        = ""
	SfnExecutionTimeout        = 30
	RefreshRate                = 15
	CiMode                     = false
	ExperimentalFlow           = false
)

func init() {
	Configuration = viper.New()
	// enable automatic environment variable mapping
	// any variable prefixed with TERRA_CI will be mapped
	Configuration.SetEnvPrefix("TERRA_CI")
	Configuration.AutomaticEnv()

	// Set defaults
	Configuration.SetDefault("log_level", LogLevel)
	Configuration.SetDefault("default_module_location", DefaultModuleLocation)
	Configuration.SetDefault("default_workspace_prod_branch", DefaultWorkspaceProdBranch)
	Configuration.SetDefault("default_ci_directory", DefaultCiDirectory)
	Configuration.SetDefault("state_machine_arn", StateMachineArn)
	Configuration.SetDefault("sfn_execution_timeout", SfnExecutionTimeout)
	Configuration.SetDefault("ci_mode", CiMode)
	Configuration.SetDefault("refresh_rate", RefreshRate)
	Configuration.SetDefault("experimental_flow", ExperimentalFlow)
	Configuration.SetDefault("plan_sfn_arn", PlanStateMachineArn)
}

func AddConfigFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().StringVarP(&ConfigFilePath, "config-file-path", "c", ConfigFilePath, "Configuration file location")

	cmd.PersistentFlags().StringVarP(&LogLevel, "log-level", "l", LogLevel, "Log level")
	Configuration.BindPFlag("log_level", cmd.PersistentFlags().Lookup("log-level")) //nolint
	cmd.PersistentFlags().StringVarP(&DefaultModuleLocation, "default-module-location", "m", DefaultModuleLocation, "Default location to source terragrunt modules")
	Configuration.BindPFlag("default_module_location", cmd.PersistentFlags().Lookup("default-module-location")) //nolint
	cmd.PersistentFlags().StringVarP(&DefaultWorkspaceProdBranch, "default-workspace-prod-branch", "b", DefaultWorkspaceProdBranch, "Default Git branch used for production deployments")
	Configuration.BindPFlag("default_workspace_prod_branch", cmd.PersistentFlags().Lookup("default-workspace-prod-branch")) //nolint
	cmd.PersistentFlags().StringVarP(&DefaultCiDirectory, "default-ci-directory", "d", DefaultCiDirectory, "Default direcotory for ci workflows")
	Configuration.BindPFlag("default_ci_directory", cmd.PersistentFlags().Lookup("default-ci-directory")) //nolint
	cmd.PersistentFlags().StringVarP(&StateMachineArn, "state-machine-arn", "s", StateMachineArn, "AWS state machine arn to execute terragrunt commands")
	Configuration.BindPFlag("state_machine_arn", cmd.PersistentFlags().Lookup("state-machine-arn")) //nolint
	cmd.PersistentFlags().IntVarP(&SfnExecutionTimeout, "sfn-execution-timeout", "t", SfnExecutionTimeout, "AWS state machine timeout in minutes. This is local timeout after which CLI will stop polling the status")
	Configuration.BindPFlag("sfn_execution_timeout", cmd.PersistentFlags().Lookup("sfn-execution-timeout")) //nolint
	cmd.PersistentFlags().BoolVarP(&CiMode, "ci-mode", "i", CiMode, "Determine if runs in CI. Disables spinners")
	Configuration.BindPFlag("ci_mode", cmd.PersistentFlags().Lookup("ci-mode")) //nolint
	cmd.PersistentFlags().IntVarP(&RefreshRate, "refresh-rate", "r", RefreshRate, "Refresh rate of sfn execution status update")
	Configuration.BindPFlag("refresh_rate", cmd.PersistentFlags().Lookup("refresh-rate")) //nolint
	cmd.PersistentFlags().BoolVarP(&ExperimentalFlow, "experimental-flow", "e", ExperimentalFlow, "Use experimental code flow. Assume broken")
	Configuration.BindPFlag("experimental_flow", cmd.PersistentFlags().Lookup("experimental-flow")) //nolint
}

func LoadConfig(cmd *cobra.Command) error {
	// Read config from file
	filename := filepath.Base(ConfigFilePath)
	Configuration.SetConfigName(filename[:len(filename)-len(filepath.Ext(filename))])
	Configuration.AddConfigPath(filepath.Dir(ConfigFilePath))
	// check for err state and print warning message
	// continue to execute with defaults and cli flags
	if err := Configuration.ReadInConfig(); err != nil {
		return err
	}
	return nil
}
