package logs

import (
	"strings"

	"github.com/p0tr3c/terra-ci/config"

	"go.uber.org/zap"
)

var (
	LogConfig zap.Config
	Logger    *zap.SugaredLogger
)

func Init() error {
	LogConfig = zap.NewProductionConfig()
	LogConfig.Level = GetAtomicLevel(config.LogLevel)

	logger, err := LogConfig.Build()
	if err != nil {
		return err
	}
	Logger = logger.Sugar()
	defer Logger.Sync() //nolint
	return nil
}

func GetAtomicLevel(level string) zap.AtomicLevel {
	switch strings.ToUpper(level) {
	case "INFO":
		return zap.NewAtomicLevelAt(zap.InfoLevel)
	case "ERROR":
		return zap.NewAtomicLevelAt(zap.ErrorLevel)
	case "DEBUG":
		return zap.NewAtomicLevelAt(zap.DebugLevel)
	case "WARN":
		return zap.NewAtomicLevelAt(zap.WarnLevel)
	default:
		return zap.NewAtomicLevelAt(zap.InfoLevel)
	}
}

func SetAtomicLevel(level string) {
	switch strings.ToUpper(level) {
	case "INFO":
		LogConfig.Level.SetLevel(zap.InfoLevel)
	case "ERROR":
		LogConfig.Level.SetLevel(zap.ErrorLevel)
	case "DEBUG":
		LogConfig.Level.SetLevel(zap.DebugLevel)
	case "WARN":
		LogConfig.Level.SetLevel(zap.WarnLevel)
	default:
		LogConfig.Level.SetLevel(zap.InfoLevel)
	}
}

func UpdateLoggerConfig() {
	SetAtomicLevel(config.LogLevel)
}

func Flush() {
	Logger.Sync() //nolint
}
