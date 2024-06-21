package cmd

import (
	"os"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/config"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/featureflags"
)

// LogLevel Flag
var LogLevel = "info"

// LogFormat Flag
var LogFormat = "json"
var cfgFile string
var rootCmd = &cobra.Command{
	Use:   "exchange_api_v2",
	Short: "The backend of an exchange system",
	Long: `A component for exchange api. Created by Around25 to support high frequency trading on crypto markets.
	For a complete documentation and available licenses please contact https://around25.com`,
}

func init() {
	// set log level
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	initLoggingEnv()
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ./.config.yaml)")
	rootCmd.PersistentFlags().StringVarP(&LogLevel, "log-level", "", LogLevel, "logging level to show (options: debug|info|warn|error|fatal|panic, default: info)")
	rootCmd.PersistentFlags().StringVarP(&LogFormat, "log-format", "", LogFormat, "log format to generate (Options: json|pretty, default: json)")
}

func initConfig() {
	config.OpenConfig(cfgFile)
	customizeLogger()
	cfg := config.LoadConfig(viper.GetViper())
	// init featureflags
	if err := featureflags.Initialize(cfg.Unleash); err != nil {
		log.Fatal().Err(err).Str("lib", "unleash").Msg("Unable to init feature flags")
	}
}

func initLoggingEnv() {
	// load log level from env by default
	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel != "" {
		LogLevel = logLevel
	}
	// load log format from env by default
	logFormat := os.Getenv("LOG_FORMAT")
	if logFormat != "" {
		LogFormat = logFormat
	}
}

// Execute the commands
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal().Err(err)
	}
}

func customizeLogger() {
	if LogFormat == "pretty" {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	}
	switch LogLevel {
	case "debug":
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	case "info":
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	case "warn":
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	case "error":
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	case "fatal":
		zerolog.SetGlobalLevel(zerolog.FatalLevel)
	case "panic":
		zerolog.SetGlobalLevel(zerolog.PanicLevel)
	default:
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}

	gin.SetMode(gin.ReleaseMode)
	if gin.IsDebugging() {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}
}
