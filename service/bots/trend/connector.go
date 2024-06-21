package botTrendConnector

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/rs/zerolog/log"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/config"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/featureflags"
	"io/ioutil"
	"net/http"
)

type botConnector struct {
	config config.BotTrendConfig
}

var connector *botConnector

func (connector *botConnector) GetStartUrl() string {
	return connector.config.ApiEndpoints.Start
}

func (connector *botConnector) GetStopUrl() string {
	return connector.config.ApiEndpoints.Stop
}

func (connector *botConnector) Request(url string, b []byte) (int, []byte, error) {
	body := bytes.NewReader(b)
	resp, err := http.Post(url, "application/json", body)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	defer resp.Body.Close()

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, errors.New("unable to read service response")
	}

	return resp.StatusCode, bodyBytes, nil
}

func Init(c config.BotTrendConfig) {
	connector = &botConnector{
		config: c,
	}
}

func Start(body []byte) (string, error) {

	var respStruct struct {
		GridTradingBotId string `json:"trendBotId,omitempty"`
		Status           int    `json:"status,omitempty"`
		Error            string `json:"error,omitempty"`
		Message          string `json:"message,omitempty"`
	}

	code, resp, err := connector.Request(connector.GetStartUrl(), body)
	if err != nil {
		return "", err
	}

	if err := json.Unmarshal(resp, &respStruct); err != nil {
		return "", err
	}

	if code == http.StatusOK {
		return respStruct.GridTradingBotId, nil
	}

	return respStruct.Message, errors.New(respStruct.Message)
}

func GetStartUrl() string {
	return connector.GetStartUrl()
}

func Stop(botSystemId string) error {

	var bodyStruct = struct {
		BotSystemId string `json:"botSystemId"`
	}{
		BotSystemId: botSystemId,
	}

	ignore := featureflags.IsEnabled("api.bots.trend.requests-ignore-errors")

	b, err := json.Marshal(bodyStruct)
	if err != nil {
		if ignore {
			log.Error().Err(err).
				Str("botSystemId", botSystemId).
				Msg("Received error on the bot service response")
		} else {
			return err
		}
	}

	code, _, err := connector.Request(connector.GetStopUrl(), b)
	if code != http.StatusOK || err != nil {
		if ignore {
			log.Error().Err(err).
				Str("botSystemId", botSystemId).
				Msg("Received error on the bot service response")
		} else {
			return err
		}
	}

	return nil
}
