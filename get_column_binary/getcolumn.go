package main

import (
	"database/cassandra"
	"flag"
	"log"
)

func main() {
	var uuid cassandra.UUID
	var conn *cassandra.RetryCassandraClient
	var r *cassandra.ColumnOrSuperColumn
	var ire *cassandra.InvalidRequestException
	var nfe *cassandra.NotFoundException
	var ue *cassandra.UnavailableException
	var te *cassandra.TimedOutException
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

	ire, err = conn.SetKeyspace(dbname)
	if ire != nil {
		log.Fatal(ire.Why)
	}
	if err != nil {
		log.Fatal(err)
	}

	cp = cassandra.NewColumnPath()
	cp.ColumnFamily = columnfamily
	cp.Column = []byte(column)

	r, ire, nfe, ue, te, err = conn.Get([]byte(uuid), cp,
		cassandra.ConsistencyLevel_ONE)
	if ire != nil {
		log.Fatal(ire.Why)
	}
	if nfe != nil {
		log.Fatal("Not found")
	}
	if ue != nil {
		log.Fatal("Unavailable")
	}
	if te != nil {
		log.Fatal("Timed out")
	}
	if err != nil {
		log.Fatal(err)
	}

	log.Print(r.Column.Name, ": ", r.Column.Value, " (",
		r.Column.Timestamp, ")")
}
