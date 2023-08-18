package network

import (
	"github.com/Seb369888/smartnode/shared/services"
	"github.com/Seb369888/smartnode/shared/services/state"
	"github.com/Seb369888/smartnode/shared/types/api"
	"github.com/urfave/cli"
)

func isAtlasDeployed(c *cli.Context) (*api.IsAtlasDeployedResponse, error) {

	// Get services
	if err := services.RequireRocketStorage(c); err != nil {
		return nil, err
	}
	rp, err := services.GetRocketPool(c)
	if err != nil {
		return nil, err
	}

	// Response
	response := api.IsAtlasDeployedResponse{}

	isAtlasDeployed, err := state.IsAtlasDeployed(rp, nil)
	if err != nil {
		return nil, err
	}
	response.IsAtlasDeployed = isAtlasDeployed

	// Return response
	return &response, nil

}
