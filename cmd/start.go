package cmd

import (
	"github.com/rs/zerolog/log"

	"gitlab.com/paramountdax-exchange/exchange_api_v2/cmd/commands"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/config"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/server"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func init() {
	rootCmd.AddCommand(startCmd)
}

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the market price service and listen for any orders and trades comming from the engine",
	Long:  `Connect to the configured message queue and listen for new requests or trades and generate market data`,
	Run: func(cmd *cobra.Command, args []string) {
		// load server configuration from server
		log.Debug().Msg("Loading server configuration")
		if viper.ConfigFileUsed() != "" {
			log.Debug().Str("section", "init").Str("path", viper.ConfigFileUsed()).Msg("Configuration file loaded")
		}
		cfg := config.LoadConfig(viper.GetViper())
		// Running migrations
		log.Debug().Msg("Running migrations")
		commands.Migrate(cfg)

		// start a new server
		log.Debug().Str("section", "init").Msg("Starting new server instance")
		srv := server.NewServer(cfg)
		// listen for new messages
		log.Info().Str("section", "init").Msg("Listening for incoming events")
		srv.Listen()
	},
}
