/*
 * (c) 2016, Tonnerre Lombard <tonnerre@ancient-solutions.com>,
 *       Starship Factory. All rights reserved.
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
	"github.com/golang/protobuf/proto"
	"github.com/starshipfactory/membersys"
	"io/ioutil"
	"log"
	"os"
	"time"
)

func main() {
	var db *membersys.MembershipDB
	var agreement *membersys.MembershipAgreement
	var config membersys.MemberCreatorConfig
	var wm *membersys.WelcomeMail
	var config_contents []byte
	var config_path string
	var lookup_key string
	var help bool
	var err error

	flag.BoolVar(&help, "help", false, "Display help")
	flag.StringVar(&config_path, "config", "",
		"Path to the member creator configuration file")
	flag.StringVar(&lookup_key, "key", "",
		"Key of the user record to look up")
	flag.Parse()

	if help || config_path == "" {
		flag.Usage()
		os.Exit(1)
	}

	config_contents, err = ioutil.ReadFile(config_path)
	if err != nil {
		log.Fatal("Unable to read ", config_path, ": ", err)
	}
	err = proto.Unmarshal(config_contents, &config)
	if err != nil {
		err = proto.UnmarshalText(string(config_contents), &config)
	}
	if err != nil {
		log.Fatal("Error parsing ", config_path, ": ", err)
	}

	db, err = membersys.NewMembershipDB(
		config.DatabaseConfig.GetDatabaseServer(),
		config.DatabaseConfig.GetDatabaseName(),
		time.Duration(config.DatabaseConfig.GetDatabaseTimeout())*time.Millisecond)
	if err != nil {
		log.Fatal("Unable to connect to the cassandra DB ",
			config.DatabaseConfig.GetDatabaseServer(), " at ",
			config.DatabaseConfig.GetDatabaseName(), ": ", err)
	}

	wm, err = membersys.NewWelcomeMail(config.GetWelcomeMailConfig())
	if err != nil {
		log.Fatal("Error setting up mailer: ", err)
	}

	agreement, err = db.GetMemberDetail(lookup_key)
	if err != nil {
		log.Fatal("Error fetching member ", lookup_key, ": ", err)
	}

	err = wm.SendMail(agreement.GetMemberData())
	if err != nil {
		log.Fatal("Error sending mail to ",
			agreement.GetMemberData().GetEmail(), ": ", err)
	}
}
