package queue

import (
	"fmt"

	"github.com/urfave/cli"

	"github.com/Seb369888/smartnode/shared/services/gas"
	"github.com/Seb369888/smartnode/shared/services/rocketpool"
	cliutils "github.com/Seb369888/smartnode/shared/utils/cli"
)

func processQueue(c *cli.Context) error {

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

	// Check deposit queue can be processed
	canProcess, err := rp.CanProcessQueue()
	if err != nil {
		return err
	}
	if !canProcess.CanProcess {
		fmt.Println("The deposit queue cannot be processed:")
		if canProcess.AssignDepositsDisabled {
			fmt.Println("Deposit assignments are currently disabled.")
		}
		if !canProcess.IsAtlasDeployed {
			if canProcess.NoMinipoolsAvailable {
				fmt.Println("No minipools are available for assignment.")
			}
			if canProcess.InsufficientDepositBalance {
				fmt.Println("The deposit pool has an insufficient balance for assignment.")
			}
		}
		return nil
	}

	// Assign max fees
	err = gas.AssignMaxFeeAndLimit(canProcess.GasInfo, rp, c.Bool("yes"))
	if err != nil {
		return err
	}

	// Prompt for confirmation
	if !(c.Bool("yes") || cliutils.Confirm("Do you accept this gas fee?")) {
		fmt.Println("Cancelled.")
		return nil
	}

	// Process deposit queue
	response, err := rp.ProcessQueue()
	if err != nil {
		return err
	}

	fmt.Printf("Processing queue...\n")
	cliutils.PrintTransactionHash(rp, response.TxHash)
	if _, err = rp.WaitForTransaction(response.TxHash); err != nil {
		return err
	}

	// Log & return
	fmt.Println("The deposit queue was successfully processed.")
	return nil

}
