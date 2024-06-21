package actions

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"gitlab.com/paramountdax-exchange/exchange_api_v2/lib/gostop"

	"github.com/rs/zerolog/log"

	"github.com/centrifugal/centrifuge"
	marketCache "gitlab.com/paramountdax-exchange/exchange_api_v2/cache/markets"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/monitor"
)

func (srv *Actions) getMarketLevel2Publisher(node *centrifuge.Node) gostop.StoppableRoutine {
	return func(ctx context.Context, wait *sync.WaitGroup) {
		logger := log.With().
			Str("section", "websocket").
			Str("method", "getMarketLevel2Publisher").
			Logger()

		logger.Info().Str("cron", "market_level2_publisher").Str("action", "start").Msg("Market level2 publisher - started")

		ticker := time.NewTicker(1 * time.Second)
		for {
			select {
			case <-ticker.C:
				markets := marketCache.GetAllActive()
				for _, market := range markets {
					id := market.ID
					// @fixme Eliminate creating new goroutines every second for each market
					go func(market string) {
						// start timer
						start := time.Now()
						depth, err := srv.service.OMS.GetMarketDepthLevel2ByID(market, 50)
						if err != nil {
							//logger.Error().Err(err).Msg("Can not get depth lvl")
							return
						}
						data, err := json.Marshal(depth)
						if err != nil {
							logger.Error().Err(err).Msg("Can not marshal json value")
							return
						}
						channel := fmt.Sprintf("public:market-depth/%s", market)
						if _, err := node.Publish(channel, data); err != nil {
							logger.Error().Err(err).
								Str("market", market).
								Str("channel", channel).
								Msg("Unable to publish on websocket")
						}
						// end timer
						end := time.Now()
						monitor.DepthLevel2Delay.WithLabelValues(market).Set(float64(end.Sub(start)))
						monitor.DepthLevel2Count.WithLabelValues(market).Inc()
					}(id)
				}
			case <-ctx.Done():
				ticker.Stop()
				log.Info().Str("cron", "market_level2_publisher").Str("action", "stop").Msg("15 => Market level2 publisher - stopped")
				wait.Done()
				return
			}
		}
	}
}
