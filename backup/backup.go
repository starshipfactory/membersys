package main

import (
	"context"
	"flag"
	"io/ioutil"
	"log"
	"os"

	"github.com/caoimhechaos/go-serialdata"
	"github.com/golang/protobuf/proto"
	"github.com/starshipfactory/membersys"
	"github.com/starshipfactory/membersys/config"
	"github.com/starshipfactory/membersys/db"
)

func main() {
	var ctx context.Context
	var configData config.DatabaseConfig
	var configContents []byte
	var configPath string
	var chdirPath string
	var database membersys.MembershipDB
	var verbose bool

	var memberAgreementStream chan *membersys.MembershipAgreementWithKey = make(chan *membersys.MembershipAgreementWithKey)
	var memberStream chan *membersys.Member = make(chan *membersys.Member)
	var errorStream chan error = make(chan error)
	var moreData bool

	var out *os.File
	var writer *serialdata.SerialDataWriter
	var err error

	flag.StringVar(&configPath, "config", "",
		"Path to a configuration file for the backup tool.")
	flag.StringVar(&chdirPath, "chdir", "",
		"Path to change directory to before backup.")
	flag.BoolVar(&verbose, "verbose", false,
		"Verbosely display backup progress.")
	flag.Parse()

	if len(configPath) == 0 {
		flag.Usage()
		return
	}

	if chdirPath != "" {
		err = os.Chdir(chdirPath)
		if err != nil {
			log.Fatal("Unable to change directory to ", chdirPath,
				": ", err)
		}
	}

	configContents, err = ioutil.ReadFile(configPath)
	if err != nil {
		log.Fatal("Unable to read ", configPath, ": ", err)
	}

	if verbose {
		log.Print("Read ", len(configContents), " bytes from config")
	}

	err = proto.Unmarshal(configContents, &configData)
	if err != nil {
		err = proto.UnmarshalText(string(configContents), &configData)
	}
	if err != nil {
		log.Fatal("Unable to parse ", configPath, ": ", err)
	}

	// Back up all members.
	out, err = os.Create("members.pb")
	if err != nil {
		log.Fatal("Error opening members.pb for writing: ", err)
	}
	writer = serialdata.NewSerialDataWriter(out)

	database, err = db.New(&configData)
	if err != nil {
		log.Fatal("Error connecting to database: ", err)
	}

	ctx = context.Background()

	go database.StreamingEnumerateMembers(ctx, "", 0, memberStream, errorStream)

	moreData = true
	for moreData {
		var member *membersys.Member
		select {
		case member = <-memberStream:
			if verbose {
				log.Print("Backing up member ", member.GetName())
			}
			err = writer.WriteMessage(member)
			if err != nil {
				log.Fatal("Error writing record to members.pb: ", err)
			}
		case err = <-errorStream:
			log.Fatal("Error enumerating members: ", err)
		default:
			log.Print("All members backed up.")
			moreData = false
			break
		}
	}

	err = out.Close()
	if err != nil {
		log.Fatal("Error closing members.pb: ", err)
	}

	// Back up all membership requests
	out, err = os.Create("membership_requests.pb")
	if err != nil {
		log.Fatal("Error opening membership_requests.pb for writing: ", err)
	}
	writer = serialdata.NewSerialDataWriter(out)

	go database.StreamingEnumerateMembershipRequests(
		ctx, "", "", 0, memberAgreementStream, errorStream)

	moreData = true
	for moreData {
		var memberAgreement *membersys.MembershipAgreementWithKey
		select {
		case memberAgreement = <-memberAgreementStream:
			if verbose {
				log.Print("Backing up membership request for ",
					memberAgreement.MemberData.GetName())
			}
			err = writer.WriteMessage(memberAgreement)
			if err != nil {
				log.Fatal("Error writing record to membership_requests.pb: ", err)
			}

		case err = <-errorStream:
			log.Fatal("Error enumerating membership agreements: ", err)
		default:
			log.Print("All membership agreements backed up.")
			moreData = false
			break
		}
	}

	err = out.Close()
	if err != nil {
		log.Fatal("Error closing membership_requests.pb: ", err)
	}
}
