package main

import (
	"context"
	"database/cassandra"
	"flag"
	"log"
	"time"
)

func mkstringp(input string) *string {
	var rv *string = new(string)
	*rv = input
	return rv
}

func mkindextypep(input cassandra.IndexType) *cassandra.IndexType {
	var rv *cassandra.IndexType = new(cassandra.IndexType)
	*rv = input
	return rv
}

func contains(list []string, elem string) bool {
	var i string

	for _, i = range list {
		if i == elem {
			return true
		}
	}

	return false
}

var desired_cf_defs = []*cassandra.CfDef{
	// column family: application
	{
		Name:               "application",
		ComparatorType:     "AsciiType",
		Comment:            mkstringp("Membership applications"),
		KeyValidationClass: mkstringp("BytesType"),
		ColumnType:         "Standard",
		Caching:            "keys_only",
		SpeculativeRetry:   "100ms",
		ColumnMetadata: []*cassandra.ColumnDef{
			{
				Name:            []byte("name"),
				ValidationClass: "UTF8Type",
			},
			{
				Name:            []byte("street"),
				ValidationClass: "UTF8Type",
			},
			{
				Name:            []byte("city"),
				ValidationClass: "UTF8Type",
			},
			{
				Name:            []byte("zipcode"),
				ValidationClass: "UTF8Type",
			},
			{
				Name:            []byte("country"),
				ValidationClass: "UTF8Type",
			},
			{
				Name:            []byte("email"),
				ValidationClass: "UTF8Type",
			},
			{
				Name:            []byte("phone"),
				ValidationClass: "UTF8Type",
			},
			{
				Name:            []byte("username"),
				ValidationClass: "UTF8Type",
			},
			{
				Name:            []byte("sourceip"),
				ValidationClass: "AsciiType",
			},
			{
				Name:            []byte("useragent"),
				ValidationClass: "UTF8Type",
			},
			{
				Name:            []byte("pwhash"),
				ValidationClass: "UTF8Type",
			},
			{
				Name:            []byte("fee"),
				ValidationClass: "LongType",
			},
			{
				Name:            []byte("email_verified"),
				ValidationClass: "BooleanType",
			},
			{
				Name:            []byte("fee_yearly"),
				ValidationClass: "BooleanType",
			},
			{
				Name:            []byte("pb_data"),
				ValidationClass: "BytesType",
			},
		},
	},
	// column family: members
	{
		Name:               "members",
		ComparatorType:     "AsciiType",
		Comment:            mkstringp("Current Starship Factory members"),
		KeyValidationClass: mkstringp("AsciiType"),
		ColumnType:         "Standard",
		Caching:            "keys_only",
		SpeculativeRetry:   "20ms",
		ColumnMetadata: []*cassandra.ColumnDef{
			{
				Name:            []byte("name"),
				ValidationClass: "UTF8Type",
				IndexType:       mkindextypep(cassandra.IndexType_KEYS),
			},
			{
				Name:            []byte("street"),
				ValidationClass: "UTF8Type",
				IndexType:       mkindextypep(cassandra.IndexType_KEYS),
			},
			{
				Name:            []byte("city"),
				ValidationClass: "UTF8Type",
				IndexType:       mkindextypep(cassandra.IndexType_KEYS),
			},
			{
				Name:            []byte("zipcode"),
				ValidationClass: "UTF8Type",
				IndexType:       mkindextypep(cassandra.IndexType_KEYS),
			},
			{
				Name:            []byte("country"),
				ValidationClass: "UTF8Type",
				IndexType:       mkindextypep(cassandra.IndexType_KEYS),
			},
			{
				Name:            []byte("email"),
				ValidationClass: "UTF8Type",
				IndexType:       mkindextypep(cassandra.IndexType_KEYS),
			},
			{
				Name:            []byte("phone"),
				ValidationClass: "UTF8Type",
			},
			{
				Name:            []byte("username"),
				ValidationClass: "UTF8Type",
				IndexType:       mkindextypep(cassandra.IndexType_KEYS),
			},
			{
				Name:            []byte("fee"),
				ValidationClass: "LongType",
				IndexType:       mkindextypep(cassandra.IndexType_KEYS),
			},
			{
				Name:            []byte("fee_yearly"),
				ValidationClass: "BooleanType",
				IndexType:       mkindextypep(cassandra.IndexType_KEYS),
			},
			{
				Name:            []byte("has_key"),
				ValidationClass: "BooleanType",
				IndexType:       mkindextypep(cassandra.IndexType_KEYS),
			},
			{
				Name:            []byte("payments_caught_up_to"),
				ValidationClass: "LongType",
				IndexType:       mkindextypep(cassandra.IndexType_KEYS),
			},
			{
				Name:            []byte("approval_ts"),
				ValidationClass: "LongType",
				IndexType:       mkindextypep(cassandra.IndexType_KEYS),
			},
			{
				Name:            []byte("agreement_pdf"),
				ValidationClass: "BytesType",
			},
			{
				Name:            []byte("pb_data"),
				ValidationClass: "BytesType",
			},
		},
	},
	// column family: member_agreements
	{
		Name:               "member_agreements",
		ComparatorType:     "AsciiType",
		Comment:            mkstringp("PDFs of membership agreements"),
		KeyValidationClass: mkstringp("AsciiType"),
		ColumnType:         "Standard",
		Caching:            "keys_only",
		SpeculativeRetry:   "100ms",
		ColumnMetadata: []*cassandra.ColumnDef{
			{
				Name:            []byte("agreement_pdf"),
				ValidationClass: "BytesType",
			},
			{
				Name:            []byte("pb_data"),
				ValidationClass: "BytesType",
			},
		},
	},
	// column family: membership_queue
	{
		Name:               "membership_queue",
		ComparatorType:     "AsciiType",
		Comment:            mkstringp("Queue of approved membership agreements"),
		KeyValidationClass: mkstringp("BytesType"),
		ColumnType:         "Standard",
		Caching:            "keys_only",
		SpeculativeRetry:   "100ms",
		ColumnMetadata: []*cassandra.ColumnDef{
			{
				Name:            []byte("pb_data"),
				ValidationClass: "BytesType",
			},
		},
	},
	// column family: membership_dequeue
	{
		Name:               "membership_dequeue",
		ComparatorType:     "AsciiType",
		Comment:            mkstringp("Queue of departing members for deletion"),
		KeyValidationClass: mkstringp("BytesType"),
		ColumnType:         "Standard",
		Caching:            "keys_only",
		SpeculativeRetry:   "100ms",
		ColumnMetadata: []*cassandra.ColumnDef{
			{
				Name:            []byte("pb_data"),
				ValidationClass: "BytesType",
			},
		},
	},
	// column family: membership_archive
	{
		Name:               "membership_archive",
		ComparatorType:     "AsciiType",
		Comment:            mkstringp("Recently departed former members"),
		KeyValidationClass: mkstringp("BytesType"),
		ColumnType:         "Standard",
		Caching:            "keys_only",
		SpeculativeRetry:   "100ms",
		ColumnMetadata: []*cassandra.ColumnDef{
			{
				Name:            []byte("pb_data"),
				ValidationClass: "BytesType",
			},
		},
	},
}

