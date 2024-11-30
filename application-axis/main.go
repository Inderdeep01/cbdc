package main

import (
	cbdc "app/api"
	"context"
	"fmt"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/hyperledger/fabric-gateway/pkg/client"
	"github.com/hyperledger/fabric-gateway/pkg/hash"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"
)

const (
	mspID           = "AxisBankMSP"
	cryptoPath      = "../network/organizations/peerOrganizations/axis.bank.cbdc"
	certPath        = cryptoPath + "/users/User1@axis.bank.cbdc/msp/signcerts"
	keyPath         = cryptoPath + "/users/User1@axis.bank.cbdc/msp/keystore"
	tlsCertPath     = cryptoPath + "/peers/peer0.axis.bank.cbdc/tls/ca.crt"
	peerEndpoint    = "dns:///localhost:10051"
	gatewayPeer     = "peer0.axis.bank.cbdc"
	ApplicationPort = 10999
	GatewayPort     = 10998
	RBIPort         = 7999
	BankAccount     = "axis.cbdc"
)

type server struct {
	cbdc.CBDCServer
}

var (
	Contract  *client.Contract
	RBIClient cbdc.CBDCClient
)

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
	getCurrentClientId(contract)

	// Set up a gRPC Server
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", ApplicationPort))
	if err != nil {
		fmt.Errorf(fmt.Sprintf("Failed to listen %v", err))
	}
	var grpcServer = grpc.NewServer()
	cbdc.RegisterCBDCServer(grpcServer, &server{})
	go func() {
		if errServer := grpcServer.Serve(lis); errServer != nil {
			log.Fatalf("failed to serve: %v", errServer)
		}
	}()

	// Connect to RBI Server
	rbiConn, err := grpc.NewClient(fmt.Sprintf("0.0.0.0:%d", RBIPort), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalln("Failed to Dial RBI Server", err)
	}
	rbiClient := cbdc.NewCBDCClient(rbiConn)
	RBIClient = rbiClient

	// Connect gRPC-Gateway to your gRPC-Server
	conn, err := grpc.NewClient(fmt.Sprintf("0.0.0.0:%d", ApplicationPort), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalln("Failed to Dial Server", err)
	}
	gwmux := runtime.NewServeMux()
	err = cbdc.RegisterCBDCHandler(context.Background(), gwmux, conn)
	if err != nil {
		log.Fatalln("Failed to register gateway", err)
	}
	gwServer := &http.Server{Addr: ":10998", Handler: gwmux}
	log.Printf("Serving gRPC-Gateway on http://0.0.0.0:%d\n", GatewayPort)
	log.Fatalln(gwServer.ListenAndServe())
}

func (s *server) GetBalance(ctx context.Context, req *cbdc.GetBalanceRequest) (*cbdc.GetBalanceResponse, error) {
	balance, err := getClientBalance(Contract, req.GetAccount())
	if err != nil {
		return nil, err
	}
	return &cbdc.GetBalanceResponse{Balance: balance}, nil
}

func (s *server) CreateAccount(ctx context.Context, req *cbdc.CreateAccountRequest) (*cbdc.CreateAccountResponse, error) {
	_, _, _, _, _, msg := transferFrom(Contract, BankAccount, req.Account, "100")
	_, acc, _, _, suc, msg := transferFrom(Contract, req.Account, BankAccount, "100")

	return &cbdc.CreateAccountResponse{
		Account: acc,
		Success: suc,
		Message: msg,
	}, nil
}

func (s *server) Tx(ctx context.Context, req *cbdc.TxRequest) (*cbdc.TxResponse, error) {
	txId, from, to, amount, success, msg := transferFrom(Contract, req.From, req.To, strconv.FormatUint(req.Amount, 10))
	return &cbdc.TxResponse{
		TxId:    txId,
		From:    from,
		To:      to,
		Amount:  amount,
		Success: success,
		Message: msg,
	}, nil
}

func (s *server) Fund(ctx context.Context, req *cbdc.FundRequest) (*cbdc.FundResponse, error) {
	txId, acc, amt, success, msg := fund(Contract, ctx, req.Account, req.Amount)
	return &cbdc.FundResponse{
		TxId:    txId,
		Account: acc,
		Amount:  amt,
		Success: success,
		Message: msg,
	}, nil
}
