package main

import (
	"database/sql"

	"github.com/coopernurse/gorp"
	_ "github.com/lib/pq"
)

func initDb() (dbmap *gorp.DbMap, err error) {
	// connect to db using standard Go database/sql API
	postgresConnection, found := config.GetString("postgresConnection")
	if !found {
		config.SettingNotFound("postgresConnection")
	}

	logger.Infof("Using Postgres connection string '%v'\n", postgresConnection)

	db, err := sql.Open("postgres", postgresConnection)
	if err != nil {
		return nil, err
	}

	// test the connection before using it
	err = db.Ping()
	if err != nil {
		return nil, err
	}

	dbmap = &gorp.DbMap{
		Db:      db,
		Dialect: gorp.PostgresDialect{},
	}

	return dbmap, nil
}
