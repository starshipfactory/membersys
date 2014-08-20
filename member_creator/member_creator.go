/*
 * (c) 2014, Tonnerre Lombard <tonnerre@ancient-solutions.com>,
 *	     Starship Factory. All rights reserved.
 *
 * Redistribution and use in source  and binary forms, with or without
 * modification, are permitted  provided that the following conditions
 * are met:
 *
 * * Redistributions of  source code  must retain the  above copyright
 *   notice, this list of conditions and the following disclaimer.
 * * Redistributions in binary form must reproduce the above copyright
 *   notice, this  list of conditions and the  following disclaimer in
 *   the  documentation  and/or  other  materials  provided  with  the
 *   distribution.
 * * Neither  the name  of the Starship Factory  nor the  name  of its
 *   contributors may  be used to endorse or  promote products derived
 *   from this software without specific prior written permission.
 *
 * THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
 * "AS IS"  AND ANY EXPRESS  OR IMPLIED WARRANTIES  OF MERCHANTABILITY
 * AND FITNESS  FOR A PARTICULAR  PURPOSE ARE DISCLAIMED. IN  NO EVENT
 * SHALL THE COPYRIGHT OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT,
 * INDIRECT, INCIDENTAL, SPECIAL,  EXEMPLARY, OR CONSEQUENTIAL DAMAGES
 * (INCLUDING, BUT NOT LIMITED  TO, PROCUREMENT OF SUBSTITUTE GOODS OR
 * SERVICES; LOSS OF USE,  DATA, OR PROFITS; OR BUSINESS INTERRUPTION)
 * HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT,
 * STRICT  LIABILITY,  OR  TORT  (INCLUDING NEGLIGENCE  OR  OTHERWISE)
 * ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED
 * OF THE POSSIBILITY OF SUCH DAMAGE.
 */

package main

import (
	"code.google.com/p/goprotobuf/proto"
	"database/cassandra"
	"encoding/binary"
	"flag"
	"github.com/mqu/openldap"
	"io/ioutil"
	"log"
	"strconv"
	"strings"
	"time"
)

func makeMutation(mmap map[string][]*cassandra.Mutation, cf, name string,
	value []byte, now time.Time) {
	var m *cassandra.Mutation = cassandra.NewMutation()
	var col *cassandra.Column = cassandra.NewColumn()

	col.Name = []byte(name)
	col.Value = value
	col.Timestamp = now.UnixNano()

	m.ColumnOrSupercolumn = cassandra.NewColumnOrSuperColumn()
	m.ColumnOrSupercolumn.Column = col
	mmap[cf] = append(mmap[cf], m)
}

func makeMutationString(mmap map[string][]*cassandra.Mutation,
	cf, name, value string, now time.Time) {
	makeMutation(mmap, cf, name, []byte(value), now)
}

func makeMutationLong(mmap map[string][]*cassandra.Mutation,
	cf, name string, value uint64, now time.Time) {
	var val []byte = make([]byte, 8)
	binary.BigEndian.PutUint64(val, value)
	makeMutation(mmap, cf, name, val, now)
}

func makeMutationBool(mmap map[string][]*cassandra.Mutation,
	cf, name string, value bool, now time.Time) {
	var b byte
	if value {
		b = 0x1
	}
	makeMutation(mmap, cf, name, []byte{b}, now)
}

func asciiFilter(in string) string {
	var rv []rune
	var rn rune

	for _, rn = range []rune(in) {
		if rn < 128 {
			rv = append(rv, rn)
		}
	}

	return string(rv)
}

