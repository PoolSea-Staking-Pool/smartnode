package network

import (
	"fmt"

	"github.com/urfave/cli"

	"github.com/Seb369888/smartnode/shared/services/rocketpool"
	cliutils "github.com/Seb369888/smartnode/shared/utils/cli"
)

func getNodeFee(c *cli.Context) error {

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

	// Get node fee
	response, err := rp.NodeFee()
	if err != nil {
		return err
	}

	// Print & return
	fmt.Printf("The current network node commission rate is %f%%.\n", response.NodeFee*100)
	fmt.Printf("Minimum node commission rate: %f%%\n", response.MinNodeFee*100)
	fmt.Printf("Target node commission rate:  %f%%\n", response.TargetNodeFee*100)
	fmt.Printf("Maximum node commission rate: %f%%\n", response.MaxNodeFee*100)
	return nil

}
