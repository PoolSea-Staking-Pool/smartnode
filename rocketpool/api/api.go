package api

import (
	"github.com/Seb369888/smartnode/rocketpool/api/debug"
	"github.com/ethereum/go-ethereum/common"
	"github.com/urfave/cli"

	"github.com/Seb369888/poolsea-go/utils"
	"github.com/Seb369888/smartnode/rocketpool/api/auction"
	"github.com/Seb369888/smartnode/rocketpool/api/faucet"
	"github.com/Seb369888/smartnode/rocketpool/api/minipool"
	"github.com/Seb369888/smartnode/rocketpool/api/network"
	"github.com/Seb369888/smartnode/rocketpool/api/node"
	"github.com/Seb369888/smartnode/rocketpool/api/odao"
	"github.com/Seb369888/smartnode/rocketpool/api/queue"
	apiservice "github.com/Seb369888/smartnode/rocketpool/api/service"
	"github.com/Seb369888/smartnode/rocketpool/api/wallet"
	"github.com/Seb369888/smartnode/shared/services"
	apitypes "github.com/Seb369888/smartnode/shared/types/api"
	"github.com/Seb369888/smartnode/shared/utils/api"
	cliutils "github.com/Seb369888/smartnode/shared/utils/cli"
)

// Waits for an auction transaction
func waitForTransaction(c *cli.Context, hash common.Hash) (*apitypes.APIResponse, error) {

	rp, err := services.GetRocketPool(c)
	if err != nil {
		return nil, err
	}

	// Response
	response := apitypes.APIResponse{}
	_, err = utils.WaitForTransaction(rp.Client, hash)
	if err != nil {
		return nil, err
	}

	// Return response
	return &response, nil

}

// Register commands
func RegisterCommands(app *cli.App, name string, aliases []string) {

	// CLI command
	command := cli.Command{
		Name:        name,
		Aliases:     aliases,
		Usage:       "Run Poolsea API commands",
		Subcommands: []cli.Command{},
	}

	// Don't show help message for api errors because of JSON serialisation
	command.OnUsageError = func(context *cli.Context, err error, isSubcommand bool) error {
		return err
	}

	// Register subcommands
	auction.RegisterSubcommands(&command, "auction", []string{"a"})
	faucet.RegisterSubcommands(&command, "faucet", []string{"f"})
	minipool.RegisterSubcommands(&command, "minipool", []string{"m"})
	network.RegisterSubcommands(&command, "network", []string{"e"})
	node.RegisterSubcommands(&command, "node", []string{"n"})
	odao.RegisterSubcommands(&command, "odao", []string{"o"})
	queue.RegisterSubcommands(&command, "queue", []string{"q"})
	wallet.RegisterSubcommands(&command, "wallet", []string{"w"})
	apiservice.RegisterSubcommands(&command, "service", []string{"s"})
	debug.RegisterSubcommands(&command, "debug", []string{"d"})

	// Append a general wait-for-transaction command to support async operations
	command.Subcommands = append(command.Subcommands, cli.Command{
		Name:      "wait",
		Aliases:   []string{"t"},
		Usage:     "Wait for a transaction to complete",
		UsageText: "poolsea api wait tx-hash",
		Action: func(c *cli.Context) error {
			// Validate args
			if err := cliutils.ValidateArgCount(c, 1); err != nil {
				return err
			}
			hash, err := cliutils.ValidateTxHash("tx-hash", c.Args().Get(0))
			if err != nil {
				return err
			}

			// Run
			api.PrintResponse(waitForTransaction(c, hash))
			return nil
		},
	})

	// Register CLI command
	app.Commands = append(app.Commands, command)

}
