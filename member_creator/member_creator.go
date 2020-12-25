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
	"context"
	"crypto/tls"
	"crypto/x509"
	"flag"
	"io/ioutil"
	"log"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/starshipfactory/membersys"
	"github.com/starshipfactory/membersys/config"
	mdb "github.com/starshipfactory/membersys/db"
	"gopkg.in/ldap.v2"
)

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
	var config_file string
	var config_contents []byte
	var config config.MemberCreatorConfig
	var greatestUid uint64 = 1000
	var noop, verbose bool
	var welcome *membersys.WelcomeMail

	var ld *ldap.Conn
	var sreq *ldap.SearchRequest
	var lres *ldap.SearchResult
	var entry *ldap.Entry
	var tlsconfig tls.Config

	var db membersys.MembershipDB
	var batchOpTimeout time.Duration

	var requests []*membersys.MemberWithKey
	var request *membersys.MemberWithKey

	var ctx context.Context
	var cancel context.CancelFunc

	var err error

	flag.StringVar(&config_file, "config", "",
		"Path to the member creator configuration file")
	flag.BoolVar(&noop, "dry-run", false, "Do a dry run")
	flag.BoolVar(&verbose, "verbose", false,
		"Whether or not to display verbose messages")
	flag.DurationVar(&batchOpTimeout, "batch-op-timeout",
		5*time.Minute, "Timeout for batch operations")
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
	db, err = mdb.NewCassandraDB(
		config.DatabaseConfig.GetDatabaseServer(),
		config.DatabaseConfig.GetDatabaseName(),
		time.Duration(config.DatabaseConfig.GetDatabaseTimeout())*time.Millisecond)
	if err != nil {
		log.Fatal("Error connecting to Cassandra database at ",
			config.DatabaseConfig.GetDatabaseServer(), ": ", err)
	}

	ctx, cancel = context.WithTimeout(context.Background(), batchOpTimeout)
	requests, err = db.EnumerateQueuedMembers(ctx, "", 0)
	cancel()
	if err != nil {
		log.Fatal("Error listing membership requests: ", err)
	}

	for _, request = range requests {
		if request.Username != nil {
			var attrs *ldap.AddRequest

			greatestUid++

			attrs = ldap.NewAddRequest("uid=" +
				asciiFilter(request.GetUsername()) + "," +
				config.LdapConfig.GetNewUserSuffix() + "," +
				config.LdapConfig.GetBase())

			attrs.Attribute("uidNumber", []string{
				strconv.FormatUint(greatestUid, 10)})
			attrs.Attribute("gecos", []string{
				asciiFilter(request.GetName())})
			attrs.Attribute("shadowLastChange", []string{"11457"})
			attrs.Attribute("shadowMax", []string{"9999"})
			attrs.Attribute("shadowWarning", []string{"7"})
			attrs.Attribute("gidNumber", []string{strconv.FormatUint(
				uint64(config.LdapConfig.GetNewUserGid()), 10)})
			attrs.Attribute("objectClass", []string{
				"account", "posixAccount", "shadowAccount", "top",
			})
			attrs.Attribute("uid", []string{
				asciiFilter(request.GetUsername())})
			attrs.Attribute("cn", []string{
				asciiFilter(request.GetUsername())})
			attrs.Attribute("homeDirectory", []string{"/home/" +
				asciiFilter(request.GetUsername())})
			attrs.Attribute("loginShell", []string{
				config.LdapConfig.GetNewUserShell()})
			attrs.Attribute("userPassword", []string{
				request.GetPwhash(),
			})

			request.Id = proto.Uint64(greatestUid)
			if verbose {
				log.Print("Creating user: uid=" +
					request.GetUsername() +
					"," + config.LdapConfig.GetNewUserSuffix() + "," +
					config.LdapConfig.GetBase())
			}

			if !noop {
				var group string

				err = ld.Add(attrs)
				if err != nil {
					log.Print("Unable to create LDAP Account ",
						request.GetUsername(), ": ", err)
					continue
				}

				for _, group = range config.LdapConfig.GetNewUserGroup() {
					var grpadd = ldap.NewModifyRequest("cn=" + group +
						",ou=Groups," + config.LdapConfig.GetBase())

					grpadd.Add("memberUid", []string{
						request.GetUsername()})

					err = ld.Modify(grpadd)
					if err != nil {
						log.Print("Unable to add user ",
							request.GetUsername(),
							" to group ", group, ": ",
							err)
					}
				}
			}
		}

		ctx, cancel = context.WithTimeout(context.Background(),
			batchOpTimeout)
		err = db.MoveNewMemberToFullMember(ctx, request)
		cancel()
		if err != nil {
			log.Print("Error moving member to full membership: ", err)
			continue
		}

		// Write welcome e-mail to new member.
		if welcome != nil {
			err = welcome.SendMail(&request.Member)
			if err != nil {
				log.Print("Error sending welcome e-mail to ",
					request.GetEmail(), ": ", err)
			}
		}
	}

	// Delete parting members.
	ctx, cancel = context.WithTimeout(context.Background(), batchOpTimeout)
	requests, err = db.EnumerateDeQueuedMembers(ctx, "", 0)
	cancel()
	if err != nil {
		log.Fatal("Error getting range slice: ", err)
	}

	for _, request = range requests {
		var ldapuser string
		var attrs *ldap.ModifyRequest

		ldapuser = "uid=" +
			asciiFilter(request.GetUsername()) + "," +
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
					asciiFilter(request.GetUsername())+"))",
				[]string{"cn"}, []ldap.Control{})

			lres, err = ld.Search(sreq)
			if err != nil {
				log.Print("Error searching groups for user ",
					request.GetUsername(), ": ", err)
				continue
			}

			for _, entry = range lres.Entries {
				var cn string
				var found bool

				cn = entry.GetAttributeValue("cn")
				if cn == "" {
					log.Print("No group name set for ", request.GetUsername())
					continue
				}
				groups = append(groups, cn)

				for _, group = range config.LdapConfig.GetNewUserGroup() {
					if cn == group {
						found = true
					}
				}

				for _, group = range config.LdapConfig.GetIgnoreUserGroup() {
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
						asciiFilter(request.GetUsername())})
					err = ld.Modify(attrs)
					if err != nil {
						log.Print("Error deleting ",
							request.GetUsername(), " from ", group, ": ",
							err)
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

		ctx, cancel = context.WithTimeout(context.Background(),
			batchOpTimeout)
		err = db.MoveDeletedMemberToArchive(ctx, request)
		cancel()
		if err != nil {
			log.Print("Error moving deleted member to archive: ", err)
			continue
		}
	}

	if verbose {
		log.Print("Greatest UID: ", greatestUid)
	}
}
