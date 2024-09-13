package mempool

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// Contract represents a contract's address and ABI
type Contract struct {
	Name    string          `json:"name"`
	Address string          `json:"address"`
	ABI     json.RawMessage `json:"abi"` // Use json.RawMessage to handle the ABI as a raw JSON object
}

// LoadContracts loads the contracts from a JSON file
func LoadContracts(filename string) ([]Contract, error) {
	// Open the JSON file
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file: %w", err)
	}
	defer file.Close()

	// Read the file content
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse the JSON content
	var contracts []Contract
	if err := json.Unmarshal(data, &contracts); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	return contracts, nil
}
