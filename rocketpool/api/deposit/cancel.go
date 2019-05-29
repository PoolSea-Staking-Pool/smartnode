package deposit

import (
    "context"
    "errors"
    "fmt"

    "github.com/ethereum/go-ethereum/accounts/abi/bind"
    "gopkg.in/urfave/cli.v1"

    "github.com/rocket-pool/smartnode-cli/rocketpool/services"
)


// Cancel a node deposit reservation
func cancelDeposit(c *cli.Context) error {

    // Initialise services
    p, err := services.NewProvider(c, services.ProviderOpts{
        AM: true,
        NodeContract: true,
        LoadContracts: []string{"rocketNodeAPI"},
        LoadAbis: []string{"rocketNodeContract"},
    })
    if err != nil {
        return err 
    }

    // Check node has current deposit reservation
    hasReservation := new(bool)
    if err := p.NodeContract.Call(nil, hasReservation, "getHasDepositReservation"); err != nil {
        return errors.New("Error retrieving deposit reservation status: " + err.Error())
    } else if !*hasReservation {
        fmt.Println("Node does not have a current deposit reservation")
        return nil
    }

    // Cancel deposit reservation
    if txor, err := p.AM.GetNodeAccountTransactor(); err != nil {
        return err
    } else {
        if tx, err := p.NodeContract.Transact(txor, "depositReserveCancel"); err != nil {
            return errors.New("Error canceling deposit reservation: " + err.Error())
        } else {

            // Wait for transaction to be mined before continuing
            fmt.Println("Deposit reservation cancellation transaction awaiting mining...")
            bind.WaitMined(context.Background(), p.Client, tx)

        }
    }

    // Log & return
    fmt.Println("Deposit reservation cancelled successfully")
    return nil

}

