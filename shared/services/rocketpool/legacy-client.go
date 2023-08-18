package rocketpool

import (
	"fmt"
	"os"

	"github.com/Seb369888/smartnode/shared/services/config"
	"github.com/alessio/shellescape"
	"github.com/mitchellh/go-homedir"
)

// Config
const (
	LegacyGlobalConfigFile    = "config.yml"
	LegacyUserConfigFile      = "settings.yml"
	LegacyComposeFile         = "docker-compose.yml"
	LegacyMetricsComposeFile  = "docker-compose-metrics.yml"
	LegacyFallbackComposeFile = "docker-compose-fallback.yml"
)

// Load the global config
func (c *Client) LoadGlobalConfig_Legacy(globalConfigPath string) (config.LegacyRocketPoolConfig, error) {
	return c.loadConfig_Legacy(globalConfigPath)
}

// Load/save the user config
func (c *Client) LoadUserConfig_Legacy(userConfigPath string) (config.LegacyRocketPoolConfig, error) {
	return c.loadConfig_Legacy(userConfigPath)
}

// Load the merged global & user config
func (c *Client) LoadMergedConfig_Legacy(globalConfigPath string, userConfigPath string) (config.LegacyRocketPoolConfig, error) {
	globalConfig, err := c.LoadGlobalConfig_Legacy(globalConfigPath)
	if err != nil {
		return config.LegacyRocketPoolConfig{}, err
	}
	userConfig, err := c.LoadUserConfig_Legacy(userConfigPath)
	if err != nil {
		return config.LegacyRocketPoolConfig{}, err
	}
	return config.Merge(&globalConfig, &userConfig)
}

// Load a config file
func (c *Client) loadConfig_Legacy(path string) (config.LegacyRocketPoolConfig, error) {
	expandedPath, err := homedir.Expand(path)
	if err != nil {
		return config.LegacyRocketPoolConfig{}, err
	}
	configBytes, err := os.ReadFile(expandedPath)
	if err != nil {
		return config.LegacyRocketPoolConfig{}, fmt.Errorf("Could not read poolsea Pool config at %s: %w", shellescape.Quote(path), err)
	}
	return config.Parse(configBytes)
}
