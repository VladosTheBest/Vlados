package actions

import (
	"context"
	"fmt"
	marketCache "gitlab.com/paramountdax-exchange/exchange_api_v2/cache/markets"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/lib/gostop"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/lib/httpagent"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/monitor"
	"sync"
	"time"

	"github.com/centrifugal/centrifuge"
	"github.com/rs/zerolog/log"
)

func (srv *Actions) getPublicTradesPublisher(node *centrifuge.Node) gostop.StoppableRoutine {
	return func(ctx context.Context, wait *sync.WaitGroup) {
		log.Info().Str("cron", "public_trades_publisher").Str("action", "start").Msg("Public trades publisher - started")
		ticker := time.NewTicker(1 * time.Second)
		for {
			select {
			case <-ticker.C:
				markets := marketCache.GetAllActive()
				for _, market := range markets {
					id := market.ID
					go func(market string) {
						// start timer
						start := time.Now()

						url := fmt.Sprintf("%s/%s", srv.cfg.Server.TradesEndpoint, market)
						status, data, err := httpagent.Get(url)
						if err != nil {
							log.Error().Err(err).
								Int("status_code", status).
								Str("market_id", market).
								Str("url", srv.cfg.Server.TradesEndpoint).
								Msg("Unable to get public trades to publish on websocket")
							return
						}
						channel := fmt.Sprintf("public:trades/%s", market)
						if _, err := node.Publish(channel, data); err != nil {
							log.Error().
								Err(err).
								Str("channel", channel).
								Msg("Unable to publish on websocket")
						}

						// end timer
						end := time.Now()
						monitor.PublicTradesDelay.WithLabelValues(market).Set(float64(end.Sub(start)))
						monitor.PublicTradesCount.WithLabelValues(market).Inc()
					}(id)
				}
			case <-ctx.Done():
				ticker.Stop()
				log.Info().Str("cron", "public_trades_publisher").Str("action", "stop").Msg("16 => Public trades publisher - stopped")
				wait.Done()
				return
			}
		}
	}
}