func main() {
	var cf string = "members"
	var config_file string
	var config_contents []byte
	var config MemberCreatorConfig
	var greatestUid uint64 = 1000
	var now time.Time
	var noop bool

	var ldap *openldap.Ldap
	var msg *openldap.LdapMessage
	var lres *openldap.LdapSearchResult

	var mmap map[string]map[string][]*cassandra.Mutation
	var db *cassandra.RetryCassandraClient
	var cp *cassandra.ColumnParent
	var pred *cassandra.SlicePredicate
	var kr *cassandra.KeyRange
	var kss []*cassandra.KeySlice
	var ks *cassandra.KeySlice

	var ire *cassandra.InvalidRequestException
	var ue *cassandra.UnavailableException
	var te *cassandra.TimedOutException
	var err error

	flag.StringVar(&config_file, "config", "",
		"Path to the member creator configuration file")
	flag.BoolVar(&noop, "dry-run", false, "Do a dry run")
	flag.Parse()

	if len(config_file) == 0 {
		flag.Usage()
		return
	}

	config_contents, err = ioutil.ReadFile(config_file)
	if err != nil {
		log.Fatal("Unable to read ", config_file, ": ", err)
	}

	err = proto.Unmarshal(config_contents, &config)
	if err != nil {
		err = proto.UnmarshalText(string(config_contents), &config)
	}
	if err != nil {
		log.Fatal("Unable to parse ", config_file, ": ", err)
	}
	now = time.Now()

	if !noop {
		ldap, err = openldap.Initialize(config.LdapConfig.GetServer())
		if err != nil {
			log.Fatal("Error connecting to LDAP server ",
				config.LdapConfig.GetServer(), ": ", err)
		}

		err = ldap.SetOption(openldap.LDAP_OPT_PROTOCOL_VERSION,
			openldap.LDAP_VERSION3)
		if err != nil {
			log.Print("Error setting version to 3: ", err)
		}

		err = ldap.SetOption(openldap.LDAP_OPT_SIZELIMIT, 0)
		if err != nil {
			log.Print("Error setting the size limit to 0: ", err)
		}

		err = ldap.Bind(config.LdapConfig.GetSuperUser()+","+
			config.LdapConfig.GetBase(), config.LdapConfig.GetSuperPassword())
		if err != nil {
			log.Fatal("Unable to bind as ", config.LdapConfig.GetSuperUser()+
				","+config.LdapConfig.GetBase(), " to ",
				config.LdapConfig.GetServer(), ": ", err)
		}
		defer ldap.Unbind()

		// Find the highest assigned UID.
		msg, err = ldap.Search(config.LdapConfig.GetBase(),
			openldap.LDAP_SCOPE_SUBTREE, "(objectClass=posixAccount)",
			[]string{"uidNumber"})
		if err != nil {
			log.Fatal("Unable to search for posix accounts in ",
				config.LdapConfig.GetBase(), ": ", err)
		}
		for msg != nil {
			var entry = msg.FirstEntry()

			for entry != nil {
				var uid string

				for _, uid = range entry.GetValues("uidNumber") {
					var uidNumber uint64
					uidNumber, err = strconv.ParseUint(uid, 10, 64)
					if err != nil {
						log.Print("Error parsing \"", uid, "\" as a number: ",
							err)
					} else if uidNumber > greatestUid {
						greatestUid = uidNumber
					}
				}
				entry = entry.NextEntry()
			}

			msg = msg.NextMessage()
		}
	}

	// Connect to Cassandra so we can get a list of records to be processed.
	db, err = cassandra.NewRetryCassandraClient(
		config.DatabaseConfig.GetDatabaseServer())
	if err != nil {
		log.Fatal("Error connecting to Cassandra database at ",
			config.DatabaseConfig.GetDatabaseServer(), ": ", err)
	}

	ire, err = db.SetKeyspace(config.DatabaseConfig.GetDatabaseName())
	if ire != nil {
		log.Fatal("Invalid Cassandra request: ", ire.Why)
	}
	if err != nil {
		log.Fatal("Error setting keyspace: ", err)
	}

	cp = cassandra.NewColumnParent()
	cp.ColumnFamily = "membership_queue"
	pred = cassandra.NewSlicePredicate()
	pred.ColumnNames = [][]byte{[]byte("pb_data")}
	kr = cassandra.NewKeyRange()
	kr.StartKey = make([]byte, 0)
	kr.EndKey = make([]byte, 0)

	kss, ire, ue, te, err = db.GetRangeSlices(cp, pred, kr,
		cassandra.ConsistencyLevel_QUORUM)
	if ire != nil {
		log.Fatal("Invalid Cassandra request: ", ire.Why)
	}
	if ue != nil {
		log.Fatal("Cassandra unavailable")
	}
	if te != nil {
		log.Fatal("Cassandra timed out: ", te.String())
	}
	if err != nil {
		log.Fatal("Error getting range slice: ", err)
	}

	mmap = make(map[string]map[string][]*cassandra.Mutation)
	for _, ks = range kss {
		var csc *cassandra.ColumnOrSuperColumn
		for _, csc = range ks.Columns {
			var col *cassandra.Column = csc.Column
			var agreement MembershipAgreement
			var attrs map[string][]string
			var m *cassandra.Mutation

			if col == nil {
				continue
			}

			if string(col.Name) != "pb_data" {
				log.Print("Column selected was not as requested: ",
					col.Name)
				continue
			}

			err = proto.Unmarshal(col.Value, &agreement)
			if err != nil {
				log.Print("Unable to parse column ", ks.Key, ": ", err)
				continue
			}

			greatestUid++
			attrs = make(map[string][]string)
			attrs["uidNumber"] = []string{
				strconv.FormatUint(greatestUid, 10)}
			attrs["gecos"] = []string{agreement.MemberData.GetName()}
			attrs["shadowLastChange"] = []string{"11457"}
			attrs["shadowMax"] = []string{"9999"}
			attrs["shadowWarning"] = []string{"7"}
			attrs["gidNumber"] = []string{strconv.FormatUint(uint64(
				config.LdapConfig.GetNewUserGid()), 10)}
			attrs["objectClass"] = []string{
				"account", "posixAccount", "shadowAccount", "top",
			}
			attrs["uid"] = []string{agreement.MemberData.GetUsername()}
			attrs["cn"] = []string{agreement.MemberData.GetUsername()}
			attrs["homeDirectory"] = []string{
				"/home/" + agreement.MemberData.GetUsername()}
			attrs["loginShell"] = []string{
				config.LdapConfig.GetNewUserShell()}
			attrs["userPassword"] = []string{
				agreement.MemberData.GetPwhash(),
			}

			for key, ivalue := range attrs {
				for i, value := range ivalue {
					attrs[key][i] = asciiFilter(value)
				}
			}

			if noop {
				log.Print("Would create user: ", attrs)
			} else {
				err = ldap.Add("uid="+agreement.MemberData.GetUsername()+
					","+config.LdapConfig.GetNewUserSuffix()+","+
					config.LdapConfig.GetBase(), attrs)
				if err != nil {
					log.Print("Unable to create user ",
						agreement.MemberData.GetUsername(), ": ", err)
					continue
				}
			}

			mmap[string(agreement.MemberData.GetEmail())] =
				make(map[string][]*cassandra.Mutation)
			mmap[string(agreement.MemberData.GetEmail())][cf] =
				make([]*cassandra.Mutation, 0)

			makeMutation(mmap[string(agreement.MemberData.GetEmail())],
				cf, "pb_data", col.Value, now)
			makeMutationString(mmap[string(agreement.MemberData.GetEmail())],
				cf, "name", agreement.MemberData.GetName(), now)
			makeMutationString(mmap[string(agreement.MemberData.GetEmail())],
				cf, "street", agreement.MemberData.GetStreet(), now)
			makeMutationString(mmap[string(agreement.MemberData.GetEmail())],
				cf, "city", agreement.MemberData.GetCity(), now)
			makeMutationString(mmap[string(agreement.MemberData.GetEmail())],
				cf, "country", agreement.MemberData.GetCountry(), now)
			makeMutationString(mmap[string(agreement.MemberData.GetEmail())],
				cf, "email", agreement.MemberData.GetEmail(), now)
			makeMutationString(mmap[string(agreement.MemberData.GetEmail())],
				cf, "phone", agreement.MemberData.GetPhone(), now)
			makeMutationString(mmap[string(agreement.MemberData.GetEmail())],
				cf, "username", agreement.MemberData.GetUsername(), now)
			makeMutationLong(mmap[string(agreement.MemberData.GetEmail())],
				cf, "fee", agreement.MemberData.GetFee(), now)
			makeMutationBool(mmap[string(agreement.MemberData.GetEmail())],
				cf, "fee_yearly", agreement.MemberData.GetFeeYearly(), now)
			makeMutationLong(mmap[string(agreement.MemberData.GetEmail())],
				cf, "approval_ts", agreement.Metadata.GetApprovalTimestamp(),
				now)
			makeMutation(mmap[string(agreement.MemberData.GetEmail())], cf,
				"agreement_pdf", agreement.AgreementPdf, now)

			// Now, delete the original record.
			m = cassandra.NewMutation()
			m.Deletion = cassandra.NewDeletion()
			m.Deletion.Predicate = cassandra.NewSlicePredicate()
			m.Deletion.Predicate.ColumnNames = [][]byte{[]byte("pb_data")}
			m.Deletion.Timestamp = col.Timestamp

			mmap[string(ks.Key)] = make(map[string][]*cassandra.Mutation)
			mmap[string(ks.Key)]["membership_queue"] = []*cassandra.Mutation{m}
		}
	}

	// Delete parting members.
	cp.ColumnFamily = "membership_dequeue"
	kss, ire, ue, te, err = db.GetRangeSlices(cp, pred, kr,
		cassandra.ConsistencyLevel_QUORUM)
	if ire != nil {
		log.Fatal("Invalid Cassandra request: ", ire.Why)
	}
	if ue != nil {
		log.Fatal("Cassandra unavailable")
	}
	if te != nil {
		log.Fatal("Cassandra timed out: ", te.String())
	}
	if err != nil {
		log.Fatal("Error getting range slice: ", err)
	}

	for _, ks = range kss {
		var csc *cassandra.ColumnOrSuperColumn
		for _, csc = range ks.Columns {
			var ldapuser string
			var col *cassandra.Column = csc.Column
			var agreement MembershipAgreement
			var attrs map[string][]string
			var m *cassandra.Mutation

			if col == nil {
				continue
			}

			if string(col.Name) != "pb_data" {
				log.Print("Column selected was not as requested: ",
					col.Name)
				continue
			}

			err = proto.Unmarshal(col.Value, &agreement)
			if err != nil {
				log.Print("Unable to parse column ", ks.Key, ": ", err)
				continue
			}

			ldapuser = "uid=" + agreement.MemberData.GetUsername() +
				"," + config.LdapConfig.GetNewUserSuffix() + "," +
				config.LdapConfig.GetBase()
			if noop {
				log.Print("Would remove user ", ldapuser)
			} else {
				var groups []string
				var groups_differ bool
				var entry openldap.LdapEntry
				var group string

				lres, err = ldap.SearchAll(
					config.LdapConfig.GetBase(),
					openldap.LDAP_SCOPE_SUB,
					"(&(objectClass=posixGroup)(memberUid="+
						agreement.MemberData.GetUsername()+"))",
					[]string{"cn"})
				if err != nil {
					log.Print("Error searching groups for user ",
						agreement.MemberData.GetUsername(), ": ", err)
					continue
				}

				for _, entry = range lres.Entries() {
					var cn string
					var found bool

					cn, err = entry.GetOneValueByName("cn")
					if err != nil {
						log.Fatal("Error determining groups for ",
							agreement.MemberData.GetUsername(), ": ",
							err)
					}
					groups = append(groups, cn)

					for _, group = range config.LdapConfig.GetNewUserGroup() {
						if cn == group {
							found = true
						}
					}

					if !found {
						groups_differ = true
					}
				}

				if groups_differ {
					log.Print("User is in other groups than expected: ",
						strings.Join(groups, ", "))

					attrs = make(map[string][]string)
					attrs["memberUid"] = []string{
						agreement.MemberData.GetUsername()}
					for _, group = range config.LdapConfig.GetNewUserGroup() {
						ldap.ModifyDel("cn="+group+",ou=Groups,"+
							config.LdapConfig.GetBase(), attrs)
					}
				} else {
					// The user appears to be only in the groups given in
					// the config.
					err = ldap.Delete(ldapuser)
					if err != nil {
						log.Print("Unable to delete user ", ldapuser, ": ",
							err)
					}
				}
			}

			m = cassandra.NewMutation()
			m.Deletion = cassandra.NewDeletion()
			m.Deletion.Predicate = pred
			m.Deletion.Timestamp = col.Timestamp

			mmap[string(ks.Key)] = make(map[string][]*cassandra.Mutation)
			mmap[string(ks.Key)]["membership_dequeue"] = []*cassandra.Mutation{m}

			// 2 years retention.
			col.Ttl = 720 * 24 * 3600
			col.Timestamp = now.UnixNano()

			m = cassandra.NewMutation()
			m.ColumnOrSupercolumn = cassandra.NewColumnOrSuperColumn()
			m.ColumnOrSupercolumn.Column = col
			mmap[string(ks.Key)]["membership_archive"] = []*cassandra.Mutation{m}
		}
	}

	// Apply all database mutations.
	ire, ue, te, err = db.BatchMutate(mmap, cassandra.ConsistencyLevel_QUORUM)
	if ire != nil {
		log.Fatal("Invalid Cassandra request: ", ire.Why)
	}
	if ue != nil {
		log.Fatal("Cassandra unavailable")
	}
	if te != nil {
		log.Fatal("Cassandra timed out: ", te.String())
	}
	if err != nil {
		log.Fatal("Error getting range slice: ", err)
	}

	log.Print("Greatest UID: ", greatestUid)
}
