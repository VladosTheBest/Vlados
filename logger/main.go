package logger

import (
	"context"
	"net/http"
	"regexp"
	"strings"
	"time"

	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"

	"github.com/gin-gonic/gin"
	"github.com/rs/xid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"gitlab.com/paramountdax-exchange/exchange_api_v2/monitor"
)

type TimeContext string

const (
	TimelogsKey TimeContext = "_timelogs"
)

type CustomEntryContext string

const (
	CustomEntryLogsKey CustomEntryContext = "_customEntrylogs"
)

type TimeLog struct {
	Key   string
	Value time.Time
}

// Config for logger
type Config struct {
	Logger *zerolog.Logger
	// UTC a boolean stating whether to use UTC time zone or local.
	UTC            bool
	SkipPath       []string
	SkipPathRegexp *regexp.Regexp
}

// GetLogger from gin context
func GetLogger(c *gin.Context) zerolog.Logger {
	if logger, ok := c.Get("_log"); ok {
		return logger.(zerolog.Logger)
	}
	return log.Logger
}

// LogTimestamp godoc
func LogTimestamp(c context.Context, key string, t time.Time) {
	if v := c.Value(TimelogsKey); v != nil {
		timelogs := v.(*[]TimeLog)
		*timelogs = append(*timelogs, TimeLog{Key: key, Value: t})
	}
}

// GetTimestamps godoc
func GetTimestamps(c context.Context) []TimeLog {
	if v := c.Value(TimelogsKey); v != nil {
		return *v.(*[]TimeLog)
	}
	return []TimeLog{}
}

// SetLogger initializes the logging middleware.
func SetLogger(config ...Config) gin.HandlerFunc {
	var newConfig Config
	if len(config) > 0 {
		newConfig = config[0]
	}
	var skip map[string]struct{}
	if length := len(newConfig.SkipPath); length > 0 {
		skip = make(map[string]struct{}, length)
		for _, path := range newConfig.SkipPath {
			skip[path] = struct{}{}
		}
	}

	var sublog zerolog.Logger
	if newConfig.Logger == nil {
		sublog = log.Logger
	} else {
		sublog = *newConfig.Logger
	}

	return func(c *gin.Context) {
		var start time.Time
		var ctx context.Context
		var timeCtx context.Context

		// get full url path for logs
		path := c.Request.URL.Path
		fullPath := path
		raw := c.Request.URL.RawQuery
		if raw != "" {
			fullPath = path + "?" + raw
		}

		symbol := c.Param("market")

		track := true
		watch := strings.HasPrefix(path, "/orders") || strings.HasPrefix(path, "/internal/orders") || strings.HasPrefix(path, "/internal/bots") || strings.HasPrefix(path, "/bulk/orders")

		if c.Request.Method == "GET" {
			track = false
			watch = false
		}

		if _, ok := skip[path]; ok {
			track = false
		}

		if track &&
			newConfig.SkipPathRegexp != nil &&
			newConfig.SkipPathRegexp.MatchString(path) {
			track = false
		}

		id := xid.New().String()
		c.Writer.Header().Set("X-Request-Id", id)
		var reqlogger zerolog.Logger

		if watch {
			monitor.APIOrderRequestQueue.WithLabelValues().Inc()
			start = time.Now()
			ctx = context.Background()
			timelogs := make([]TimeLog, 0, 13)
			timeCtx = context.WithValue(ctx, TimelogsKey, &timelogs)
			c.Set("_timecontext", timeCtx)
			LogTimestamp(timeCtx, "start", start)
		}

		if track {
			reqlogger = sublog.With().
				Str("request_id", id).
				Logger()
			c.Set("_log", reqlogger)
		}

		c.Next()

		// monitor the requests to the server
		if watch {
			monitor.APIOrderRequestQueue.WithLabelValues().Dec()
			LogTimestamp(timeCtx, "finish", time.Now())
			timestamps := GetTimestamps(timeCtx)
			for i := range timestamps {
				monitor.RequestDelay.WithLabelValues(symbol, c.Request.Method, timestamps[i].Key).Set(float64(timestamps[i].Value.Sub(timestamps[0].Value)))
			}
		}

		if track && c.Writer.Status() >= http.StatusBadRequest {
			msg := "Request"
			if len(c.Errors) > 0 {
				msg = c.Errors.String()
			}

			timeDict := zerolog.Dict()
			if timeCtx != nil {
				timestamps := GetTimestamps(timeCtx)
				for i := range timestamps {
					timeDict.Dur(timestamps[i].Key, timestamps[i].Value.Sub(timestamps[0].Value))
				}
			}

			dumplogger := reqlogger.With().
				Str("method", c.Request.Method).
				Str("path", fullPath).
				Str("ip", c.ClientIP()).
				Str("user-agent", c.Request.UserAgent()).
				Int("status", c.Writer.Status()).
				Dict("latencies", timeDict).
				Logger()

			if val, ok := c.Get("auth_user_id"); ok {
				dumplogger = dumplogger.With().Uint64("user_id", val.(uint64)).Logger()
			}

			if val, ok := c.Get("sub_account"); ok {
				dumplogger = dumplogger.With().Uint64("subAccount", val.(uint64)).Logger()
			}

			if iMarket, ok := c.Get("data_market"); ok {
				market := iMarket.(*model.Market)
				dumplogger = dumplogger.With().Str("market", market.ID).Logger()
			}

			switch {
			case c.Writer.Status() >= http.StatusBadRequest && c.Writer.Status() < http.StatusInternalServerError:
				dumplogger.Warn().Stack().Msg(msg)
			case c.Writer.Status() >= http.StatusInternalServerError:
				dumplogger.Error().Msg(msg)
			default:
				dumplogger.Info().Msg(msg)
			}
		}

	}
}
