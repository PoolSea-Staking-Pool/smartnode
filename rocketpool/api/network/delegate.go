package network

import (
	"github.com/Seb369888/smartnode/shared/services"
	"github.com/Seb369888/smartnode/shared/types/api"
	"github.com/urfave/cli"
)

// Get the latest delegate contract address
func getLatestDelegate(c *cli.Context) (*api.GetLatestDelegateResponse, error) {

	// Get services
	rp, err := services.GetRocketPool(c)
	if err != nil {
		return nil, err
	}

	// Response
	response := api.GetLatestDelegateResponse{}

	// Get latest delegate address
	latestDelegateAddress, err := rp.GetAddress("poolseaMinipoolDelegate", nil)
	if err != nil {
		return nil, err
	}
	response.Address = *latestDelegateAddress

	// Return response
	return &response, nil

}
