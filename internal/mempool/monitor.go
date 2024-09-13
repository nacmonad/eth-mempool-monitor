package mempool

import (
	"bytes"
	"context"
	"encoding/json"
	"eth-mempool-monitor/internal/decoder"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
)

// Global variables
var (
	wsEndpoint    string
	httpsEndpoint string
	username      string
	password      string
	txCount       uint64     // Counter for the number of transactions
	contracts     []Contract // Loaded contracts
	recentTx      string
)

var relevantSelectorsUniswap = map[string]bool{
	"38ed1739": true, // swapExactTokensForTokens
	"8803dbee": true, // swapTokensForExactTokens
	"7ff36ab5": true, // swapExactETHForTokens
	"4a25d94a": true, // swapTokensForExactETH
	"18cbafe5": true, // swapExactTokensForETH
	"fb3bdb41": true, // swapETHForExactTokens
	"e8e33700": true, // addLiquidity
	"f305d719": true, // addLiquidityETH
	"baa2abde": true, // removeLiquidity
	"02751cec": true, // removeLiquidityETH
}

var relevantSelectorsWETH = map[string]bool{
	"d0e30db0": true, // deposit
	"2e1a7d4d": true, // withdraw
	"095ea7b3": true, // approve
	"a9059cbb": true, // transfer
	"23b872dd": true, // transferFrom
}

// Combine the two maps into a single map
var relevantSelectors = make(map[string]bool)

// Initialize and load environment variables
func init() {
	// Merge selectors from Uniswap
	for key, value := range relevantSelectorsUniswap {
		relevantSelectors[key] = value
	}

	// Merge selectors from WETH
	for key, value := range relevantSelectorsWETH {
		relevantSelectors[key] = value
	}

	// Load the environment variables from .env file
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}

	// Assign environment variables to package-level variables
	wsEndpoint = os.Getenv("WS_ENDPOINT")
	httpsEndpoint = os.Getenv("HTTPS_ENDPOINT")
	username = os.Getenv("USERNAME")
	password = os.Getenv("PASSWORD")

	// Load contracts from the configuration file
	contracts, err = LoadContracts("configs/contracts.json")
	if err != nil {
		log.Fatalf("Error loading contracts: %v", err)
	}
}

// MonitorMempool connects to the Ethereum mempool via WebSocket and listens for new pending transactions
func MonitorMempool(ctx context.Context, tpsChan chan uint64, txChan chan string, txDetailsChan chan string) {
	// Setup a dialer for connecting with basic authentication
	dialer := websocket.Dialer{
		Proxy: http.ProxyFromEnvironment,
	}

	header := http.Header{}
	header.Set("Authorization", "Basic "+basicAuth(username, password))

	// Connect to the WebSocket
	conn, _, err := dialer.Dial(wsEndpoint, header)
	if err != nil {
		log.Fatalf("Failed to connect to WebSocket: %v", err)
	}
	defer conn.Close()

	// Subscribe to new pending transactions
	subscribe := `{"jsonrpc":"2.0","id":1,"method":"eth_subscribe","params":["newPendingTransactions"]}`
	if err := conn.WriteMessage(websocket.TextMessage, []byte(subscribe)); err != nil {
		log.Fatalf("Failed to subscribe: %v", err)
	}

	// Create a channel to handle incoming messages
	msgChan := make(chan string)

	// Launch a goroutine to listen to incoming messages
	go func() {
		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				log.Printf("Error reading message: %v", err)
				return
			}
			msgChan <- string(message)
		}
	}()

	// Use a ticker to calculate and display TPS every second
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	// Use a goroutine and channel to handle messages
	for {
		select {
		case <-ctx.Done():
			fmt.Println("Shutting down mempool monitoring...")
			return
		case <-ticker.C:
			// Calculate and display TPS
			currentTxCount := atomic.SwapUint64(&txCount, 0) // Atomically get and reset the transaction count
			tpsChan <- currentTxCount
		case msg := <-msgChan:
			go processTransaction(msg, txChan, txDetailsChan) // Process transaction in a separate goroutine
		}
	}
}

