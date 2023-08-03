package faucet

import (
	"github.com/urfave/cli"

	"github.com/rocket-pool/smartnode/shared/utils/api"
	cliutils "github.com/rocket-pool/smartnode/shared/utils/cli"
)

// Register subcommands
func RegisterSubcommands(command *cli.Command, name string, aliases []string) {
	command.Subcommands = append(command.Subcommands, cli.Command{
		Name:    name,
		Aliases: aliases,
		Usage:   "Access the legacy RPL faucet",
		Subcommands: []cli.Command{

			{
				Name:      "status",
				Aliases:   []string{"s"},
				Usage:     "Get the faucet's status",
				UsageText: "poolsea api faucet status",
				Action: func(c *cli.Context) error {

					// Validate args
					if err := cliutils.ValidateArgCount(c, 0); err != nil {
						return err
					}

					// Run
					api.PrintResponse(getStatus(c))
					return nil

				},
			},

			{
				Name:      "can-withdraw-rpl",
				Usage:     "Check whether the node can withdraw legacy RPL from the faucet",
				UsageText: "poolsea api faucet can-withdraw-rpl",
				Action: func(c *cli.Context) error {

					// Validate args
					if err := cliutils.ValidateArgCount(c, 0); err != nil {
						return err
					}

					// Run
					api.PrintResponse(canWithdrawRpl(c))
					return nil

				},
			},
			{
				Name:      "withdraw-rpl",
				Aliases:   []string{"w"},
				Usage:     "Withdraw legacy RPL from the faucet",
				UsageText: "poolsea api faucet withdraw-rpl",
				Action: func(c *cli.Context) error {

					// Validate args
					if err := cliutils.ValidateArgCount(c, 0); err != nil {
						return err
					}

					// Run
					api.PrintResponse(withdrawRpl(c))
					return nil

				},
			},
		},
	})
}
