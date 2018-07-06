package main

import (
	"database/cassandra"
	"flag"
	"log"
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
	&cassandra.CfDef{
		Name:               "application",
		ComparatorType:     "AsciiType",
		Comment:            mkstringp("Membership applications"),
		KeyValidationClass: mkstringp("BytesType"),
		ColumnType:         "Standard",
		Caching:            "keys_only",
		SpeculativeRetry:   "100ms",
		ColumnMetadata: []*cassandra.ColumnDef{
			&cassandra.ColumnDef{
				Name:            []byte("name"),
				ValidationClass: "UTF8Type",
			},
			&cassandra.ColumnDef{
				Name:            []byte("street"),
				ValidationClass: "UTF8Type",
			},
			&cassandra.ColumnDef{
				Name:            []byte("city"),
				ValidationClass: "UTF8Type",
			},
			&cassandra.ColumnDef{
				Name:            []byte("zipcode"),
				ValidationClass: "UTF8Type",
			},
			&cassandra.ColumnDef{
				Name:            []byte("country"),
				ValidationClass: "UTF8Type",
			},
			&cassandra.ColumnDef{
				Name:            []byte("email"),
				ValidationClass: "UTF8Type",
			},
			&cassandra.ColumnDef{
				Name:            []byte("phone"),
				ValidationClass: "UTF8Type",
			},
			&cassandra.ColumnDef{
				Name:            []byte("username"),
				ValidationClass: "UTF8Type",
			},
			&cassandra.ColumnDef{
				Name:            []byte("sourceip"),
				ValidationClass: "AsciiType",
			},
			&cassandra.ColumnDef{
				Name:            []byte("useragent"),
				ValidationClass: "UTF8Type",
			},
			&cassandra.ColumnDef{
				Name:            []byte("pwhash"),
				ValidationClass: "UTF8Type",
			},
			&cassandra.ColumnDef{
				Name:            []byte("fee"),
				ValidationClass: "LongType",
			},
			&cassandra.ColumnDef{
				Name:            []byte("email_verified"),
				ValidationClass: "BooleanType",
			},
			&cassandra.ColumnDef{
				Name:            []byte("fee_yearly"),
				ValidationClass: "BooleanType",
			},
			&cassandra.ColumnDef{
				Name:            []byte("pb_data"),
				ValidationClass: "BytesType",
			},
		},
	},
	// column family: members
	&cassandra.CfDef{
		Name:               "members",
		ComparatorType:     "AsciiType",
		Comment:            mkstringp("Current Starship Factory members"),
		KeyValidationClass: mkstringp("AsciiType"),
		ColumnType:         "Standard",
		Caching:            "keys_only",
		SpeculativeRetry:   "20ms",
		ColumnMetadata: []*cassandra.ColumnDef{
			&cassandra.ColumnDef{
				Name:            []byte("name"),
				ValidationClass: "UTF8Type",
				IndexType:       mkindextypep(cassandra.IndexType_KEYS),
			},
			&cassandra.ColumnDef{
				Name:            []byte("street"),
				ValidationClass: "UTF8Type",
				IndexType:       mkindextypep(cassandra.IndexType_KEYS),
			},
			&cassandra.ColumnDef{
				Name:            []byte("city"),
				ValidationClass: "UTF8Type",
				IndexType:       mkindextypep(cassandra.IndexType_KEYS),
			},
			&cassandra.ColumnDef{
				Name:            []byte("zipcode"),
				ValidationClass: "UTF8Type",
				IndexType:       mkindextypep(cassandra.IndexType_KEYS),
			},
			&cassandra.ColumnDef{
				Name:            []byte("country"),
				ValidationClass: "UTF8Type",
				IndexType:       mkindextypep(cassandra.IndexType_KEYS),
			},
			&cassandra.ColumnDef{
				Name:            []byte("email"),
				ValidationClass: "UTF8Type",
				IndexType:       mkindextypep(cassandra.IndexType_KEYS),
			},
			&cassandra.ColumnDef{
				Name:            []byte("phone"),
				ValidationClass: "UTF8Type",
			},
			&cassandra.ColumnDef{
				Name:            []byte("username"),
				ValidationClass: "UTF8Type",
				IndexType:       mkindextypep(cassandra.IndexType_KEYS),
			},
			&cassandra.ColumnDef{
				Name:            []byte("fee"),
				ValidationClass: "LongType",
				IndexType:       mkindextypep(cassandra.IndexType_KEYS),
			},
			&cassandra.ColumnDef{
				Name:            []byte("fee_yearly"),
				ValidationClass: "BooleanType",
				IndexType:       mkindextypep(cassandra.IndexType_KEYS),
			},
			&cassandra.ColumnDef{
				Name:            []byte("has_key"),
				ValidationClass: "BooleanType",
				IndexType:       mkindextypep(cassandra.IndexType_KEYS),
			},
			&cassandra.ColumnDef{
				Name:            []byte("payments_caught_up_to"),
				ValidationClass: "LongType",
				IndexType:       mkindextypep(cassandra.IndexType_KEYS),
			},
			&cassandra.ColumnDef{
				Name:            []byte("approval_ts"),
				ValidationClass: "LongType",
				IndexType:       mkindextypep(cassandra.IndexType_KEYS),
			},
			&cassandra.ColumnDef{
				Name:            []byte("agreement_pdf"),
				ValidationClass: "BytesType",
			},
			&cassandra.ColumnDef{
				Name:            []byte("pb_data"),
				ValidationClass: "BytesType",
			},
		},
	},
	// column family: member_agreements
	&cassandra.CfDef{
		Name:               "member_agreements",
		ComparatorType:     "AsciiType",
		Comment:            mkstringp("PDFs of membership agreements"),
		KeyValidationClass: mkstringp("AsciiType"),
		ColumnType:         "Standard",
		Caching:            "keys_only",
		SpeculativeRetry:   "100ms",
		ColumnMetadata: []*cassandra.ColumnDef{
			&cassandra.ColumnDef{
				Name:            []byte("agreement_pdf"),
				ValidationClass: "BytesType",
			},
			&cassandra.ColumnDef{
				Name:            []byte("pb_data"),
				ValidationClass: "BytesType",
			},
		},
	},
	// column family: membership_queue
	&cassandra.CfDef{
		Name:               "membership_queue",
		ComparatorType:     "AsciiType",
		Comment:            mkstringp("Queue of approved membership agreements"),
		KeyValidationClass: mkstringp("BytesType"),
		ColumnType:         "Standard",
		Caching:            "keys_only",
		SpeculativeRetry:   "100ms",
		ColumnMetadata: []*cassandra.ColumnDef{
			&cassandra.ColumnDef{
				Name:            []byte("pb_data"),
				ValidationClass: "BytesType",
			},
		},
	},
	// column family: membership_dequeue
	&cassandra.CfDef{
		Name:               "membership_dequeue",
		ComparatorType:     "AsciiType",
		Comment:            mkstringp("Queue of departing members for deletion"),
		KeyValidationClass: mkstringp("BytesType"),
		ColumnType:         "Standard",
		Caching:            "keys_only",
		SpeculativeRetry:   "100ms",
		ColumnMetadata: []*cassandra.ColumnDef{
			&cassandra.ColumnDef{
				Name:            []byte("pb_data"),
				ValidationClass: "BytesType",
			},
		},
	},
	// column family: membership_archive
	&cassandra.CfDef{
		Name:               "membership_archive",
		ComparatorType:     "AsciiType",
		Comment:            mkstringp("Recently departed former members"),
		KeyValidationClass: mkstringp("BytesType"),
		ColumnType:         "Standard",
		Caching:            "keys_only",
		SpeculativeRetry:   "100ms",
		ColumnMetadata: []*cassandra.ColumnDef{
			&cassandra.ColumnDef{
				Name:            []byte("pb_data"),
				ValidationClass: "BytesType",
			},
		},
	},
}

