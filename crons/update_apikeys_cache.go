package crons

import (
	cache "gitlab.com/paramountdax-exchange/exchange_api_v2/cache/apikey"
	cacheV2 "gitlab.com/paramountdax-exchange/exchange_api_v2/cache/apikeyV2"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/queries"
)

// CronUpdateAPIKeysCache godoc
func CronUpdateAPIKeysCache() {
	repo := queries.GetRepo()

	apiKeys, err := repo.GetMostActiveAPIKeys()
	if err != nil {
		//log.Debug().Err(err).Msg("Unable to update cached api keys list")
		return
	}

	newAPIKeys := make(map[string]*model.UserAPIKeys)
	for _, key := range apiKeys {
		newAPIKeys[key.Prefix] = key
	}

	apiKeysV2, err := repo.GetMostActiveAPIKeysV2()
	if err != nil {
		//log.Debug().Err(err).Msg("Unable to update cached api keys list")
		return
	}

	newAPIKeysV2 := make(map[string]*model.UserAPIKeysV2)
	for _, key := range apiKeysV2 {
		newAPIKeysV2[key.PublicKey] = key
		newAPIKeysV2[key.PrivateKey] = key
	}

	cache.SetAll(newAPIKeys)
	cacheV2.SetAll(newAPIKeysV2)
}
