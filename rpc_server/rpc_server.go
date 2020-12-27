package main

import (
	"crypto/tls"
	"flag"
	"io/ioutil"
	"log"
	"net"

	"github.com/golang/protobuf/proto"
	"github.com/starshipfactory/membersys"
	"github.com/starshipfactory/membersys/config"
	mdb "github.com/starshipfactory/membersys/db"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/reflection"
)

func main() {
	var config_file string
	var config_contents []byte
	var config_data config.MemberCreatorConfig

	var rpc_listen_address string
	var http_listen_address string
	var rpc_listener net.Listener
	var grpc_server *grpc.Server

	var certFile string
	var keyFile string
	var caFile string

	var db membersys.MembershipDB
	var end_user_service *EndUserService
	var err error

	flag.StringVar(&config_file, "config", "",
		"Path to the member creator configuration file")
	flag.StringVar(&rpc_listen_address, "listen-address", ":9090",
		"IP and port to bind the RPC server to")
	flag.StringVar(&http_listen_address, "status-address", ":8080",
		"IP and port to bind the HTTP status server to")

	flag.StringVar(&certFile, "cert", "", "Path to TLS certificate")
	flag.StringVar(&keyFile, "key", "", "Path to TLS private key")
	flag.StringVar(&caFile, "ca", "", "Path to TLS client CA certificate")
	flag.Parse()

	if len(config_file) == 0 {
		flag.Usage()
		return
	}

	config_contents, err = ioutil.ReadFile(config_file)
	if err != nil {
		log.Fatal("Unable to read ", config_file, ": ", err)
	}

	err = proto.Unmarshal(config_contents, &config_data)
	if err != nil {
		err = proto.UnmarshalText(string(config_contents), &config_data)
	}
	if err != nil {
		log.Fatal("Unable to parse ", config_file, ": ", err)
	}

	// Connect to database.
	db, err = mdb.New(config_data.DatabaseConfig)
	if err != nil {
		log.Fatal("Error connecting to database: ", err)
	}

	end_user_service = &EndUserService{database: db}

	if keyFile == "" || certFile == "" {
		grpc_server = grpc.NewServer()

		log.Print("WARNING: running RPC server in insecure mode. NEVER use " +
			"this mode with a production database!")
	} else if caFile == "" {
		var creds credentials.TransportCredentials
		creds, err = credentials.NewServerTLSFromFile(certFile, keyFile)
		if err != nil {
			log.Fatal("Error reading credentails from (",
				certFile, ", ", keyFile, ": ", err)
		}
		grpc_server = grpc.NewServer(grpc.Creds(creds))

		log.Print("WARNING: running RPC server without client " +
			"authentication. NEVER use this mode with a production database!")
	} else {
		var tlsConfig *tls.Config = &tls.Config{}
		var creds credentials.TransportCredentials

		creds = credentials.NewTLS(tlsConfig)
		grpc_server = grpc.NewServer(grpc.Creds(creds))
	}

	rpc_listener, err = net.Listen("tcp", rpc_listen_address)
	if err != nil {
		log.Fatal("Error listening to ", rpc_listen_address, ": ", err)
	}

	membersys.RegisterEndUserServer(grpc_server, end_user_service)
	reflection.Register(grpc_server)

	err = grpc_server.Serve(rpc_listener)
	if err != nil {
		log.Fatal("Error serving GRPC: ", err)
	}
}
