package decoder

import (
	"encoding/hex"
	"eth-mempool-monitor/internal/cache"
	"fmt"
	"log"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
)

// Define the struct type for the transaction result
type TransactionResult struct {
	Result struct {
		BlockHash        string `json:"blockHash"`
		BlockNumber      string `json:"blockNumber"`
		From             string `json:"from"`
		Gas              string `json:"gas"`
		GasPrice         string `json:"gasPrice"`
		Hash             string `json:"hash"`
		Input            string `json:"input"`
		Nonce            string `json:"nonce"`
		To               string `json:"to"`
		TransactionIndex string `json:"transactionIndex"`
		Value            string `json:"value"`
		V                string `json:"v"`
		R                string `json:"r"`
		S                string `json:"s"`
	} `json:"result"`
}

// DecodeInputData decodes the input data of a transaction using the provided ABI
func DecodeInputData(result TransactionResult, contractABI string, txDetailsChan chan string) {
	// Remove the "0x" prefix
	inputData := strings.TrimPrefix(result.Result.Input, "0x")

	// Decode the method selector (first 4 bytes)
	methodSelector := inputData[:8]

	// Decode the parameters (remaining bytes)
	data, err := hex.DecodeString(inputData[8:])
	if err != nil {
		log.Fatalf("Failed to decode input data: %v", err)
	}

	// Parse the provided ABI
	parsedABI, err := abi.JSON(strings.NewReader(contractABI))
	if err != nil {
		log.Fatalf("Failed to parse ABI: %v", err)
	}

	// Use the ABI to decode the method and parameters
	method, err := parsedABI.MethodById(common.FromHex("0x" + methodSelector))
	if err != nil {
		log.Printf("Failed to identify method: %v", err)
		return
	}

	// Send the method name to txDetailsChan
	txDetailsChan <- fmt.Sprintf("TxHash: %s\n", result.Result.Hash)
	txDetailsChan <- fmt.Sprintf("Method Name: %s\n", method.Name)

	// Decode the parameters
	params, err := method.Inputs.Unpack(data)
	if err != nil {
		log.Printf("Failed to unpack parameters: %v", err)
		return
	}

	// Send the decoded parameters in a human-readable way to txDetailsChan
	for i, param := range params {
		var formattedParam string

		switch v := param.(type) {
		case *big.Int:
			// Convert large numbers to decimal strings
			formattedParam = fmt.Sprintf("  %s (%s): %s\n", method.Inputs[i].Name, method.Inputs[i].Type, v.String())
		case common.Address:
			// Format Ethereum addresses
			formattedParam = fmt.Sprintf("  %s (%s): %s\n", method.Inputs[i].Name, method.Inputs[i].Type, v.Hex())
		case []common.Address:
			// Handle an array of Ethereum addresses and fetch token details
			formattedParam = fmt.Sprintf("  %s (%s):\n", method.Inputs[i].Name, method.Inputs[i].Type)
			for _, addr := range v {
				// Fetch the token details
				tokenInfo, err := cache.FetchTokenDetails(addr)
				if err != nil {
					formattedParam += fmt.Sprintf("    - %s (Token details fetch failed)\n", addr.Hex())
				} else {
					formattedParam += fmt.Sprintf("    - %s (%s: %s)\n", addr.Hex(), tokenInfo.Symbol, tokenInfo.Name)
				}
			}
		default:
			// Print the value directly if no special formatting is needed
			formattedParam = fmt.Sprintf("  %s (%s): %v\n", method.Inputs[i].Name, method.Inputs[i].Type, param)
		}

		// Send the formatted string to txChan
		txDetailsChan <- formattedParam
	}
}
