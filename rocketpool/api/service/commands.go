package service

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
		Usage:   "Manage the Poolsea deposit queue",
		Subcommands: []cli.Command{
			{
				Name:      "terminate-data-folder",
				Aliases:   []string{"t"},
				Usage:     "Deletes the data folder including the wallet file, password file, and all validator keys - don't use this unless you have a very good reason to do it (such as switching from Prater to Mainnet)",
				UsageText: "poolsea api service terminate-data-folder",
				Action: func(c *cli.Context) error {

					// Validate args
					if err := cliutils.ValidateArgCount(c, 0); err != nil {
						return err
					}

					// Run
					api.PrintResponse(terminateDataFolder(c))
					return nil

				},
			},

			{
				Name:      "get-client-status",
				Aliases:   []string{"g"},
				Usage:     "Gets the status of the configured Execution and Beacon clients",
				UsageText: "poolsea api service get-client-status",
				Action: func(c *cli.Context) error {

					// Validate args
					if err := cliutils.ValidateArgCount(c, 0); err != nil {
						return err
					}

					// Run
					api.PrintResponse(getClientStatus(c))
					return nil

				},
			},

			{
				Name:      "restart-vc",
				Usage:     "Restarts the validator client",
				UsageText: "poolsea api service restart-vc",
				Action: func(c *cli.Context) error {

					// Validate args
					if err := cliutils.ValidateArgCount(c, 0); err != nil {
						return err
					}

					// Run
					api.PrintResponse(restartVc(c))
					return nil

				},
			},
		},
	})
}
