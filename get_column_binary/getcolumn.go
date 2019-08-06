package main

import (
	"context"
	"database/cassandra"
	"flag"
	"log"
)

func main() {
	var uuid cassandra.UUID
	var conn *cassandra.RetryCassandraClient
	var r *cassandra.ColumnOrSuperColumn
	var cp *cassandra.ColumnPath
	var err error

	var uuid_str, dbserver, dbname, columnfamily, column string

	flag.StringVar(&uuid_str, "uuid-string", "",
		"UUID string to look at")
	flag.StringVar(&dbserver, "cassandra-server", "localhost:9160",
		"Database server to look at")
	flag.StringVar(&dbname, "dbname", "sfmembersys",
		"Database name to look at")
	flag.StringVar(&columnfamily, "column-family", "",
		"Column family to look at")
	flag.StringVar(&column, "column-name", "",
		"Column name to look at")
	flag.Parse()

	uuid, err = cassandra.ParseUUID(uuid_str)
	if err != nil {
		log.Fatal(err)
	}

	conn, err = cassandra.NewRetryCassandraClient(dbserver)
	if err != nil {
		log.Fatal(err)
	}

	err = conn.SetKeyspace(context.Background(), dbname)
	if err != nil {
		log.Fatal(err)
	}

	cp = cassandra.NewColumnPath()
	cp.ColumnFamily = columnfamily
	cp.Column = []byte(column)

	r, err = conn.Get(context.Background(), []byte(uuid), cp,
		cassandra.ConsistencyLevel_ONE)
	if err != nil {
		log.Fatal(err)
	}

	log.Print(r.Column.Name, ": ", r.Column.Value, " (",
		r.Column.Timestamp, ")")
}
