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
	"fmt"
	"github.com/golang/protobuf/proto"
	"github.com/starshipfactory/membersys"
	"io/ioutil"
	"log"
	"os"
	"time"
)

func main() {
	var db *membersys.MembershipDB
	var config membersys.MembersysConfig
	var config_contents []byte
	var config_path string
	var prev_key string
	var help bool
	var err error

	flag.BoolVar(&help, "help", false, "Display help")
	flag.StringVar(&config_path, "config", "",
		"Path to the member creator configuration file")
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

	for {
		var members []*membersys.Member
		var member *membersys.Member

		members, err = db.EnumerateMembers(prev_key, 25)

		if err != nil {
			log.Fatal("Error fetching data starting from ", prev_key, ": ",
				err)
		}
		if len(members) == 0 {
			break
		}

		for _, member = range members {
			fmt.Printf("Name:\t\t%s\r\nAddress:\t%s, %s\r\nEmail:\t\t%s\r\n"+
				"Username:\t%s\r\n\r\n",
				member.GetName(), member.GetStreet(), member.GetCity(),
				member.GetEmail(), member.GetUsername())
			prev_key = member.GetEmail() + "\000"
		}
	}
}
