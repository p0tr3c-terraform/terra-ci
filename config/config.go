package config

import (
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	Configuration         *viper.Viper
	ConfigFilePath        = "config.yaml"
	LogLevel              = "ERROR"
	DefaultModuleLocation = ""
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
}

func AddConfigFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().StringVarP(&ConfigFilePath, "config-file-path", "c", ConfigFilePath, "Configuration file location")

	cmd.PersistentFlags().StringVarP(&LogLevel, "log-level", "l", LogLevel, "Log level")
	Configuration.BindPFlag("log_level", cmd.PersistentFlags().Lookup("log-level")) //nolint
	cmd.PersistentFlags().StringVarP(&DefaultModuleLocation, "default-module-location", "m", DefaultModuleLocation, "Default location to source terragrunt modules")
	Configuration.BindPFlag("default_module_location", cmd.PersistentFlags().Lookup("default-module-location")) //nolint

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
