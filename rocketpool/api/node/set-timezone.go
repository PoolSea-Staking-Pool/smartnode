package node

import (
	"fmt"
	_ "time/tzdata"

	"github.com/Seb369888/poolsea-go/node"
	"github.com/urfave/cli"

	"github.com/Seb369888/smartnode/shared/services"
	"github.com/Seb369888/smartnode/shared/types/api"
	"github.com/Seb369888/smartnode/shared/utils/eth1"
)

func canSetTimezoneLocation(c *cli.Context, timezoneLocation string) (*api.CanSetNodeTimezoneResponse, error) {

	// Get services
	if err := services.RequireNodeRegistered(c); err != nil {
		return nil, err
	}
	w, err := services.GetWallet(c)
	if err != nil {
		return nil, err
	}
	rp, err := services.GetRocketPool(c)
	if err != nil {
		return nil, err
	}

	// Response
	response := api.CanSetNodeTimezoneResponse{}

	// Get gas estimate
	opts, err := w.GetNodeAccountTransactor()
	if err != nil {
		return nil, err
	}
	gasInfo, err := node.EstimateSetTimezoneLocationGas(rp, timezoneLocation, opts)
	if err != nil {
		return nil, err
	}
	response.GasInfo = gasInfo
	response.CanSet = true
	return &response, nil

}

func setTimezoneLocation(c *cli.Context, timezoneLocation string) (*api.SetNodeTimezoneResponse, error) {

	// Get services
	if err := services.RequireNodeRegistered(c); err != nil {
		return nil, err
	}
	w, err := services.GetWallet(c)
	if err != nil {
		return nil, err
	}
	rp, err := services.GetRocketPool(c)
	if err != nil {
		return nil, err
	}

	// Response
	response := api.SetNodeTimezoneResponse{}

	// Get transactor
	opts, err := w.GetNodeAccountTransactor()
	if err != nil {
		return nil, err
	}

	// Override the provided pending TX if requested
	err = eth1.CheckForNonceOverride(c, opts)
	if err != nil {
		return nil, fmt.Errorf("Error checking for nonce override: %w", err)
	}

	// Set timezone location
	hash, err := node.SetTimezoneLocation(rp, timezoneLocation, opts)
	if err != nil {
		return nil, err
	}
	response.TxHash = hash

	// Return response
	return &response, nil

}
