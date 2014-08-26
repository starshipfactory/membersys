/*
 * (c) 2013, Caoimhe Chaos <caoimhechaos@protonmail.com>,
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
	"flag"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	"ancient-solutions.com/ancientauth"
	"ancient-solutions.com/doozer/exportedservice"
	"code.google.com/p/goprotobuf/proto"
)

func main() {
	var help bool
	var bindto, config_file string
	var config_contents []byte
	var application_tmpl, memberlist_tmpl, print_tmpl *template.Template
	var exporter *exportedservice.ServiceExporter
	var authenticator *ancientauth.Authenticator
	var debug_authenticator bool
	var config MembersysConfig
	var db *MembershipDB
	var err error

	flag.BoolVar(&help, "help", false, "Display help")
	flag.StringVar(&bindto, "bind", "127.0.0.1:8080",
		"The address to bind the web server to")
	flag.StringVar(&config_file, "config",
		"", "Path to a file containing a MembersysConfig protocol buffer")
	flag.BoolVar(&debug_authenticator, "debug-authenticator", false,
		"Debug the authenticator?")
	flag.Parse()

	if help || config_file == "" {
		flag.Usage()
		os.Exit(1)
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
		log.Fatal("Error parsing ", config_file, ": ", err)
	}

	// Load and parse the HTML templates to be displayed.
	application_tmpl, err = template.ParseFiles(
		config.GetTemplateDir() + "/form.html")
	if err != nil {
		log.Fatal("Unable to parse form template: ", err)
	}

	print_tmpl, err = template.ParseFiles(
		config.GetTemplateDir() + "/printlayout.html")
	if err != nil {
		log.Fatal("Unable to parse print layout template: ", err)
	}

	memberlist_tmpl = template.New("memberlist")
	memberlist_tmpl.Funcs(fmap)
	memberlist_tmpl, err = memberlist_tmpl.ParseFiles(
		config.GetTemplateDir() + "/memberlist.html")
	if err != nil {
		log.Fatal("Unable to parse member list template: ", err)
	}

	authenticator, err = ancientauth.NewAuthenticator(
		config.AuthenticationConfig.GetAppName(),
		config.AuthenticationConfig.GetCertPath(),
		config.AuthenticationConfig.GetKeyPath(),
		config.AuthenticationConfig.GetCaBundlePath(),
		config.AuthenticationConfig.GetAuthServerHost(),
		config.AuthenticationConfig.GetX509KeyserverHost(),
		int(config.AuthenticationConfig.GetX509CertificateCacheSize()))
	if err != nil {
		log.Fatal("Unable to assemble authenticator: ", err)
	}

	if debug_authenticator {
		authenticator.Debug()
	}

	db, err = NewMembershipDB(config.DatabaseConfig.GetDatabaseServer(),
		config.DatabaseConfig.GetDatabaseName(),
		time.Duration(config.DatabaseConfig.GetDatabaseTimeout())*time.Millisecond)
	if err != nil {
		log.Fatal("Unable to connect to the cassandra DB ",
			config.DatabaseConfig.GetDatabaseServer(), " at ",
			config.DatabaseConfig.GetDatabaseName(), ": ", err)
	}

	// Register the URL handlers to be invoked.
	http.Handle("/admin/api/members", &MemberListHandler{
		admingroup: config.AuthenticationConfig.GetAuthGroup(),
		auth:       authenticator,
		database:   db,
		pagesize:   config.GetResultPageSize(),
	})

	http.Handle("/admin/api/applicants", &ApplicantListHandler{
		admingroup: config.AuthenticationConfig.GetAuthGroup(),
		auth:       authenticator,
		database:   db,
		pagesize:   config.GetResultPageSize(),
	})

	http.Handle("/admin/api/queue", &MemberQueueListHandler{
		admingroup: config.AuthenticationConfig.GetAuthGroup(),
		auth:       authenticator,
		database:   db,
		pagesize:   config.GetResultPageSize(),
	})

	http.Handle("/admin/api/dequeue", &MemberDeQueueListHandler{
		admingroup: config.AuthenticationConfig.GetAuthGroup(),
		auth:       authenticator,
		database:   db,
		pagesize:   config.GetResultPageSize(),
	})

	http.Handle("/admin/api/trash", &MemberTrashListHandler{
		admingroup: config.AuthenticationConfig.GetAuthGroup(),
		auth:       authenticator,
		database:   db,
		pagesize:   config.GetResultPageSize(),
	})

	http.Handle("/admin/api/accept", &MemberAcceptHandler{
		admingroup: config.AuthenticationConfig.GetAuthGroup(),
		auth:       authenticator,
		database:   db,
	})

	http.Handle("/admin/api/reject", &MemberRejectHandler{
		admingroup: config.AuthenticationConfig.GetAuthGroup(),
		auth:       authenticator,
		database:   db,
	})

	http.Handle("/admin/api/editfee", &MemberFeeHandler{
		admingroup: config.AuthenticationConfig.GetAuthGroup(),
		auth:       authenticator,
		database:   db,
	})

	http.Handle("/admin/api/agreement-upload", &MemberAgreementUploadHandler{
		admingroup: config.AuthenticationConfig.GetAuthGroup(),
		auth:       authenticator,
		database:   db,
	})

	http.Handle("/admin/api/cancel-queued", &MemberQueueCancelHandler{
		admingroup: config.AuthenticationConfig.GetAuthGroup(),
		auth:       authenticator,
		database:   db,
	})

	http.Handle("/admin/api/goodbye-member", &MemberGoodbyeHandler{
		admingroup: config.AuthenticationConfig.GetAuthGroup(),
		auth:       authenticator,
		database:   db,
	})

	http.Handle("/admin/api/member", &MemberDetailHandler{
		admingroup: config.AuthenticationConfig.GetAuthGroup(),
		auth:       authenticator,
		database:   db,
	})

	http.Handle("/admin", &TotalListHandler{
		admingroup: config.AuthenticationConfig.GetAuthGroup(),
		auth:       authenticator,
		database:   db,
		pagesize:   config.GetResultPageSize(),
		template:   memberlist_tmpl,
	})

	http.Handle("/", &FormInputHandler{
		applicationTmpl: application_tmpl,
		database:        db,
		passthrough:     http.FileServer(http.Dir(config.GetTemplateDir())),
		printTmpl:       print_tmpl,
		useProxyRealIP:  config.GetUseProxyRealIp(),
	})

	// If a lock server was specified, attempt to use an anonymous port as
	// a Doozer exported HTTP service. Otherwise, just bind to the address
	// given in bindto, for debugging etc.
	if config.LockserviceConfig != nil {
		exporter, err = exportedservice.NewExporter(
			config.LockserviceConfig.GetLockserverUri(),
			config.LockserviceConfig.GetLockserverBootUri())
		if err != nil {
			log.Fatal("doozer.DialUri ",
				config.LockserviceConfig.GetLockserverUri(), " (",
				config.LockserviceConfig.GetLockserverBootUri(), "): ", err)
		}

		defer exporter.UnexportPort()
		err = exporter.ListenAndServeNamedHTTP(
			config.LockserviceConfig.GetDoozerServiceName(), bindto, nil)
		if err != nil {
			log.Fatal("ListenAndServe: ", err)
		}
	} else {
		err = http.ListenAndServe(bindto, nil)
		if err != nil {
			log.Fatal("ListenAndServe: ", err)
		}
	}
}
