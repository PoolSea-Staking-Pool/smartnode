package addons

import (
	cfgtypes "github.com/Seb369888/smartnode/shared/types/config"
)

// Interface for Smartnode addons
type SmartnodeAddon interface {
	GetName() string
	GetDescription() string
	GetConfig() cfgtypes.Config
	GetContainerName() string
	GetContainerTag() string
	GetEnabledParameter() *cfgtypes.Parameter
	UpdateEnvVars(envVars map[string]string) error
}
