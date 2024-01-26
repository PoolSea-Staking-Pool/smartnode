package watchtower

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"

	"github.com/Seb369888/poolsea-go/network"
	"github.com/Seb369888/poolsea-go/rocketpool"
	"github.com/Seb369888/poolsea-go/utils/eth"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/urfave/cli"

	"github.com/Seb369888/smartnode/shared/services"
	"github.com/Seb369888/smartnode/shared/services/beacon"
	"github.com/Seb369888/smartnode/shared/services/config"
	"github.com/Seb369888/smartnode/shared/services/state"
	"github.com/Seb369888/smartnode/shared/services/wallet"
	"github.com/Seb369888/smartnode/shared/utils/api"
	"github.com/Seb369888/smartnode/shared/utils/eth1"
	"github.com/Seb369888/smartnode/shared/utils/log"
	mathutils "github.com/Seb369888/smartnode/shared/utils/math"
)

const (
	RplTwapPoolAbi string = `[
  {
    "constant": true,
    "inputs": [],
    "name": "getReserves",
    "outputs": [
      {
        "internalType": "uint112",
        "name": "_reserve0",
        "type": "uint112"
      },
      {
        "internalType": "uint112",
        "name": "_reserve1",
        "type": "uint112"
      },
      {
        "internalType": "uint32",
        "name": "_blockTimestampLast",
        "type": "uint32"
      }
    ],
    "payable": false,
    "stateMutability": "view",
    "type": "function"
  },
  {
    "constant": true,
    "inputs": [],
    "name": "price0CumulativeLast",
    "outputs": [
      {
        "internalType": "uint256",
        "name": "",
        "type": "uint256"
      }
    ],
    "payable": false,
    "stateMutability": "view",
    "type": "function"
  }
]`
)

// Settings
const (
	SubmissionKey string = "network.prices.submitted.node.key"
)

type poolReservesResponse struct {
	Reserve0           *big.Int `abi:"_reserve0"`
	Reserve1           *big.Int `abi:"_reserve1"`
	BlockTimestampLast *big.Int `abi:"_blockTimestampLast"`
}

// Submit RPL price task
type submitRplPrice struct {
	c         *cli.Context
	log       log.ColorLogger
	errLog    log.ColorLogger
	cfg       *config.RocketPoolConfig
	ec        rocketpool.ExecutionClient
	w         *wallet.Wallet
	rp        *rocketpool.RocketPool
	bc        beacon.Client
	lock      *sync.Mutex
	isRunning bool
}

// UQ112x112 represents a fixed-point number with a resolution of 1 / 2^112
type UQ112x112 struct {
	value *big.Int
}

// NewUQ112x112 creates a new UQ112x112 instance from a big.Int value
func NewUQ112x112(y *big.Int) *UQ112x112 {
	value := new(big.Int).Set(y)
	value = value.Lsh(value, 112) // y * 2^112
	return &UQ112x112{value}
}

// UQdiv divides a UQ112x112 by a uint64, returning a new UQ112x112
func (uq *UQ112x112) UQdiv(y *big.Int) *UQ112x112 {
	result := new(big.Int).Set(uq.value)
	result = result.Div(result, y)
	return &UQ112x112{result}
}

// Create submit RPL price task
func newSubmitRplPrice(c *cli.Context, logger log.ColorLogger, errorLogger log.ColorLogger) (*submitRplPrice, error) {

	// Get services
	cfg, err := services.GetConfig(c)
	if err != nil {
		return nil, err
	}
	w, err := services.GetWallet(c)
	if err != nil {
		return nil, err
	}
	ec, err := services.GetEthClient(c)
	if err != nil {
		return nil, err
	}
	rp, err := services.GetRocketPool(c)
	if err != nil {
		return nil, err
	}
	bc, err := services.GetBeaconClient(c)
	if err != nil {
		return nil, err
	}

	// Return task
	lock := &sync.Mutex{}
	return &submitRplPrice{
		c:      c,
		log:    logger,
		errLog: errorLogger,
		cfg:    cfg,
		ec:     ec,
		w:      w,
		rp:     rp,
		bc:     bc,
		lock:   lock,
	}, nil

}

