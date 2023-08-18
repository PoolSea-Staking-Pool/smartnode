package service

import (
	"github.com/urfave/cli"

	"github.com/Seb369888/smartnode/shared/services"
	"github.com/Seb369888/smartnode/shared/types/api"
)

// Gets the status of the configured Execution clients
func getClientStatus(c *cli.Context) (*api.ClientStatusResponse, error) {

	// Get services
	ec, err := services.GetEthClient(c)
	if err != nil {
		return nil, err
	}
	bc, err := services.GetBeaconClient(c)
	if err != nil {
		return nil, err
	}

	cfg, err := services.GetConfig(c)
	if err != nil {
		return nil, err
	}

	// Response
	response := api.ClientStatusResponse{}

	// Get the EC manager status
	ecMgrStatus := ec.CheckStatus(cfg)
	response.EcManagerStatus = *ecMgrStatus

	// Get the BC manager status
	bcMgrStatus := bc.CheckStatus()
	response.BcManagerStatus = *bcMgrStatus

	// Return response
	return &response, nil

}
