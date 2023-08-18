package services

import (
	"context"

	"github.com/Seb369888/poolsea-go/rocketpool"
)

func GetEthClientLatestBlockTimestamp(ec rocketpool.ExecutionClient) (uint64, error) {
	// Get latest block
	header, err := ec.HeaderByNumber(context.Background(), nil)
	if err != nil {
		return 0, err
	}

	// Return block timestamp
	return header.Time, nil
}
