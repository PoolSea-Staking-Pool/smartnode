package wallet

import (
    "errors"

    "github.com/urfave/cli"

    "github.com/rocket-pool/smartnode/shared/services"
    "github.com/rocket-pool/smartnode/shared/types/api"
)


func initWallet(c *cli.Context) (*api.InitWalletResponse, error) {

    // Get services
    if err := services.RequireNodePassword(c); err != nil { return nil, err }
    w, err := services.GetWallet(c)
    if err != nil { return nil, err }

    // Response
    response := api.InitWalletResponse{}

    // Check if wallet is already initialized
    if w.IsInitialized() {
        return nil, errors.New("The wallet is already initialized")
    }

    // Initialize wallet
    mnemonic, err := w.Initialize()
    if err != nil {
        return nil, err
    }
    response.Mnemonic = mnemonic

    // Save wallet
    if err := w.Save(); err != nil {
        return nil, err
    }

    // Return response
    return &response, nil

}

