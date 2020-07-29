package watchtower

import (
    "context"
    "log"
    "math/big"
    "time"

    "github.com/ethereum/go-ethereum/accounts/abi/bind"
    "github.com/ethereum/go-ethereum/common"
    "github.com/ethereum/go-ethereum/crypto"
    "github.com/rocket-pool/rocketpool-go/deposit"
    "github.com/rocket-pool/rocketpool-go/network"
    "github.com/rocket-pool/rocketpool-go/node"
    "github.com/rocket-pool/rocketpool-go/rocketpool"
    "github.com/rocket-pool/rocketpool-go/settings"
    "github.com/rocket-pool/rocketpool-go/tokens"
    "github.com/rocket-pool/rocketpool-go/utils/eth"
    "github.com/urfave/cli"
    "golang.org/x/sync/errgroup"

    "github.com/rocket-pool/smartnode/shared/services"
    "github.com/rocket-pool/smartnode/shared/services/accounts"
)


// Settings
var submitNetworkBalancesInterval, _ = time.ParseDuration("1m")


// Network balance info
type networkBalances struct {
    DepositPool *big.Int
    RETHContract *big.Int
    RETHSupply *big.Int
}


// Start submit network balances task
func startSubmitNetworkBalances(c *cli.Context) error {

    // Get services
    if err := services.WaitNodeRegistered(c, true); err != nil { return err }
    am, err := services.GetAccountManager(c)
    if err != nil { return err }
    rp, err := services.GetRocketPool(c)
    if err != nil { return err }

    // Submit network balances at interval
    go (func() {
        for {
            if err := submitNetworkBalances(c, am, rp); err != nil {
                log.Println(err)
            }
            time.Sleep(submitNetworkBalancesInterval)
        }
    })()

    // Return
    return nil

}


// Submit network balances
func submitNetworkBalances(c *cli.Context, am *accounts.AccountManager, rp *rocketpool.RocketPool) error {

    // Wait for eth client to sync
    if err := services.WaitClientSynced(c, true); err != nil {
        return err
    }

    // Get node account
    nodeAccount, err := am.GetNodeAccount()
    if err != nil {
        return err
    }

    // Check node trusted status
    nodeTrusted, err := node.GetNodeTrusted(rp, nodeAccount.Address, nil)
    if err != nil {
        return err
    }
    if !nodeTrusted {
        return nil
    }

    // Get block to submit balances for
    blockNumber, err := getLatestReportableBlock(rp)
    if err != nil {
        return err
    }

    // Check if balances for block can be submitted by node
    canSubmit, err := canSubmitBlockBalances(rp, nodeAccount.Address, blockNumber)
    if err != nil {
        return err
    }
    if !canSubmit {
        return nil
    }

    // Log
    log.Printf("Calculating network balances for block %d...\n", blockNumber)

    // Get network balances at block
    balances, err := getNetworkBalances(rp, blockNumber)
    if err != nil {
        return err
    }

    // Log
    log.Printf("Deposit pool balance: %.2f ETH\n", eth.WeiToEth(balances.DepositPool))
    log.Printf("rETH contract balance: %.2f ETH\n", eth.WeiToEth(balances.RETHContract))
    log.Printf("rETH token supply: %.2f rETH\n", eth.WeiToEth(balances.RETHSupply))

    // Return
    return nil

}


// Get the latest block number to report balances for
func getLatestReportableBlock(rp *rocketpool.RocketPool) (int64, error) {

    // Data
    var wg errgroup.Group
    var currentBlock int64
    var submitBalancesFrequency int64

    // Get current block
    wg.Go(func() error {
        header, err := rp.Client.HeaderByNumber(context.Background(), nil)
        if err == nil {
            currentBlock = header.Number.Int64()
        }
        return err
    })

    // Get balance submission frequency
    wg.Go(func() error {
        var err error
        submitBalancesFrequency, err = settings.GetSubmitBalancesFrequency(rp, nil)
        return err
    })

    // Wait for data
    if err := wg.Wait(); err != nil {
        return 0, err
    }

    // Calculate and return
    return (currentBlock / submitBalancesFrequency) * submitBalancesFrequency, nil

}


// Check whether balances for a block can be submitted by the node
func canSubmitBlockBalances(rp *rocketpool.RocketPool, nodeAddress common.Address, blockNumber int64) (bool, error) {

    // Data
    var wg errgroup.Group
    var submitBalancesEnabled bool
    var currentBalancesBlock int64
    var nodeSubmittedBlock bool

    // Get data
    wg.Go(func() error {
        var err error
        submitBalancesEnabled, err = settings.GetSubmitBalancesEnabled(rp, nil)
        return err
    })
    wg.Go(func() error {
        var err error
        currentBalancesBlock, err = network.GetBalancesBlock(rp, nil)
        return err
    })
    wg.Go(func() error {
        var err error
        nodeSubmittedBlock, err = rp.RocketStorage.GetBool(nil, crypto.Keccak256Hash([]byte("network.balances.submitted.node"), nodeAddress.Bytes(), big.NewInt(blockNumber).Bytes()))
        return err
    })

    // Wait for data
    if err := wg.Wait(); err != nil {
        return false, err
    }

    // Return
    return (submitBalancesEnabled && blockNumber > currentBalancesBlock && !nodeSubmittedBlock), nil

}


// Get the network balances at a specific block
func getNetworkBalances(rp *rocketpool.RocketPool, blockNumber int64) (networkBalances, error) {

    // Initialize call options
    opts := &bind.CallOpts{
        BlockNumber: big.NewInt(blockNumber),
    }

    // Data
    var wg errgroup.Group
    balances := networkBalances{}

    // Get deposit pool balance
    wg.Go(func() error {
        var err error
        balances.DepositPool, err = deposit.GetBalance(rp, opts)
        return err
    })

    // Get rETH contract balance
    wg.Go(func() error {
        rethContractAddress, err := rp.GetAddress("rocketETHToken")
        if err != nil {
            return err
        }
        rethContractBalance, err := rp.Client.BalanceAt(context.Background(), *rethContractAddress, big.NewInt(blockNumber))
        if err != nil {
            return err
        }
        balances.RETHContract = rethContractBalance
        return nil
    })

    // Get rETH token supply
    wg.Go(func() error {
        var err error
        balances.RETHSupply, err = tokens.GetRETHTotalSupply(rp, opts)
        return err
    })

    // Wait for data
    if err := wg.Wait(); err != nil {
        return networkBalances{}, err
    }

    // Return
    return balances, nil

}
