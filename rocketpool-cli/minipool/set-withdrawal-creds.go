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

func setWithdrawalCreds(c *cli.Context, minipoolAddress common.Address) error {

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
		fmt.Println("You cannot change a solo validator's withdrawal credentials to a minipool address until Atlas has been deployed.")
		return nil
	}

	fmt.Printf("This will convert the withdrawal credentials for minipool %s's validator from the old 0x00 (BLS) value to the minipool address. This is meant for solo validator conversion **only**.\n\n", minipoolAddress.Hex())

	// Get the mnemonic
	mnemonic := ""
	if c.IsSet("mnemonic") {
		mnemonic = c.String("mnemonic")
	} else {
		mnemonic = wallet.PromptMnemonic()
	}

	success := migration.ChangeWithdrawalCreds(rp, minipoolAddress, mnemonic)
	if !success {
		fmt.Println("Your withdrawal credentials cannot be automatically changed at this time. Import aborted.\nYou can try again later by using `Poolsea minipool set-withdrawal-creds`.")
	}

	return nil
}