func main() {
	var ctx context.Context
	var cancel context.CancelFunc
	var existing_cfs []string = make([]string, 0)
	var ks_def *cassandra.KsDef
	var cf_def *cassandra.CfDef
	var conn *cassandra.RetryCassandraClient
	var batchOpTimeout time.Duration
	var err error

	var dbserver, dbname string

	flag.StringVar(&dbserver, "cassandra-server", "localhost:9160",
		"Database server to set up")
	flag.StringVar(&dbname, "dbname", "sfmembersys",
		"Database name to set up")
	flag.DurationVar(&batchOpTimeout, "batch-op-timeout",
		5*time.Minute, "Timeout for batch operations")
	flag.Parse()

	ctx, cancel = context.WithTimeout(context.Background(), batchOpTimeout)
	defer cancel()

	conn, err = cassandra.NewRetryCassandraClient(dbserver)
	if err != nil {
		log.Fatal(err)
	}

	err = conn.SetKeyspace(ctx, dbname)
	if err != nil {
		log.Fatal(err)
	}

	ks_def, err = conn.DescribeKeyspace(ctx, dbname)
	if err != nil {
		log.Fatal("Error describing keyspace ", dbname, ": ", err)
	}

	for _, cf_def = range ks_def.GetCfDefs() {
		existing_cfs = append(existing_cfs, cf_def.Name)
		log.Print("Found existing column family ", cf_def.Name)
	}

	for _, cf_def = range desired_cf_defs {
		cf_def.Keyspace = dbname

		if contains(existing_cfs, cf_def.Name) {
			var rv string
			rv, err = conn.SystemUpdateColumnFamily(ctx, cf_def)
			if err != nil {
				log.Fatal("Unable to update column family ", cf_def.Name, ": ", err)
			}
			log.Print("Successfully updated column family ", cf_def.Name, ": ", rv)
		} else {
			var rv string
			rv, err = conn.SystemAddColumnFamily(ctx, cf_def)
			if err != nil {
				log.Fatal("Unable to add column family ", cf_def.Name, ": ", err)
			}
			log.Print("Successfully added column family ", cf_def.Name, ": ", rv)
		}
	}
}