// Submit RPL price
func (t *submitRplPrice) run(state *state.NetworkState) error {

	// Wait for eth client to sync
	if err := services.WaitEthClientSynced(t.c, true); err != nil {
		return err
	}

	// Get node account
	nodeAccount, err := t.w.GetNodeAccount()
	if err != nil {
		return err
	}

	// Check if submission is enabled
	if !state.NetworkDetails.SubmitPricesEnabled {
		return nil
	}

	// Log
	t.log.Println("Checking for POOL price checkpoint...")

	// Get block to submit price for
	blockNumber := state.NetworkDetails.LatestReportablePricesBlock

	// Check if a submission needs to be made
	pricesBlock := state.NetworkDetails.PricesBlock
	if blockNumber <= pricesBlock {
		return nil
	}

	// Get the time of the block
	header, err := t.ec.HeaderByNumber(context.Background(), big.NewInt(0).SetUint64(blockNumber))
	if err != nil {
		return err
	}
	blockTime := time.Unix(int64(header.Time), 0)

	// Get the Beacon block corresponding to this time
	eth2Config := state.BeaconConfig
	genesisTime := time.Unix(int64(eth2Config.GenesisTime), 0)
	timeSinceGenesis := blockTime.Sub(genesisTime)
	slotNumber := uint64(timeSinceGenesis.Seconds()) / eth2Config.SecondsPerSlot

	// Check if the targetEpoch is finalized yet
	targetEpoch := slotNumber / eth2Config.SlotsPerEpoch
	beaconHead, err := t.bc.GetBeaconHead()
	if err != nil {
		return err
	}
	finalizedEpoch := beaconHead.FinalizedEpoch
	if targetEpoch > finalizedEpoch {
		t.log.Printlnf("Prices must be reported for EL block %d, waiting until Epoch %d is finalized (currently %d)", blockNumber, targetEpoch, finalizedEpoch)
		return nil
	}

	// Check if the process is already running
	t.lock.Lock()
	if t.isRunning {
		t.log.Println("Prices report is already running in the background.")
		t.lock.Unlock()
		return nil
	}
	t.lock.Unlock()

	go func() {
		t.lock.Lock()
		t.isRunning = true
		t.lock.Unlock()
		logPrefix := "[Price Report]"
		t.log.Printlnf("%s Starting price report in a separate thread.", logPrefix)

		// Log
		t.log.Printlnf("Getting POOL price for block %d...", blockNumber)

		// Get RPL price at block
		rplPrice, err := t.getRplTwap(blockNumber)
		if err != nil {
			t.handleError(fmt.Errorf("%s %w", logPrefix, err))
			return
		}

		// Log
		t.log.Printlnf("POOL price: %.6f PLS", mathutils.RoundDown(eth.WeiToEth(rplPrice), 6))

		// Check if we have reported these specific values before
		hasSubmittedSpecific, err := t.hasSubmittedSpecificBlockPrices(nodeAccount.Address, blockNumber, rplPrice)
		if err != nil {
			t.handleError(fmt.Errorf("%s %w", logPrefix, err))
			return
		}
		if hasSubmittedSpecific {
			t.lock.Lock()
			t.isRunning = false
			t.lock.Unlock()
			return
		}

		// We haven't submitted these values, check if we've submitted any for this block so we can log it
		hasSubmitted, err := t.hasSubmittedBlockPrices(nodeAccount.Address, blockNumber)
		if err != nil {
			t.handleError(fmt.Errorf("%s %w", logPrefix, err))
			return
		}
		if hasSubmitted {
			t.log.Printlnf("Have previously submitted out-of-date prices for block %d, trying again...", blockNumber)
		}

		// Log
		t.log.Println("Submitting POOL price...")

		// Submit RPL price
		if err := t.submitRplPrice(blockNumber, rplPrice); err != nil {
			t.handleError(fmt.Errorf("%s could not submit POOL price: %w", logPrefix, err))
			return
		}

		// Log and return
		t.log.Printlnf("%s Price report complete.", logPrefix)
		t.lock.Lock()
		t.isRunning = false
		t.lock.Unlock()
	}()

	// Return
	return nil

}

