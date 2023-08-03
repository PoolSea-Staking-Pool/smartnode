package state

import (
	"math/big"
	"time"

	v110rc1_rewards "github.com/RedDuck-Software/poolsea-go/legacy/v1.1.0-rc1/rewards"
	"github.com/RedDuck-Software/poolsea-go/rewards"
	"github.com/RedDuck-Software/poolsea-go/rocketpool"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/rocket-pool/smartnode/shared/services/config"
	cfgtypes "github.com/rocket-pool/smartnode/shared/types/config"
)

func GetClaimIntervalTime(cfg *config.RocketPoolConfig, index uint64, rp *rocketpool.RocketPool, opts *bind.CallOpts) (time.Duration, error) {
	switch cfg.Smartnode.Network.Value.(cfgtypes.Network) {
	case cfgtypes.Network_Prater:
		if index < 2 {
			contractAddress := cfg.Smartnode.GetPreviousRewardsPoolAddresses()["v1.1.0-rc1"][0]
			return v110rc1_rewards.GetClaimIntervalTime(rp, opts, &contractAddress)
		}
	}

	return rewards.GetClaimIntervalTime(rp, opts)
}

func GetNodeOperatorRewardsPercent(cfg *config.RocketPoolConfig, index uint64, rp *rocketpool.RocketPool, opts *bind.CallOpts) (*big.Int, error) {
	switch cfg.Smartnode.Network.Value.(cfgtypes.Network) {
	case cfgtypes.Network_Prater:
		if index < 2 {
			contractAddress := cfg.Smartnode.GetPreviousRewardsPoolAddresses()["v1.1.0-rc1"][0]
			return v110rc1_rewards.GetNodeOperatorRewardsPercent(rp, opts, &contractAddress)
		}
	}

	return rewards.GetNodeOperatorRewardsPercent(rp, opts)
}

func GetTrustedNodeOperatorRewardsPercent(cfg *config.RocketPoolConfig, index uint64, rp *rocketpool.RocketPool, opts *bind.CallOpts) (*big.Int, error) {
	switch cfg.Smartnode.Network.Value.(cfgtypes.Network) {
	case cfgtypes.Network_Prater:
		if index < 2 {
			contractAddress := cfg.Smartnode.GetPreviousRewardsPoolAddresses()["v1.1.0-rc1"][0]
			return v110rc1_rewards.GetTrustedNodeOperatorRewardsPercent(rp, opts, &contractAddress)
		}
	}

	return rewards.GetTrustedNodeOperatorRewardsPercent(rp, opts)
}

func GetProtocolDaoRewardsPercent(cfg *config.RocketPoolConfig, index uint64, rp *rocketpool.RocketPool, opts *bind.CallOpts) (*big.Int, error) {
	switch cfg.Smartnode.Network.Value.(cfgtypes.Network) {
	case cfgtypes.Network_Prater:
		if index < 2 {
			contractAddress := cfg.Smartnode.GetPreviousRewardsPoolAddresses()["v1.1.0-rc1"][0]
			return v110rc1_rewards.GetProtocolDaoRewardsPercent(rp, opts, &contractAddress)
		}
	}

	return rewards.GetProtocolDaoRewardsPercent(rp, opts)
}

func GetPendingRPLRewards(cfg *config.RocketPoolConfig, index uint64, rp *rocketpool.RocketPool, opts *bind.CallOpts) (*big.Int, error) {
	switch cfg.Smartnode.Network.Value.(cfgtypes.Network) {
	case cfgtypes.Network_Prater:
		if index < 2 {
			contractAddress := cfg.Smartnode.GetPreviousRewardsPoolAddresses()["v1.1.0-rc1"][0]
			return v110rc1_rewards.GetPendingRPLRewards(rp, opts, &contractAddress)
		}
	}

	return rewards.GetPendingRPLRewards(rp, opts)
}
