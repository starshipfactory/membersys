/*
 * (c) 2014, Caoimhe Chaos <caoimhechaos@protonmail.com>,
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
	"crypto/tls"
	"crypto/x509"
	"database/cassandra"
	"encoding/binary"
	"flag"
	"io/ioutil"
	"log"
	"net"
	"strconv"
	"strings"
	"time"

	"code.google.com/p/goprotobuf/proto"
	"github.com/starshipfactory/membersys"
	"gopkg.in/ldap.v2"
)

func makeMutation(mmap map[string][]*cassandra.Mutation, cf, name string,
	value []byte, now time.Time) {
	var m *cassandra.Mutation = cassandra.NewMutation()
	var col *cassandra.Column = cassandra.NewColumn()

	col.Name = []byte(name)
	col.Value = value
	col.Timestamp = proto.Int64(now.UnixNano())

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
	var config membersys.MemberCreatorConfig
	var greatestUid uint64 = 1000
	var now time.Time
	var noop, verbose bool
	var welcome *membersys.WelcomeMail

	var ld *ldap.Conn
	var sreq *ldap.SearchRequest
	var lres *ldap.SearchResult
	var entry *ldap.Entry
	var tlsconfig tls.Config

	var mmap map[string]map[string][]*cassandra.Mutation
	var db *cassandra.RetryCassandraClient
	var cp *cassandra.ColumnParent
	var pred *cassandra.SlicePredicate
	var kr *cassandra.KeyRange
	var kss []*cassandra.KeySlice
	var ks *cassandra.KeySlice

	var err error

	flag.StringVar(&config_file, "config", "",
		"Path to the member creator configuration file")
	flag.BoolVar(&noop, "dry-run", false, "Do a dry run")
	flag.BoolVar(&verbose, "verbose", false,
		"Whether or not to display verbose messages")
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
	if config.WelcomeMailConfig != nil {
		welcome, err = membersys.NewWelcomeMail(
			config.WelcomeMailConfig)
		if err != nil {
			log.Fatal("Error creating WelcomeMail: ", err)
		}
	}

	tlsconfig.MinVersion = tls.VersionTLS12
	tlsconfig.ServerName, _, err = net.SplitHostPort(
		config.LdapConfig.GetServer())
	if err != nil {
		log.Fatal("Can't split ", config.LdapConfig.GetServer(),
			" into host and port: ", err)
	}

	if config.LdapConfig.CaCertificate != nil {
		var certData []byte

		certData, err = ioutil.ReadFile(config.LdapConfig.GetCaCertificate())
		if err != nil {
			log.Fatal("Unable to read certificate from ",
				config.LdapConfig.GetCaCertificate(), ": ", err)
		}

		tlsconfig.RootCAs = x509.NewCertPool()
		tlsconfig.RootCAs.AppendCertsFromPEM(certData)
	}

	now = time.Now()

	if !noop {
		ld, err = ldap.DialTLS("tcp", config.LdapConfig.GetServer(),
			&tlsconfig)
		if err != nil {
			log.Fatal("Error connecting to LDAP server ",
				config.LdapConfig.GetServer(), ": ", err)
		}

		err = ld.Bind(config.LdapConfig.GetSuperUser()+","+
			config.LdapConfig.GetBase(), config.LdapConfig.GetSuperPassword())
		if err != nil {
			log.Fatal("Unable to bind as ", config.LdapConfig.GetSuperUser()+
				","+config.LdapConfig.GetBase(), " to ",
				config.LdapConfig.GetServer(), ": ", err)
		}
		defer ld.Close()

		sreq = ldap.NewSearchRequest(
			config.LdapConfig.GetBase(), ldap.ScopeWholeSubtree,
			ldap.NeverDerefAliases, 1000, 90, false,
			"(objectClass=posixAccount)", []string{"uidNumber"},
			[]ldap.Control{})

		// Find the highest assigned UID.
		lres, err = ld.Search(sreq)
		if err != nil {
			log.Fatal("Unable to search for posix accounts in ",
				config.LdapConfig.GetBase(), ": ", err)
		}
		for _, entry = range lres.Entries {
			var uid string

			for _, uid = range entry.GetAttributeValues("uidNumber") {
				var uidNumber uint64
				uidNumber, err = strconv.ParseUint(uid, 10, 64)
				if err != nil {
					log.Print("Error parsing \"", uid, "\" as a number: ",
						err)
				} else if uidNumber > greatestUid {
					greatestUid = uidNumber
				}
			}
		}
	}

	// Connect to Cassandra so we can get a list of records to be processed.
	db, err = cassandra.NewRetryCassandraClient(
		config.DatabaseConfig.GetDatabaseServer())
	if err != nil {
		log.Fatal("Error connecting to Cassandra database at ",
			config.DatabaseConfig.GetDatabaseServer(), ": ", err)
	}

	err = db.SetKeyspace(config.DatabaseConfig.GetDatabaseName())
	if err != nil {
		log.Fatal("Error setting keyspace: ", err)
	}

	cp = cassandra.NewColumnParent()
	cp.ColumnFamily = "membership_queue"
	pred = cassandra.NewSlicePredicate()
	pred.ColumnNames = [][]byte{[]byte("pb_data")}
	kr = cassandra.NewKeyRange()
	kr.StartKey = []byte("queue:")
	kr.EndKey = []byte("queue;")

	kss, err = db.GetRangeSlices(cp, pred, kr,
		cassandra.ConsistencyLevel_QUORUM)
	if err != nil {
		log.Fatal("Error getting range slice: ", err)
	}

	for _, ks = range kss {
		mmap = make(map[string]map[string][]*cassandra.Mutation)
		var csc *cassandra.ColumnOrSuperColumn
		for _, csc = range ks.Columns {
			var col *cassandra.Column = csc.Column
			var agreement membersys.MembershipAgreement
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
				log.Print("Unable to parse column ", col.Name, " of ",
					ks.Key, ": ", err)
				continue
			}

			if agreement.MemberData.Username != nil {
				var attrs *ldap.AddRequest

				greatestUid++

				attrs = ldap.NewAddRequest("uid=" +
					asciiFilter(agreement.MemberData.GetUsername()) + "," +
					config.LdapConfig.GetNewUserSuffix() + "," +
					config.LdapConfig.GetBase())

				attrs.Attribute("uidNumber", []string{
					strconv.FormatUint(greatestUid, 10)})
				attrs.Attribute("gecos", []string{
					asciiFilter(agreement.MemberData.GetName())})
				attrs.Attribute("shadowLastChange", []string{"11457"})
				attrs.Attribute("shadowMax", []string{"9999"})
				attrs.Attribute("shadowWarning", []string{"7"})
				attrs.Attribute("gidNumber", []string{strconv.FormatUint(
					uint64(config.LdapConfig.GetNewUserGid()), 10)})
				attrs.Attribute("objectClass", []string{
					"account", "posixAccount", "shadowAccount", "top",
				})
				attrs.Attribute("uid", []string{
					asciiFilter(agreement.MemberData.GetUsername())})
				attrs.Attribute("cn", []string{
					asciiFilter(agreement.MemberData.GetUsername())})
				attrs.Attribute("homeDirectory", []string{"/home/" +
					asciiFilter(agreement.MemberData.GetUsername())})
				attrs.Attribute("loginShell", []string{
					config.LdapConfig.GetNewUserShell()})
				attrs.Attribute("userPassword", []string{
					agreement.MemberData.GetPwhash(),
				})

				agreement.MemberData.Id = proto.Uint64(greatestUid)
				if verbose {
					log.Print("Creating user: uid=" +
						agreement.MemberData.GetUsername() +
						"," + config.LdapConfig.GetNewUserSuffix() + "," +
						config.LdapConfig.GetBase())
				}

				if !noop {
					var group string

					err = ld.Add(attrs)
					if err != nil {
						log.Print("Unable to create LDAP Account ",
							agreement.MemberData.GetUsername(), ": ", err)
						continue
					}

					for _, group = range config.LdapConfig.GetNewUserGroup() {
						var grpadd = ldap.NewModifyRequest("cn=" + group +
							",ou=Groups," + config.LdapConfig.GetBase())

						grpadd.Add("memberUid", []string{
							agreement.MemberData.GetUsername()})

						err = ld.Modify(grpadd)
						if err != nil {
							log.Print("Unable to add user ",
								agreement.MemberData.GetUsername(),
								" to group ", group, ": ",
								err)
						}
					}
				}
			}

			col.Value, err = proto.Marshal(&agreement)
			if err != nil {
				log.Print("Error marshalling agreement: ", err)
				continue
			}

			mmap["member:"+string(agreement.MemberData.GetEmail())] =
				make(map[string][]*cassandra.Mutation)
			mmap["member:"+string(agreement.MemberData.GetEmail())][cf] =
				make([]*cassandra.Mutation, 0)

			makeMutation(mmap["member:"+string(agreement.MemberData.GetEmail())],
				cf, "pb_data", col.Value, now)
			makeMutationString(mmap["member:"+string(agreement.MemberData.GetEmail())],
				cf, "name", agreement.MemberData.GetName(), now)
			makeMutationString(mmap["member:"+string(agreement.MemberData.GetEmail())],
				cf, "street", agreement.MemberData.GetStreet(), now)
			makeMutationString(mmap["member:"+string(agreement.MemberData.GetEmail())],
				cf, "city", agreement.MemberData.GetCity(), now)
			makeMutationString(mmap["member:"+string(agreement.MemberData.GetEmail())],
				cf, "country", agreement.MemberData.GetCountry(), now)
			makeMutationString(mmap["member:"+string(agreement.MemberData.GetEmail())],
				cf, "email", agreement.MemberData.GetEmail(), now)
			if agreement.MemberData.Phone != nil {
				makeMutationString(mmap["member:"+string(agreement.MemberData.GetEmail())],
					cf, "phone", agreement.MemberData.GetPhone(), now)
			}
			if agreement.MemberData.Username != nil {
				makeMutationString(mmap["member:"+string(agreement.MemberData.GetEmail())],
					cf, "username", agreement.MemberData.GetUsername(), now)
			}
			makeMutationLong(mmap["member:"+string(agreement.MemberData.GetEmail())],
				cf, "fee", agreement.MemberData.GetFee(), now)
			makeMutationBool(mmap["member:"+string(agreement.MemberData.GetEmail())],
				cf, "fee_yearly", agreement.MemberData.GetFeeYearly(), now)
			makeMutationLong(mmap["member:"+string(agreement.MemberData.GetEmail())],
				cf, "approval_ts", agreement.Metadata.GetApprovalTimestamp(),
				now)
			makeMutation(mmap["member:"+string(agreement.MemberData.GetEmail())],
				cf, "agreement_pdf", agreement.AgreementPdf, now)

			// Now, delete the original record.
			m = cassandra.NewMutation()
			m.Deletion = cassandra.NewDeletion()
			m.Deletion.Predicate = cassandra.NewSlicePredicate()
			m.Deletion.Predicate.ColumnNames = [][]byte{[]byte("pb_data")}
			m.Deletion.Timestamp = col.Timestamp

			mmap[string(ks.Key)] = make(map[string][]*cassandra.Mutation)
			mmap[string(ks.Key)]["membership_queue"] = []*cassandra.Mutation{m}

			// Write welcome e-mail to new member.
			if welcome != nil {
				err = welcome.SendMail(agreement.MemberData)
				if err != nil {
					log.Print("Error sending welcome e-mail to ",
						agreement.MemberData.GetEmail(), ": ", err)
				}
			}
		}

		// Apply all database mutations.
		err = db.BatchMutate(mmap, cassandra.ConsistencyLevel_QUORUM)
		if err != nil {
			log.Fatal("Error getting range slice: ", err)
		}
	}

	// Delete parting members.
	cp.ColumnFamily = "membership_dequeue"
	kr.StartKey = []byte("dequeue:")
	kr.EndKey = []byte("dequeue;")
	kss, err = db.GetRangeSlices(cp, pred, kr,
		cassandra.ConsistencyLevel_QUORUM)
	if err != nil {
		log.Fatal("Error getting range slice: ", err)
	}

	for _, ks = range kss {
		var csc *cassandra.ColumnOrSuperColumn
		mmap = make(map[string]map[string][]*cassandra.Mutation)
		for _, csc = range ks.Columns {
			var uuid cassandra.UUID
			var ldapuser string
			var col *cassandra.Column = csc.Column
			var agreement membersys.MembershipAgreement
			var attrs *ldap.ModifyRequest
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

			if agreement.MemberData.Username == nil {
				continue
			}

			ldapuser = "uid=" +
				asciiFilter(agreement.MemberData.GetUsername()) + "," +
				config.LdapConfig.GetNewUserSuffix() + "," +
				config.LdapConfig.GetBase()
			if noop {
				log.Print("Would remove user ", ldapuser)
			} else {
				var groups []string
				var groups_differ bool
				var entry *ldap.Entry
				var group string

				sreq = ldap.NewSearchRequest(
					config.LdapConfig.GetBase(), ldap.ScopeWholeSubtree,
					ldap.NeverDerefAliases, 1000, 90, false,
					"(&(objectClass=posixGroup)(memberUid="+
						asciiFilter(agreement.MemberData.GetUsername())+"))",
					[]string{"cn"}, []ldap.Control{})

				lres, err = ld.Search(sreq)
				if err != nil {
					log.Print("Error searching groups for user ",
						agreement.MemberData.GetUsername(), ": ", err)
					continue
				}

				for _, entry = range lres.Entries {
					var cn string
					var found bool

					cn = entry.GetAttributeValue("cn")
					if cn == "" {
						log.Print("No group name set for ",
							agreement.MemberData.GetUsername())
						continue
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

					for _, group = range config.LdapConfig.GetNewUserGroup() {
						attrs = ldap.NewModifyRequest("cn=" + group +
							",ou=Groups," + config.LdapConfig.GetBase())
						attrs.Delete("memberUid", []string{
							asciiFilter(agreement.MemberData.GetUsername())})
						err = ld.Modify(attrs)
						if err != nil {
							log.Print("Error deleting ",
								agreement.MemberData.GetUsername(), " from ",
								group, ": ", err)
						}
					}
				} else {
					var dr = ldap.NewDelRequest(ldapuser, []ldap.Control{})
					// The user appears to be only in the groups given in
					// the config.
					err = ld.Del(dr)
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
			col.Ttl = proto.Int32(720 * 24 * 3600)
			col.Timestamp = proto.Int64(now.UnixNano())

			uuid = cassandra.UUIDFromBytes(ks.Key[len("dequeue:"):])

			m = cassandra.NewMutation()
			m.ColumnOrSupercolumn = cassandra.NewColumnOrSuperColumn()
			m.ColumnOrSupercolumn.Column = col
			mmap["archive:"+string([]byte(uuid))] = make(map[string][]*cassandra.Mutation)
			mmap["archive:"+string([]byte(uuid))]["membership_archive"] =
				[]*cassandra.Mutation{m}
		}

		// Apply all database mutations.
		err = db.BatchMutate(mmap, cassandra.ConsistencyLevel_QUORUM)
		if err != nil {
			log.Fatal("Error getting range slice: ", err)
		}
	}

	if verbose {
		log.Print("Greatest UID: ", greatestUid)
	}
}
