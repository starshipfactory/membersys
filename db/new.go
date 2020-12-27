package db

import (
	"errors"
	"time"

	"github.com/starshipfactory/membersys"
	"github.com/starshipfactory/membersys/config"
)

// Create new database connection to the configured database configuration.
func New(dbConfig *config.DatabaseConfig) (membersys.MembershipDB, error) {
	if dbConfig.GetCassandra() != nil {
		var timeout time.Duration
		cassandra := dbConfig.GetCassandra()
		timeout = (time.Duration(cassandra.GetDatabaseTimeout()) *
			time.Millisecond)
		return NewCassandraDB(cassandra.GetDatabaseServer(),
			cassandra.GetDatabaseName(), timeout)
	}
	if dbConfig.GetPostgresql() != nil {
		postgresql := dbConfig.GetPostgresql()
		return NewPostgreSQLDB(postgresql.GetDatabaseServer(),
			postgresql.GetDatabaseName(), postgresql.GetUser(),
			postgresql.GetPassword(), postgresql.GetSsl())
	}
	return nil, errors.New("No database backend confgiured")
}
