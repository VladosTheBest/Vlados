package actions

import (
	"context"
	"encoding/json"
	"fmt"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/lib/gostop"
	"strconv"
	"sync"
	"time"

	"github.com/centrifugal/centrifuge"
	"github.com/rs/zerolog/log"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/lib/httpagent"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/monitor"
)

type _MarketStats struct {
	MarketID          string `json:"m"`
	Change            string `json:"ch"`
	Open              string `json:"o"`
	High              string `json:"h"`
	Low               string `json:"l"`
	Close             string `json:"c"`
	Volume            string `json:"v"`
	QuoteVolume       string `json:"qv"`
	QuoteVolumeChange string `json:"qvc"`
}

type quotVolumeMap map[string]float64

// start ticket stats publisher for all markets
func (srv *Actions) getTickerStatsPublisher(node *centrifuge.Node) gostop.StoppableRoutine {
	return func(ctx context.Context, wait *sync.WaitGroup) {
		log.Info().Str("cron", "ticker_stats_publisher").Str("action", "start").Msg("Ticker stats publisher - started")
		channel := "public:market/24h_tick"
		ticker := time.NewTicker(3 * time.Second)
		for {
			select {
			case <-ticker.C:
				// start timer
				start := time.Now()
				status, data, err := httpagent.Get(srv.cfg.Server.StatsEndpoint)
				if err != nil || status != 200 {
					// @todo commenting this out. It should be replaced with metrics instead of logging the error every second
					//log.Debug().Err(err).
					//	Int("status_code", status).
					//	Str("url", srv.cfg.Server.StatsEndpoint).
					//	Msg("Unable to get stats to publish on websocket")
					continue
				}
				stats := []_MarketStats{}
				err = json.Unmarshal(data, &stats)
				if err != nil {
					log.Error().Err(err).Msg("Unable to unmarshal data as an array of interfaces")
					continue
				}

				oldQVs := make(quotVolumeMap)
				status, data, err = httpagent.Get(srv.cfg.Server.QuoteVolumeDayBeforeEndpoint)
				if err == nil && status == 200 {
					_ = json.Unmarshal(data, &oldQVs)
				}

				for _, stat := range stats {
					quoteVolume, err := strconv.ParseFloat(stat.QuoteVolume, 64)
					if err != nil {
						continue
					}

					oldQuoteVolume, ok := oldQVs[stat.MarketID]
					if !ok {
						stat.QuoteVolumeChange = "0.00"
					} else {
						stat.QuoteVolumeChange = fmt.Sprintf("%.2f", oldQuoteVolume*100/quoteVolume-100)
					}

					data, err := json.Marshal(stat)
					if err != nil {
						log.Error().Err(err).Msg("Unable to marshal data for publishing")
						continue
					}

					if _, err := node.Publish(channel, data); err != nil {
						log.Error().
							Err(err).
							Str("channel", channel).
							Msg("Unable to publish on websocket")
					}
				}

				// end timer
				end := time.Now()
				monitor.TickerStatsDelay.Set(float64(end.Sub(start)))
				monitor.TickerStatsCount.Inc()
			case <-ctx.Done():
				ticker.Stop()
				log.Info().Str("cron", "ticker_stats_publisher").Str("action", "stop").Msg("14 => Ticker stats publisher - stopped")
				wait.Done()
				return
			}
		}
	}
}
