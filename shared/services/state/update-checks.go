package state

import (
	"github.com/Seb369888/poolsea-go/rocketpool"
	"github.com/Seb369888/poolsea-go/utils"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/hashicorp/go-version"
)

// Check if Redstone has been deployed
func IsRedstoneDeployed(rp *rocketpool.RocketPool, opts *bind.CallOpts) (bool, error) {
	currentVersion, err := utils.GetCurrentVersion(rp, opts)
	if err != nil {
		return false, err
	}

	constraint, _ := version.NewConstraint(">= 1.1.0")
	return constraint.Check(currentVersion), nil
}

// Check if Atlas has been deployed
func IsAtlasDeployed(rp *rocketpool.RocketPool, opts *bind.CallOpts) (bool, error) {
	currentVersion, err := utils.GetCurrentVersion(rp, opts)
	if err != nil {
		return false, err
	}

	constraint, _ := version.NewConstraint(">= 1.2.0")
	return constraint.Check(currentVersion), nil
}
