package main

import (
	cbdc "app/api"
	"bytes"
	"context"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"github.com/hyperledger/fabric-gateway/pkg/client"
	"github.com/hyperledger/fabric-gateway/pkg/identity"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"os"
	"path"
	"strconv"
)

// Get Name
func getName(contract *client.Contract) {
	fmt.Println("\n--> Evaluate Transaction: Name, returns a descriptive name for fungible tokens in the contract")
	evaluateResult, err := contract.EvaluateTransaction("Name")
	if err != nil {
		panic(fmt.Errorf("failed to evaluate transaction: %w", err))
	}
	result := string(evaluateResult)

	fmt.Printf("*** Result:%s\n", result)
}

// Get Symbol
func getSymbol(contract *client.Contract) {
	fmt.Println("\n--> Evaluate Transaction: Name, returns an abbreviated name for fungible tokens in the contract")
	evaluateResult, err := contract.EvaluateTransaction("Symbol")
	if err != nil {
		panic(fmt.Errorf("failed to evaluate transaction: %w", err))
	}
	result := string(evaluateResult)

	fmt.Printf("*** Result:%s\n", result)
}

// Get Current Client Id
func getCurrentClientId(contract *client.Contract) string {
	fmt.Println("\n--> Evaluate Transaction: ClientAccountID, function returns the id of the requesting client's account")
	evaluateResult, err := contract.EvaluateTransaction("ClientAccountID")
	if err != nil {
		panic(fmt.Errorf("failed to evaluate transaction: %w", err))
	}
	result := string(evaluateResult)

	fmt.Printf("*** Result:%s\n", result)
	return result
}

// Get Current Client Balance
func getCurrentClientBalance(contract *client.Contract) {
	fmt.Println("\n--> Evaluate Transaction: ClientAccountID, function returns the balance of the requesting client's account")
	evaluateResult, err := contract.EvaluateTransaction("ClientAccountBalance")
	if err != nil {
		panic(fmt.Errorf("failed to evaluate transaction: %w", err))
	}
	result := string(evaluateResult)

	fmt.Printf("*** Result:%s\n", result)
}

// Get any Client Balance
func getClientBalance(contract *client.Contract, account string) (uint64, error) {
	fmt.Println("\n--> Evaluate Transaction: ClientAccountID, function returns the balance of the requesting client's account")
	evaluateResult, err := contract.EvaluateTransaction("BalanceOf", account)
	if err != nil {
		panic(fmt.Errorf("failed to evaluate transaction: %w", err))
	}
	result := string(evaluateResult)

	fmt.Printf("*** %s:%s\n", account, result)
	bal, err := strconv.Atoi(result)
	if err != nil {
		return 0, err
	}
	return uint64(bal), nil
}

// Transfer to end user
func transferToUser(contract *client.Contract, account, amount string) (string, bool, string) {
	fmt.Println("\n--> Submit Transaction: Transfer, transfers to an end user e.g., inderdeep.cbdc")
	_, commit, err := contract.SubmitAsync("Transfer", client.WithArguments(account, amount))
	if err != nil {
		panic(fmt.Errorf("failed to submit transaction: %w \n %v", err, err.Error()))
	}
	fmt.Println("*** Waiting for transaction commit.")

	if commitStatus, err := commit.Status(); err != nil {
		panic(fmt.Errorf("failed to get commit status: %w", err))
	} else if !commitStatus.Successful {
		fmt.Printf("transaction %s failed to commit with status: %d", commitStatus.TransactionID, int32(commitStatus.Code))
		return "", false, fmt.Sprintf("transaction %s failed to commit with status: %d", commitStatus.TransactionID, int32(commitStatus.Code))
		//panic(fmt.Errorf("transaction %s failed to commit with status: %d", commitStatus.TransactionID, int32(commitStatus.Code)))
	}
	fmt.Printf("*** Transaction committed successfully\n")
	return account, true, "Account Created Successfully"
}