func (t *submitRplPrice) handleError(err error) {
	t.errLog.Println(err)
	t.errLog.Println("*** Price report failed. ***")
	t.lock.Lock()
	t.isRunning = false
	t.lock.Unlock()
}

// Check whether prices for a block has already been submitted by the node
func (t *submitRplPrice) hasSubmittedBlockPrices(nodeAddress common.Address, blockNumber uint64) (bool, error) {

	blockNumberBuf := make([]byte, 32)
	big.NewInt(int64(blockNumber)).FillBytes(blockNumberBuf)
	return t.rp.RocketStorage.GetBool(nil, crypto.Keccak256Hash([]byte(SubmissionKey), nodeAddress.Bytes(), blockNumberBuf))

}

// Check whether specific prices for a block has already been submitted by the node
func (t *submitRplPrice) hasSubmittedSpecificBlockPrices(nodeAddress common.Address, blockNumber uint64, rplPrice *big.Int) (bool, error) {
	blockNumberBuf := make([]byte, 32)
	big.NewInt(int64(blockNumber)).FillBytes(blockNumberBuf)

	rplPriceBuf := make([]byte, 32)
	rplPrice.FillBytes(rplPriceBuf)

	return t.rp.RocketStorage.GetBool(nil, crypto.Keccak256Hash([]byte(SubmissionKey), nodeAddress.Bytes(), blockNumberBuf, rplPriceBuf))
}

