package main

import (
	cbdc "app/api"
	"context"
	"fmt"
	"google.golang.org/grpc"
	"log"
	"net"
	"os"
	"time"

	"github.com/hyperledger/fabric-gateway/pkg/client"
	"github.com/hyperledger/fabric-gateway/pkg/hash"
)

const (
	mspID           = "RBIMSP"
	cryptoPath      = "../network/organizations/peerOrganizations/rbi.cbdc"
	certPath        = cryptoPath + "/users/User1@rbi.cbdc/msp/signcerts"
	keyPath         = cryptoPath + "/users/User1@rbi.cbdc/msp/keystore"
	tlsCertPath     = cryptoPath + "/peers/peer0.rbi.cbdc/tls/ca.crt"
	peerEndpoint    = "dns:///localhost:7051"
	gatewayPeer     = "peer0.rbi.cbdc"
	ApplicationPort = 7999
)

type server struct {
	cbdc.CBDCServer
}

var Contract *client.Contract

var now = time.Now()
var assetId = fmt.Sprintf("asset%d", now.Unix()*1e3+int64(now.Nanosecond())/1e6)

func main() {
	// The gRPC client connection should be shared by all Gateway connections to this endpoint
	clientConnection := newGrpcConnection()
	defer clientConnection.Close()

	id := newIdentity()
	sign := newSign()

	// Create a Gateway connection for a specific client identity
	gw, err := client.Connect(
		id,
		client.WithSign(sign),
		client.WithHash(hash.SHA256),
		client.WithClientConnection(clientConnection),
		// Default timeouts for different gRPC calls
		client.WithEvaluateTimeout(5*time.Second),
		client.WithEndorseTimeout(15*time.Second),
		client.WithSubmitTimeout(5*time.Second),
		client.WithCommitStatusTimeout(1*time.Minute),
	)
	if err != nil {
		panic(err)
	}
	defer gw.Close()

	// Override default values for chaincode and channel name as they may differ in testing contexts.
	chaincodeName := "cbdc"
	if ccname := os.Getenv("CHAINCODE_NAME"); ccname != "" {
		chaincodeName = ccname
	}

	channelName := "retail"
	if cname := os.Getenv("CHANNEL_NAME"); cname != "" {
		channelName = cname
	}

	network := gw.GetNetwork(channelName)
	contract := network.GetContract(chaincodeName)
	Contract = contract
	initLedgerIfNotAlready(contract)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", ApplicationPort))
	if err != nil {
		fmt.Errorf(fmt.Sprintf("Failed to listen %v", err))
	}
	var grpcServer = grpc.NewServer()
	cbdc.RegisterCBDCServer(grpcServer, &server{})
	fmt.Println("Serving gRPC server on 0.0.0.0:", ApplicationPort)
	if errServer := grpcServer.Serve(lis); errServer != nil {
		log.Fatalf("failed to serve: %v", errServer)
	}
}

func (s *server) Mint(ctx context.Context, req *cbdc.MintRequest) (*cbdc.MintResponse, error) {
	txId, acc, amt, suc, msg := mintRequest(Contract, req.Account, req.Amount)
	return &cbdc.MintResponse{
		TxId:    txId,
		Account: acc,
		Amount:  amt,
		Success: suc,
		Message: msg,
	}, nil
}
