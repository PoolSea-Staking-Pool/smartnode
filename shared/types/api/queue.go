package api

import (
	"math/big"

	"github.com/RedDuck-Software/poolsea-go/rocketpool"
	"github.com/ethereum/go-ethereum/common"
)

type QueueStatusResponse struct {
	Status                string   `json:"status"`
	Error                 string   `json:"error"`
	DepositPoolBalance    *big.Int `json:"depositPoolBalance"`
	MinipoolQueueLength   uint64   `json:"minipoolQueueLength"`
	MinipoolQueueCapacity *big.Int `json:"minipoolQueueCapacity"`
}

type CanProcessQueueResponse struct {
	Status                     string             `json:"status"`
	Error                      string             `json:"error"`
	CanProcess                 bool               `json:"canProcess"`
	AssignDepositsDisabled     bool               `json:"assignDepositsDisabled"`
	NoMinipoolsAvailable       bool               `json:"noMinipoolsAvailable"`
	InsufficientDepositBalance bool               `json:"insufficientDepositBalance"`
	IsAtlasDeployed            bool               `json:"isAtlasDeployed"`
	GasInfo                    rocketpool.GasInfo `json:"gasInfo"`
}
type ProcessQueueResponse struct {
	Status string      `json:"status"`
	Error  string      `json:"error"`
	TxHash common.Hash `json:"txHash"`
}
