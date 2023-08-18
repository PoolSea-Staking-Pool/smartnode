package minipool

import (
	"fmt"

	"github.com/Seb369888/smartnode/rocketpool-cli/wallet"
	"github.com/Seb369888/smartnode/shared/services/rocketpool"
	cliutils "github.com/Seb369888/smartnode/shared/utils/cli"
	"github.com/Seb369888/smartnode/shared/utils/cli/migration"
	"github.com/ethereum/go-ethereum/common"
	"github.com/urfave/cli"
)

func importKey(c *cli.Context, minipoolAddress common.Address) error {

	// Get RP client
	rp, err := rocketpool.NewClientFromCtx(c)
	if err != nil {
		return err
	}
	defer rp.Close()

	// Check and assign the EC status
	err = cliutils.CheckClientStatus(rp)
	if err != nil {
		return err
	}

	// Check for Atlas
	atlasResponse, err := rp.IsAtlasDeployed()
	if err != nil {
		return fmt.Errorf("error checking if Atlas has been deployed: %w", err)
	}
	if !atlasResponse.IsAtlasDeployed {
		fmt.Println("You cannot import a validator key as part of solo validator migration until Atlas has been deployed.")
		return nil
	}

	fmt.Printf("This will allow you to import the externally-created private key for the validator associated with minipool %s so it can be managed by the Smartnode's Validator Client instead of your externally-managed Validator Client.\n\n", minipoolAddress.Hex())

	// Get the mnemonic
	mnemonic := ""
	if c.IsSet("mnemonic") {
		mnemonic = c.String("mnemonic")
	} else {
		mnemonic = wallet.PromptMnemonic()
	}

	success := migration.ImportKey(c, rp, minipoolAddress, mnemonic)
	if !success {
		fmt.Println("Importing the key failed.\nYou can try again later by using `Poolsea minipool import-key`.")
	}

	return nil
}