func transferFrom(contract *client.Contract, from, to, amount string) (string, string, string, uint64, bool, string) {
	fmt.Printf("\n--> Transfer %s %s->%s", amount, from, to)
	value, err := strconv.Atoi(amount)
	if err != nil || value <= 0 {
		return "xxxxx", from, to, 0, false, fmt.Sprintf("Invalid Amount %v; generated error %v", amount, err)
	}
	_, commit, err := contract.SubmitAsync("TransferFrom", client.WithArguments(from, to, amount))

	if err != nil {
		panic(fmt.Errorf("failed to submit transaction: %w", err))
	}
	fmt.Println("*** Waiting for transaction commit.")

	commitStatus, err := commit.Status()
	if err != nil {
		panic(fmt.Errorf("failed to get commit status: %w", err))
	} else if !commitStatus.Successful {
		fmt.Printf("transaction %s failed to commit with status: %d", commitStatus.TransactionID, int32(commitStatus.Code))
		return commitStatus.TransactionID, from, to, uint64(value), false, fmt.Sprintf("transaction %s failed to commit with status: %d", commitStatus.TransactionID, int32(commitStatus.Code))
		//panic(fmt.Errorf("transaction %s failed to commit with status: %d", commitStatus.TransactionID, int32(commitStatus.Code)))
	}
	fmt.Printf("*** Transaction committed successfully\n")
	return commitStatus.TransactionID, from, to, uint64(value), true, "Transaction Committed Successfully"
}

func fund(contract *client.Contract, ctx context.Context, account string, amount uint64) (string, string, uint64, bool, string) {
	res, err := RBIClient.Mint(ctx, &cbdc.MintRequest{
		Account: BankAccount,
		Amount:  amount,
	})
	if err != nil || !res.Success {
		fmt.Println(err)
		return res.TxId, res.Account, res.Amount, false, res.Message
	}
	txId, _, to, amount, success, msg := transferFrom(contract, BankAccount, account, strconv.FormatUint(amount, 10))
	return txId, to, amount, success, msg
}

// newGrpcConnection creates a gRPC connection to the Gateway server.
func newGrpcConnection() *grpc.ClientConn {
	certificatePEM, err := os.ReadFile(tlsCertPath)
	if err != nil {
		panic(fmt.Errorf("failed to read TLS certifcate file: %w", err))
	}

	certificate, err := identity.CertificateFromPEM(certificatePEM)
	if err != nil {
		panic(err)
	}

	certPool := x509.NewCertPool()
	certPool.AddCert(certificate)
	transportCredentials := credentials.NewClientTLSFromCert(certPool, gatewayPeer)

	connection, err := grpc.NewClient(peerEndpoint, grpc.WithTransportCredentials(transportCredentials))
	if err != nil {
		panic(fmt.Errorf("failed to create gRPC connection: %w", err))
	}

	return connection
}

// newIdentity creates a client identity for this Gateway connection using an X.509 certificate.
func newIdentity() *identity.X509Identity {
	certificatePEM, err := readFirstFile(certPath)
	if err != nil {
		panic(fmt.Errorf("failed to read certificate file: %w", err))
	}

	certificate, err := identity.CertificateFromPEM(certificatePEM)
	if err != nil {
		panic(err)
	}

	id, err := identity.NewX509Identity(mspID, certificate)
	if err != nil {
		panic(err)
	}

	return id
}

// newSign creates a function that generates a digital signature from a message digest using a private key.
func newSign() identity.Sign {
	privateKeyPEM, err := readFirstFile(keyPath)
	if err != nil {
		panic(fmt.Errorf("failed to read private key file: %w", err))
	}

	privateKey, err := identity.PrivateKeyFromPEM(privateKeyPEM)
	if err != nil {
		panic(err)
	}

	sign, err := identity.NewPrivateKeySign(privateKey)
	if err != nil {
		panic(err)
	}

	return sign
}

func readFirstFile(dirPath string) ([]byte, error) {
	dir, err := os.Open(dirPath)
	if err != nil {
		return nil, err
	}

	fileNames, err := dir.Readdirnames(1)
	if err != nil {
		return nil, err
	}

	return os.ReadFile(path.Join(dirPath, fileNames[0]))
}

// Format JSON data
func formatJSON(data []byte) string {
	var prettyJSON bytes.Buffer
	if err := json.Indent(&prettyJSON, data, "", "  "); err != nil {
		panic(fmt.Errorf("failed to parse JSON: %w", err))
	}
	return prettyJSON.String()
}