// Filter transactions based on relevant selectors
func filterTransaction(inputData string) bool {

	if len(inputData) < 8 {
		//log.Printf("Invalid input data (too short): %s", inputData)
		return false
	}
	// Remove the "0x" prefix
	inputData = strings.TrimPrefix(inputData, "0x")

	// Get the method selector (first 4 bytes)
	methodSelector := inputData[:8]

	// Check if the method selector is in the relevant selectors map
	_, exists := relevantSelectors[methodSelector]
	return exists
}

// Fetch the full transaction details and check if it pertains to one of the loaded contracts
func fetchTransactionDetails(txHash string, txChan chan string, txDetailsChan chan string) {
	// Define the payload for the JSON-RPC request
	payload := fmt.Sprintf(`{"jsonrpc":"2.0","method":"eth_getTransactionByHash","params":["%s"],"id":1}`, txHash)

	req, err := http.NewRequest("POST", httpsEndpoint, bytes.NewBuffer([]byte(payload)))
	if err != nil {
		log.Printf("Failed to create request: %v", err)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(username, password)

	// Send the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Failed to send request: %v", err)
		return
	}
	defer resp.Body.Close()

	// Parse the response
	var result struct {
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
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("Failed to decode response: %v", err)
		return
	}

	atomic.AddUint64(&txCount, 1)

	// Filter based on the relevant selectors
	if !filterTransaction(result.Result.Input) {
		return // Skip transactions that are not relevant
	}

	// Check if the transaction is to one of the loaded contracts
	// Check if the transaction is to one of the loaded contracts
	for _, contract := range contracts {
		if result.Result.To != "" && common.HexToAddress(result.Result.To) == common.HexToAddress(contract.Address) {
			recentTx := fmt.Sprintf("Transaction to contract (%s) at %s:\n", contract.Name, time.Now())
			recentTx += fmt.Sprintf("Hash: %s\n", result.Result.Hash)
			recentTx += fmt.Sprintf("From: %s\n", result.Result.From)
			recentTx += fmt.Sprintf("To: %s\n", result.Result.To)
			recentTx += fmt.Sprintf("Value: %s\n", result.Result.Value)
			recentTx += fmt.Sprintf("Gas: %s\n", result.Result.Gas)
			recentTx += fmt.Sprintf("Gas Price: %s\n", result.Result.GasPrice)
			recentTx += fmt.Sprintf("Nonce: %s\n", result.Result.Nonce)
			recentTx += fmt.Sprintf("Block Hash: %s\n", result.Result.BlockHash)
			recentTx += fmt.Sprintf("Block Number: %s\n", result.Result.BlockNumber)
			recentTx += fmt.Sprintf("Transaction Index: %s\n", result.Result.TransactionIndex)
			recentTx += fmt.Sprintf("Input Data: %s\n", result.Result.Input)
			recentTx += fmt.Sprintf("V: %s, R: %s, S: %s\n", result.Result.V, result.Result.R, result.Result.S)

			txChan <- recentTx // Send the transaction details to the channel

			decoder.DecodeInputData(result, string(contract.ABI), txDetailsChan) // Use the decoder to parse the input

			break
		}
	}
}

// Process the transaction to check if it pertains to any of the loaded contracts
func processTransaction(msg string, txChan chan string, txDetailsChan chan string) {
	// Define the correct struct based on the provided JSON
	var tx struct {
		Jsonrpc string `json:"jsonrpc"`
		Method  string `json:"method"`
		Params  struct {
			Subscription string `json:"subscription"`
			Result       string `json:"result"` // Transaction hash
		} `json:"params"`
	}

	// Attempt to parse the JSON message
	err := json.Unmarshal([]byte(msg), &tx)
	if err != nil {
		log.Printf("Failed to parse transaction message: %v", err)
		return
	}

	// Fetch the transaction details by its hash
	fetchTransactionDetails(tx.Params.Result, txChan, txDetailsChan)
}

// basicAuth encodes the username and password for basic authentication
func basicAuth(username, password string) string {
	return "Basic " + username + ":" + password
}
