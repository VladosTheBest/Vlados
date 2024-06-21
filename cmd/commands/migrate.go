package commands

import (
	"fmt"

	"github.com/rs/zerolog/log"

	cfg "gitlab.com/paramountdax-exchange/exchange_api_v2/config"

	"github.com/golang-migrate/migrate/v4"

	// import support for file mime type
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

// Migrate the current database schema to the new version
func Migrate(config cfg.Config) {
	dbConf := config.DatabaseCluster.Writer
	user := dbConf.Username
	pass := dbConf.Password
	host := dbConf.Host
	port := dbConf.Port
	name := dbConf.Name
	sslmode := dbConf.SSLmode

	uri := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s", user, pass, host, port, name, sslmode)

	m, err := migrate.New("file://./db/migrations", uri)

	if err != nil {
		log.Fatal().Err(err).Str("section", "migrate").Msg("Unable to connect to database [WRITER]")
		return
	}

	if err = m.Up(); err != nil && err != migrate.ErrNoChange {
		if errMapped, ok := err.(migrate.ErrDirty); ok {
			log.Fatal().Err(err).Str("section", "migrate").Int("version", errMapped.Version).Msg("Unable to execute migration")
		} else {
			log.Fatal().Err(err).Str("section", "migrate").Msg("Unable to execute unknown migration")
		}
		return
	}
	log.Info().Str("section", "migrate").Msg("Migrations executed successfully")
}
