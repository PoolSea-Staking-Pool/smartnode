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
		"inputs": [{
			"internalType": "uint32[]",
			"name": "secondsAgos",
			"type": "uint32[]"
		}],
		"name": "observe",
		"outputs": [{
			"internalType": "int56[]",
			"name": "tickCumulatives",
			"type": "int56[]"
		}, {
			"internalType": "uint160[]",
			"name": "secondsPerLiquidityCumulativeX128s",
			"type": "uint160[]"
		}],
		"stateMutability": "view",
		"type": "function"
		}
	]`
)

// Settings
const (
	SubmissionKey string = "network.prices.submitted.node.key"
	BlocksPerTurn uint64 = 75 // Approx. 15 minutes

	twapNumberOfSeconds uint32 = 60 * 60 * 12 // 12 hours
)

type poolObserveResponse struct {
	TickCumulatives                    []*big.Int `abi:"tickCumulatives"`
	SecondsPerLiquidityCumulativeX128s []*big.Int `abi:"secondsPerLiquidityCumulativeX128s"`
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
	t.log.Println("Checking for RPL price checkpoint...")

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
		t.log.Printlnf("Getting RPL price for block %d...", blockNumber)

		// Get RPL price at block
		rplPrice, err := t.getRplTwap(blockNumber)
		if err != nil {
			t.handleError(fmt.Errorf("%s %w", logPrefix, err))
			return
		}

		// Log
		t.log.Printlnf("RPL price: %.6f ETH", mathutils.RoundDown(eth.WeiToEth(rplPrice), 6))

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
		t.log.Println("Submitting RPL price...")

		// Submit RPL price
		if err := t.submitRplPrice(blockNumber, rplPrice); err != nil {
			t.handleError(fmt.Errorf("%s could not submit RPL price: %w", logPrefix, err))
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
		return nil, fmt.Errorf("RPL TWAP pool contract not deployed on this network")
	}

	// Get a client with the block number available
	client, err := eth1.GetBestApiClient(t.rp, t.cfg, t.printMessage, opts.BlockNumber)
	if err != nil {
		return nil, err
	}

	// Construct the pool contract instance
	parsed, err := abi.JSON(strings.NewReader(RplTwapPoolAbi))
	if err != nil {
		return nil, fmt.Errorf("error decoding RPL TWAP pool ABI: %w", err)
	}
	addr := common.HexToAddress(poolAddress)
	poolContract := bind.NewBoundContract(addr, parsed, client.Client, client.Client, client.Client)
	pool := rocketpool.Contract{
		Contract: poolContract,
		Address:  &addr,
		ABI:      &parsed,
		Client:   client.Client,
	}

	// Get RPL price
	response := poolObserveResponse{}
	interval := twapNumberOfSeconds
	args := []uint32{interval, 0}

	err = pool.Call(opts, &response, "observe", args)
	if err != nil {
		return nil, fmt.Errorf("could not get RPL price at block %d: %w", blockNumber, err)
	}
	if len(response.TickCumulatives) < 2 {
		return nil, fmt.Errorf("TWAP contract didn't have enough tick cumulatives for block %d (raw: %v)", blockNumber, response.TickCumulatives)
	}

	tick := big.NewInt(0).Sub(response.TickCumulatives[1], response.TickCumulatives[0])
	tick.Div(tick, big.NewInt(int64(interval))) // tick = (cumulative[1] - cumulative[0]) / interval

	base := eth.EthToWei(1.0001) // 1.0001e18
	one := eth.EthToWei(1)       // 1e18

	numerator := big.NewInt(0).Exp(base, tick, nil) // 1.0001e18 ^ tick
	numerator.Mul(numerator, one)

	denominator := big.NewInt(0).Exp(one, tick, nil) // 1e18 ^ tick
	denominator.Div(numerator, denominator)          // denominator = (1.0001e18^tick / 1e18^tick)

	numerator.Mul(one, one)                               // 1e18 ^ 2
	rplPrice := big.NewInt(0).Div(numerator, denominator) // 1e18 ^ 2 / (1.0001e18^tick * 1e18 / 1e18^tick)

	// Return
	return rplPrice, nil

}

func (t *submitRplPrice) printMessage(message string) {
	t.log.Println(message)
}

// Submit RPL price and total effective RPL stake
func (t *submitRplPrice) submitRplPrice(blockNumber uint64, rplPrice *big.Int) error {

	// Log
	t.log.Printlnf("Submitting RPL price for block %d...", blockNumber)

	// Get transactor
	opts, err := t.w.GetNodeAccountTransactor()
	if err != nil {
		return err
	}

	// Get the gas limit
	gasInfo, err := network.EstimateSubmitPricesGas(t.rp, blockNumber, rplPrice, opts)
	if err != nil {
		return fmt.Errorf("Could not estimate the gas required to submit RPL price: %w", err)
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
	t.log.Printlnf("Successfully submitted RPL price for block %d.", blockNumber)

	// Return
	return nil
}
