package cache

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"math/big"
	"os"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rpc"
)

// Define a struct for token information
type TokenInfo struct {
	Address  string
	Symbol   string
	Name     string
	Decimals uint8
}

// A map to store known tokens, with the token address as the key
var TokenCache = make(map[string]TokenInfo)

// Global RPC client
var RpcClient *rpc.Client

// InitializeRPCClient initializes the RPC client using the provided HTTPS endpoint
func InitializeRPCClient() error {
	httpsEndpoint := os.Getenv("HTTPS_ENDPOINT")
	if httpsEndpoint == "" {
		return fmt.Errorf("HTTPS_ENDPOINT is not set in environment variables")
	}

	var err error
	RpcClient, err = rpc.Dial(httpsEndpoint)
	if err != nil {
		return err
	}
	return nil
}

// DecodeHexStringIfNeeded checks if a string is hex-encoded and decodes it to ASCII if necessary
func DecodeHexStringIfNeeded(str string) string {
	// If the string starts with "0x", it may be hex-encoded
	if strings.HasPrefix(str, "0x") {
		decodedBytes, err := hex.DecodeString(str[2:]) // Remove "0x" prefix and decode
		if err == nil {
			return string(decodedBytes)
		}
	}
	return str // Return as is if it's not hex or decoding fails
}

// FetchTokenDetails retrieves the name, symbol, and decimals for a given token address
func FetchTokenDetails(tokenAddress common.Address) (*TokenInfo, error) {
	// Check if the token details are already cached
	if info, exists := TokenCache[tokenAddress.Hex()]; exists {
		return &info, nil
	}

	// Define the ERC-20 ABI
	erc20ABI, err := abi.JSON(strings.NewReader(`[{"constant":true,"inputs":[],"name":"name","outputs":[{"name":"","type":"string"}],"payable":false,"stateMutability":"view","type":"function"},{"constant":true,"inputs":[],"name":"symbol","outputs":[{"name":"","type":"string"}],"payable":false,"stateMutability":"view","type":"function"},{"constant":true,"inputs":[],"name":"decimals","outputs":[{"name":"","type":"uint8"}],"payable":false,"stateMutability":"view","type":"function"}]`))
	if err != nil {
		log.Printf("Failed to parse ERC-20 ABI: %v", err)
		return nil, err
	}

	// Define the token instance
	token := common.HexToAddress(tokenAddress.String())

	// Call the token's name function
	nameCallData, _ := erc20ABI.Pack("name")
	var name string
	err = RpcClient.CallContext(context.Background(), &name, "eth_call", map[string]interface{}{
		"to":   token.Hex(),
		"data": "0x" + hex.EncodeToString(nameCallData),
	}, "latest")
	if err != nil || name == "" {
		log.Printf("Failed to fetch name for token %s: %v", tokenAddress.Hex(), err)
		return nil, fmt.Errorf("failed to fetch token name: %v", err)
	}

	// Decode the name if necessary
	name = DecodeHexStringIfNeeded(name)

	// Call the token's symbol function
	symbolCallData, _ := erc20ABI.Pack("symbol")
	var symbol string
	err = RpcClient.CallContext(context.Background(), &symbol, "eth_call", map[string]interface{}{
		"to":   token.Hex(),
		"data": "0x" + hex.EncodeToString(symbolCallData),
	}, "latest")
	if err != nil || symbol == "" {
		log.Printf("Failed to fetch symbol for token %s: %v", tokenAddress.Hex(), err)
		return nil, fmt.Errorf("failed to fetch token symbol: %v", err)
	}

	// Decode the symbol if necessary
	symbol = DecodeHexStringIfNeeded(symbol)

	// Call the token's decimals function
	decimalsCallData, _ := erc20ABI.Pack("decimals")
	var decimalsHex string
	err = RpcClient.CallContext(context.Background(), &decimalsHex, "eth_call", map[string]interface{}{
		"to":   token.Hex(),
		"data": "0x" + hex.EncodeToString(decimalsCallData),
	}, "latest")
	if err != nil || decimalsHex == "" {
		log.Printf("Failed to fetch decimals for token %s: %v", tokenAddress.Hex(), err)
		return nil, fmt.Errorf("failed to fetch token decimals: %v", err)
	}

	// Convert the decimals from hex to uint8
	decimals := new(big.Int)
	decimals.SetString(decimalsHex[2:], 16)

	// Store the fetched token details in cache
	tokenInfo := TokenInfo{
		Address:  token.Hex(),
		Name:     name,
		Symbol:   symbol,
		Decimals: uint8(decimals.Uint64()),
	}
	TokenCache[token.Hex()] = tokenInfo

	return &tokenInfo, nil
}
