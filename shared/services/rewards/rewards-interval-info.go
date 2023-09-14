package rewards

import (
	"fmt"

	cfgtypes "github.com/Seb369888/smartnode/shared/types/config"
)

type rewardsIntervalInfo struct {
	rewardsRulesetVersion uint64
	mainnetStartInterval  uint64
	praterStartInterval   uint64
	generator             treeGeneratorImpl
}

func (r *rewardsIntervalInfo) GetStartInterval(network cfgtypes.Network) (uint64, error) {
	switch network {
	case cfgtypes.Network_Mainnet:
		return r.mainnetStartInterval, nil
	case cfgtypes.Network_Prater:
		return r.praterStartInterval, nil
	case cfgtypes.Network_Devnet:
		return 0, nil
	case cfgtypes.Network_PulseV4:
		return 0, nil
	case cfgtypes.Network_Pulsechain:
		return 0, nil
	default:
		return 0, fmt.Errorf("unknown network: %s", string(network))
	}
}