func main() {
	var existing_cfs []string = make([]string, 0)
	var ks_def *cassandra.KsDef
	var cf_def *cassandra.CfDef
	var conn *cassandra.RetryCassandraClient
	var err error

	var dbserver, dbname string

	flag.StringVar(&dbserver, "cassandra-server", "localhost:9160",
		"Database server to set up")
	flag.StringVar(&dbname, "dbname", "sfmembersys",
		"Database name to set up")
	flag.Parse()

	conn, err = cassandra.NewRetryCassandraClient(dbserver)
	if err != nil {
		log.Fatal(err)
	}

	err = conn.SetKeyspace(dbname)
	if err != nil {
		log.Fatal(err)
	}

	ks_def, err = conn.DescribeKeyspace(dbname)
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
			rv, err = conn.SystemUpdateColumnFamily(cf_def)
			if err != nil {
				log.Fatal("Unable to update column family ", cf_def.Name, ": ", err)
			}
			log.Print("Successfully updated column family ", cf_def.Name, ": ", rv)
		} else {
			var rv string
			rv, err = conn.SystemAddColumnFamily(cf_def)
			if err != nil {
				log.Fatal("Unable to add column family ", cf_def.Name, ": ", err)
			}
			log.Print("Successfully added column family ", cf_def.Name, ": ", rv)
		}
	}
}
