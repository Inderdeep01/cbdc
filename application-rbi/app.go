package main

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/hyperledger/fabric-gateway/pkg/client"
	"github.com/hyperledger/fabric-gateway/pkg/identity"
	"github.com/hyperledger/fabric-protos-go-apiv2/gateway"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
	"os"
	"path"
	"slices"
	"strconv"
)

const (
	HDFCBankAccount = "hdfc.cbdc"
	AxisBankAccount = "axis.cbdc"
)

func getCommercialBankAccounts() []string {
	return []string{"hdfc.cbdc", "axis.cbdc"}
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

func initLedgerIfNotAlready(contract *client.Contract) {
	fmt.Println("\n--> Evaluate Transaction: Name, returns an abbreviated name for fungible tokens in the contract")
	evaluateResult, err := contract.EvaluateTransaction("Symbol")
	if err != nil {
		initLedger(contract)
		mint(contract)
		transferHDFC(contract)
		transferAxis(contract)
	}
	result := string(evaluateResult)

	fmt.Printf("*** Currency:%s\n", result)
}

// This type of transaction would typically only be run once by an application the first time it was started after its
// initial deployment. A new version of the chaincode deployed later would likely not need to run an "init" function.
func initLedger(contract *client.Contract) {
	fmt.Printf("\n--> Submit Transaction: Initialize, function creates the initial CBDC on the ledger \n")

	evaluateResult, commit, err := contract.SubmitAsync("Initialize", client.WithArguments("Indian eRupee", "eINR", "2"))
	if err != nil {
		panic(fmt.Errorf("failed to submit transaction: %w", err))
	}
	fmt.Println("*** Waiting for transaction commit.")

	if commitStatus, err := commit.Status(); err != nil {
		panic(fmt.Errorf("failed to get commit status: %w", err))
	} else if !commitStatus.Successful {
		panic(fmt.Errorf("transaction %s failed to commit with status: %d", commitStatus.TransactionID, int32(commitStatus.Code)))
	}

	result := formatJSON(evaluateResult)

	fmt.Printf("*** Result:%s\n", result)

	fmt.Printf("*** Transaction committed successfully\n")
}

func mint(contract *client.Contract) {
	fmt.Printf("\n--> Submit Transaction: Mint, creates new tokens and adds them to minter's account balance \n")

	_, err := contract.SubmitTransaction("Mint", "20000")
	if err != nil {
		panic(fmt.Errorf("failed to submit transaction: %w", err))
	}

	fmt.Printf("*** Transaction committed successfully\n")
}

func mintRequest(contract *client.Contract, account string, amount uint64) (string, string, uint64, bool, string) {
	if slices.Contains(getCommercialBankAccounts(), account) {
		value := strconv.FormatUint(amount, 10)
		_, commit, err := contract.SubmitAsync("Mint", client.WithArguments(value))
		if err != nil {
			return "xxxxx", account, amount, false, fmt.Sprintf("Failed to Submit due to error: %v", err)
		}
		fmt.Println("*** Waiting for transaction commit.")

		commitStatus, err := commit.Status()
		if err != nil {
			panic(fmt.Errorf("failed to get commit status: %w", err))
		} else if !commitStatus.Successful {
			fmt.Printf("transaction %s failed to commit with status: %d", commitStatus.TransactionID, int32(commitStatus.Code))
			return commitStatus.TransactionID, account, amount, false, fmt.Sprintf("transaction %s failed to commit with status: %d", commitStatus.TransactionID, int32(commitStatus.Code))
		}
		success := transferToAppropriateBank(contract, account, value)
		if !success {
			return commitStatus.TransactionID, account, amount, false, "Failed to transfer"
		}
		return commitStatus.TransactionID, account, amount, true, "Success!"
	}
	return "xxxxx", account, amount, false, "Not Authorized to Mint!"
}

func transferToAppropriateBank(contract *client.Contract, account, amount string) bool {
	successFlag := false
	switch account {
	case HDFCBankAccount:
		successFlag = transferHDFCAmount(contract, amount)
		break
	case AxisBankAccount:
		successFlag = transferAxisAmount(contract, amount)
		break
	}
	return successFlag
}

// Transfer to Axis
func transferAxis(contract *client.Contract) {
	fmt.Printf("\n--> Submit Transaction: Transfer, transfers tokens from client account to recipient account \n")

	_, commit, err := contract.SubmitAsync("Transfer", client.WithArguments("axis.cbdc", "10000"))
	if err != nil {
		panic(fmt.Errorf("failed to submit transaction: %w", err))
	}
	fmt.Println("*** Waiting for transaction commit.")

	if commitStatus, err := commit.Status(); err != nil {
		panic(fmt.Errorf("failed to get commit status: %w", err))
	} else if !commitStatus.Successful {
		panic(fmt.Errorf("transaction %s failed to commit with status: %d", commitStatus.TransactionID, int32(commitStatus.Code)))
	}

	fmt.Printf("*** Transaction committed successfully\n")
}

// Transfer to Axis
func transferAxisAmount(contract *client.Contract, amount string) bool {
	fmt.Printf("\n--> Transferring %s to %s \n", amount, AxisBankAccount)

	_, commit, err := contract.SubmitAsync("Transfer", client.WithArguments(AxisBankAccount, amount))
	if err != nil {
		fmt.Printf("failed to submit transaction: %v\n", err)
		return false
	}
	fmt.Println("*** Waiting for transaction commit.")

	if commitStatus, err := commit.Status(); err != nil {
		fmt.Printf("failed to get commit status: %v\n", err)
		return false
	} else if !commitStatus.Successful {
		fmt.Printf("transaction %s failed to commit with status: %d\n", commitStatus.TransactionID, int32(commitStatus.Code))
		return false
	}

	fmt.Printf("*** Transaction committed successfully\n")
	return true
}

// Transfer to HDFC
func transferHDFC(contract *client.Contract) {
	fmt.Printf("\n--> Submit Transaction: Transfer, transfers tokens from client account to recipient account \n")

	_, commit, err := contract.SubmitAsync("Transfer", client.WithArguments("hdfc.cbdc", "10000"))
	if err != nil {
		panic(fmt.Errorf("failed to submit transaction: %w", err))
	}
	fmt.Println("*** Waiting for transaction commit.")

	if commitStatus, err := commit.Status(); err != nil {
		panic(fmt.Errorf("failed to get commit status: %w", err))
	} else if !commitStatus.Successful {
		panic(fmt.Errorf("transaction %s failed to commit with status: %d", commitStatus.TransactionID, int32(commitStatus.Code)))
	}

	fmt.Printf("*** Transaction committed successfully\n")
}

// Transfer to Axis
func transferHDFCAmount(contract *client.Contract, amount string) bool {
	fmt.Printf("\n--> Transferring %s to %s \n", amount, HDFCBankAccount)

	_, commit, err := contract.SubmitAsync("Transfer", client.WithArguments(HDFCBankAccount, amount))
	if err != nil {
		fmt.Printf("failed to submit transaction: %v\n", err)
		return false
	}
	fmt.Println("*** Waiting for transaction commit.")

	if commitStatus, err := commit.Status(); err != nil {
		fmt.Printf("failed to get commit status: %v\n", err)
		return false
	} else if !commitStatus.Successful {
		fmt.Printf("transaction %s failed to commit with status: %d\n", commitStatus.TransactionID, int32(commitStatus.Code))
		return false
	}

	fmt.Printf("*** Transaction committed successfully\n")
	return true
}

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
func getCurrentClientId(contract *client.Contract) {
	fmt.Println("\n--> Evaluate Transaction: ClientAccountID, function returns the id of the requesting client's account")
	evaluateResult, err := contract.EvaluateTransaction("ClientAccountID")
	if err != nil {
		panic(fmt.Errorf("failed to evaluate transaction: %w", err))
	}
	result := string(evaluateResult)

	fmt.Printf("*** Result:%s\n", result)
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

// Evaluate a transaction to query ledger state.
func getAllAssets(contract *client.Contract) {
	fmt.Println("\n--> Evaluate Transaction: GetAllAssets, function returns all the current assets on the ledger")

	evaluateResult, err := contract.EvaluateTransaction("GetAllAssets")
	if err != nil {
		panic(fmt.Errorf("failed to evaluate transaction: %w", err))
	}
	result := formatJSON(evaluateResult)

	fmt.Printf("*** Result:%s\n", result)
}

// Submit a transaction synchronously, blocking until it has been committed to the ledger.
func createAsset(contract *client.Contract) {
	fmt.Printf("\n--> Submit Transaction: CreateAsset, creates new asset with ID, Color, Size, Owner and AppraisedValue arguments \n")

	_, err := contract.SubmitTransaction("CreateAsset", assetId, "yellow", "5", "Tom", "1300")
	if err != nil {
		panic(fmt.Errorf("failed to submit transaction: %w", err))
	}

	fmt.Printf("*** Transaction committed successfully\n")
}

// Evaluate a transaction by assetID to query ledger state.
func readAssetByID(contract *client.Contract) {
	fmt.Printf("\n--> Evaluate Transaction: ReadAsset, function returns asset attributes\n")

	evaluateResult, err := contract.EvaluateTransaction("ReadAsset", assetId)
	if err != nil {
		panic(fmt.Errorf("failed to evaluate transaction: %w", err))
	}
	result := formatJSON(evaluateResult)

	fmt.Printf("*** Result:%s\n", result)
}

// Submit transaction asynchronously, blocking until the transaction has been sent to the orderer, and allowing
// this thread to process the chaincode response (e.g. update a UI) without waiting for the commit notification
func transferAssetAsync(contract *client.Contract) {
	fmt.Printf("\n--> Async Submit Transaction: TransferAsset, updates existing asset owner")

	submitResult, commit, err := contract.SubmitAsync("TransferAsset", client.WithArguments(assetId, "Mark"))
	if err != nil {
		panic(fmt.Errorf("failed to submit transaction asynchronously: %w", err))
	}

	fmt.Printf("\n*** Successfully submitted transaction to transfer ownership from %s to Mark. \n", string(submitResult))
	fmt.Println("*** Waiting for transaction commit.")

	if commitStatus, err := commit.Status(); err != nil {
		panic(fmt.Errorf("failed to get commit status: %w", err))
	} else if !commitStatus.Successful {
		panic(fmt.Errorf("transaction %s failed to commit with status: %d", commitStatus.TransactionID, int32(commitStatus.Code)))
	}

	fmt.Printf("*** Transaction committed successfully\n")
}

// Submit transaction, passing in the wrong number of arguments ,expected to throw an error containing details of any error responses from the smart contract.
func exampleErrorHandling(contract *client.Contract) {
	fmt.Println("\n--> Submit Transaction: UpdateAsset asset70, asset70 does not exist and should return an error")

	_, err := contract.SubmitTransaction("UpdateAsset", "asset70", "blue", "5", "Tomoko", "300")
	if err == nil {
		panic("******** FAILED to return an error")
	}

	fmt.Println("*** Successfully caught the error:")

	var endorseErr *client.EndorseError
	var submitErr *client.SubmitError
	var commitStatusErr *client.CommitStatusError
	var commitErr *client.CommitError

	if errors.As(err, &endorseErr) {
		fmt.Printf("Endorse error for transaction %s with gRPC status %v: %s\n", endorseErr.TransactionID, status.Code(endorseErr), endorseErr)
	} else if errors.As(err, &submitErr) {
		fmt.Printf("Submit error for transaction %s with gRPC status %v: %s\n", submitErr.TransactionID, status.Code(submitErr), submitErr)
	} else if errors.As(err, &commitStatusErr) {
		if errors.Is(err, context.DeadlineExceeded) {
			fmt.Printf("Timeout waiting for transaction %s commit status: %s", commitStatusErr.TransactionID, commitStatusErr)
		} else {
			fmt.Printf("Error obtaining commit status for transaction %s with gRPC status %v: %s\n", commitStatusErr.TransactionID, status.Code(commitStatusErr), commitStatusErr)
		}
	} else if errors.As(err, &commitErr) {
		fmt.Printf("Transaction %s failed to commit with status %d: %s\n", commitErr.TransactionID, int32(commitErr.Code), err)
	} else {
		panic(fmt.Errorf("unexpected error type %T: %w", err, err))
	}

	// Any error that originates from a peer or orderer node external to the gateway will have its details
	// embedded within the gRPC status error. The following code shows how to extract that.
	statusErr := status.Convert(err)

	details := statusErr.Details()
	if len(details) > 0 {
		fmt.Println("Error Details:")

		for _, detail := range details {
			switch detail := detail.(type) {
			case *gateway.ErrorDetail:
				fmt.Printf("- address: %s; mspId: %s; message: %s\n", detail.Address, detail.MspId, detail.Message)
			}
		}
	}
}

// Format JSON data
func formatJSON(data []byte) string {
	var prettyJSON bytes.Buffer
	if err := json.Indent(&prettyJSON, data, "", "  "); err != nil {
		panic(fmt.Errorf("failed to parse JSON: %w", err))
	}
	return prettyJSON.String()
}