// Get RPL price via TWAP at block
func (t *submitRplPrice) getRplTwap(blockNumber uint64) (*big.Int, error) {

	// Initialize call options
	opts := &bind.CallOpts{
		BlockNumber: big.NewInt(int64(blockNumber)),
	}

	poolAddress := t.cfg.Smartnode.GetRplTwapPoolAddress()
	if poolAddress == "" {
		return nil, fmt.Errorf("POOL TWAP pool contract not deployed on this network")
	}

	//---------- LAST BLOCK DATA CALCULATION ---------

	// Get a client with the block number available
	client, err := eth1.GetBestApiClient(t.rp, t.cfg, t.printMessage, opts.BlockNumber)
	if err != nil {
		return nil, err
	}

	// Construct the pool contract instance
	parsed, err := abi.JSON(strings.NewReader(RplTwapPoolAbi))
	if err != nil {
		return nil, fmt.Errorf("error decoding POOL TWAP pool ABI: %w", err)
	}
	addr := common.HexToAddress(poolAddress)
	poolContract := bind.NewBoundContract(addr, parsed, client.Client, client.Client, client.Client)
	pool := rocketpool.Contract{
		Contract: poolContract,
		Address:  &addr,
		ABI:      &parsed,
		Client:   client.Client,
	}

	// Getting pool reserves and last update timestamp
	reservesResponse := poolReservesResponse{}
	err = pool.Call(opts, &reservesResponse, "getReserves")
	if err != nil {
		return nil, fmt.Errorf("could not get pool reserves at block %d: %w", blockNumber, err)
	}

	// Getting last cumulative POOL price in WPLS
	priceCumulativeLast := big.NewInt(0)
	err = pool.Call(opts, &priceCumulativeLast, "price0CumulativeLast")
	if err != nil {
		return nil, fmt.Errorf("could not get last cumulative POOL price at block %d: %w", blockNumber, err)
	}

	header, err := client.Client.HeaderByNumber(context.Background(), opts.BlockNumber)
	if err != nil {
		return nil, fmt.Errorf("could not get header for block %d: %w", blockNumber, err)
	}

	// calculating latest cumulative price if it needs
	blockTimestamp := big.NewInt(int64(header.Time))
	timeElapsed := big.NewInt(0).Sub(blockTimestamp, reservesResponse.BlockTimestampLast)
	if timeElapsed.Cmp(big.NewInt(0)) == 1 {
		uqReserve1 := NewUQ112x112(reservesResponse.Reserve1)
		uqDivResult := uqReserve1.UQdiv(reservesResponse.Reserve0)
		multipliedResult := new(big.Int).Mul(uqDivResult.value, timeElapsed)

		priceCumulativeLast = new(big.Int).Add(multipliedResult, priceCumulativeLast)
	}

	//---------- GETTING CUMULATIVE PRICE AND TS ON LAST SUBMISSION BLOCK ---------

	lastSubmissionBlock, err := network.GetPricesBlock(t.rp, opts)
	if err != nil {
		return nil, err
	}
	optsHistorical := &bind.CallOpts{
		BlockNumber: big.NewInt(int64(lastSubmissionBlock)),
	}

	reservesHistoricalResponse := poolReservesResponse{}
	err = pool.Call(optsHistorical, &reservesHistoricalResponse, "getReserves")
	if err != nil {
		return nil, fmt.Errorf("could not get historical pool reserves at block %d: %w", blockNumber, err)
	}

	priceCumulativeLastHistorical := big.NewInt(0)
	err = pool.Call(optsHistorical, &priceCumulativeLastHistorical, "price0CumulativeLast")
	if err != nil {
		return nil, fmt.Errorf("could not get historical last cumulative POOL price at block %d: %w", blockNumber, err)
	}

	//----------- CALCULATING TWAP POOL PRICE ------------
	// twap = (cum2 - cum1) / (time2 - time1)

	cumulativePricesDif := new(big.Int).Sub(priceCumulativeLast, priceCumulativeLastHistorical)
	timestampsDif := new(big.Int).Sub(blockTimestamp, reservesHistoricalResponse.BlockTimestampLast)
	twapPrice := new(big.Int).Div(cumulativePricesDif, timestampsDif)

	return twapPrice, nil

}

func (t *submitRplPrice) printMessage(message string) {
	t.log.Println(message)
}

// Submit RPL price and total effective RPL stake
func (t *submitRplPrice) submitRplPrice(blockNumber uint64, rplPrice *big.Int) error {

	// Log
	t.log.Printlnf("Submitting POOL price for block %d...", blockNumber)

	// Get transactor
	opts, err := t.w.GetNodeAccountTransactor()
	if err != nil {
		return err
	}

	// Get the gas limit
	gasInfo, err := network.EstimateSubmitPricesGas(t.rp, blockNumber, rplPrice, opts)
	if err != nil {
		return fmt.Errorf("Could not estimate the gas required to submit POOL price: %w", err)
	}

	// Print the gas info
	maxFee := eth.GweiToWei(getWatchtowerMaxFee(t.cfg))
	if !api.PrintAndCheckGasInfo(gasInfo, false, 0, t.log, maxFee, 0) {
		return nil
	}

	// Set the gas settings
	opts.GasFeeCap = maxFee
	opts.GasTipCap = eth.GweiToWei(getWatchtowerPrioFee(t.cfg))
	opts.GasLimit = gasInfo.SafeGasLimit

	// Submit RPL price
	hash, err := network.SubmitPrices(t.rp, blockNumber, rplPrice, opts)
	if err != nil {
		return err
	}

	// Print TX info and wait for it to be included in a block
	err = api.PrintAndWaitForTransaction(t.cfg, hash, t.rp.Client, t.log)
	if err != nil {
		return err
	}

	// Log
	t.log.Printlnf("Successfully submitted POOL price for block %d.", blockNumber)

	// Return
	return nil
}
