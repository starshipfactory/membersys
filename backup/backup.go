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

func handleErrors(errors <-chan error) {
	var err error

	for err = range errors {
		log.Fatal("Error: ", err)
	}

	log.Print("No errors detected")
}

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
	var memberWithKeyStream chan *membersys.MemberWithKey = make(chan *membersys.MemberWithKey)
	var member *membersys.Member
	var memberWithKey *membersys.MemberWithKey
	var memberAgreement *membersys.MembershipAgreementWithKey
	var errorStream chan error = make(chan error)

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
	if database == nil {
		log.Fatal("database = nil")
	}

	if verbose {
		log.Print("Database connection established")
	}

	ctx = context.Background()

	go database.StreamingEnumerateMembers(ctx, "", 0, memberStream, errorStream)
	go handleErrors(errorStream)

	for member = range memberStream {
		if member == nil {
			log.Print("Received nil member")
			continue
		}
		if verbose {
			log.Print("Backing up member ", member.GetName())
		}
		err = writer.WriteMessage(member)
		if err != nil {
			log.Fatal("Error writing record to members.pb: ", err)
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

	errorStream = make(chan error)
	go database.StreamingEnumerateMembershipRequests(
		ctx, "", "", 0, memberAgreementStream, errorStream)
	go handleErrors(errorStream)

	for memberAgreement = range memberAgreementStream {
		if memberAgreement == nil {
			log.Print("Received nil membership agreement")
		}
		if verbose {
			log.Print("Backing up membership request for ",
				memberAgreement.MemberData.GetName())
		}
		err = writer.WriteMessage(memberAgreement)
		if err != nil {
			log.Fatal("Error writing record to membership_requests.pb: ", err)
		}
	}

	err = out.Close()
	if err != nil {
		log.Fatal("Error closing membership_requests.pb: ", err)
	}

	// Back up all queued members
	out, err = os.Create("membership_queue.pb")
	if err != nil {
		log.Fatal("Error opening membership_queue.pb for writing: ", err)
	}
	writer = serialdata.NewSerialDataWriter(out)

	errorStream = make(chan error)
	go database.StreamingEnumerateQueuedMembers(
		ctx, "", 0, memberWithKeyStream, errorStream)
	go handleErrors(errorStream)

	for memberWithKey = range memberWithKeyStream {
		if memberWithKey == nil {
			log.Print("Received nil membership queue record")
		}
		if verbose {
			log.Print("Backing up membership queue record for ",
				memberWithKey.GetName())
		}
		err = writer.WriteMessage(memberWithKey)
		if err != nil {
			log.Fatal("Error writing record to membership_queue.pb: ", err)
		}
	}

	err = out.Close()
	if err != nil {
		log.Fatal("Error closing membership_queue.pb: ", err)
	}

	// Back up all de-queued members
	out, err = os.Create("membership_dequeue.pb")
	if err != nil {
		log.Fatal("Error opening membership_dequeue.pb for writing: ", err)
	}
	writer = serialdata.NewSerialDataWriter(out)

	errorStream = make(chan error)
	memberWithKeyStream = make(chan *membersys.MemberWithKey)
	go database.StreamingEnumerateDeQueuedMembers(
		ctx, "", 0, memberWithKeyStream, errorStream)
	go handleErrors(errorStream)

	for memberWithKey = range memberWithKeyStream {
		if memberWithKey == nil {
			log.Print("Received nil membership de-queue record")
		}
		if verbose {
			log.Print("Backing up membership de-queue record for ",
				memberWithKey.GetName())
		}
		err = writer.WriteMessage(memberWithKey)
		if err != nil {
			log.Fatal("Error writing record to membership_dequeue.pb: ", err)
		}
	}

	err = out.Close()
	if err != nil {
		log.Fatal("Error closing membership_dequeue.pb: ", err)
	}

	// Back up all arcguved members
	out, err = os.Create("membership_archive.pb")
	if err != nil {
		log.Fatal("Error opening membership_archive.pb for writing: ", err)
	}
	writer = serialdata.NewSerialDataWriter(out)

	errorStream = make(chan error)
	memberWithKeyStream = make(chan *membersys.MemberWithKey)
	go database.StreamingEnumerateTrashedMembers(
		ctx, "", 0, memberWithKeyStream, errorStream)
	go handleErrors(errorStream)

	for memberWithKey = range memberWithKeyStream {
		if memberWithKey == nil {
			log.Print("Received nil membership archive record")
		}
		if verbose {
			log.Print("Backing up membership archive record for ",
				memberWithKey.GetName())
		}
		err = writer.WriteMessage(memberWithKey)
		if err != nil {
			log.Fatal("Error writing record to membership_archive.pb: ", err)
		}
	}

	err = out.Close()
	if err != nil {
		log.Fatal("Error closing membership_archive.pb: ", err)
	}
}
