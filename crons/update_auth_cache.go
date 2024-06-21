package crons

import (
	"github.com/rs/zerolog/log"
	cache "gitlab.com/paramountdax-exchange/exchange_api_v2/cache/auth"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/queries"
)

// CronUpdateAuthCache godoc
func CronUpdateAuthCache() {
	repo := queries.GetRepo()

	rolePermissions, err := repo.GetRolePermissions()
	if err != nil {
		log.Error().Err(err).Msg("Unable to update cached role permissions")
		return
	}

	roles := formatRolePermissions(rolePermissions)
	cache.SetAll(roles)
	// log.Debug().Str("section", "cron:market_cache").Int("count", len(markets)).Msg("Market cache updated")
}

func formatRolePermissions(rolePermissions []model.RolePermission) map[string]map[string]bool {
	roles := make(map[string]map[string]bool)
	for _, perm := range rolePermissions {
		if _, ok := roles[perm.RoleAlias]; !ok {
			roles[perm.RoleAlias] = make(map[string]bool)
		}
		roles[perm.RoleAlias][perm.PermissionAlias] = true
	}
	return roles
}
