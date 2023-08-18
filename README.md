# Poolsea - Smart Node Package

<p align="center">
  <img src="https://www.gitbook.com/cdn-cgi/image/width=256,dpr=2,height=40,fit=contain,format=auto/https%3A%2F%2F406267852-files.gitbook.io%2F~%2Ffiles%2Fv0%2Fb%2Fgitbook-x-prod.appspot.com%2Fo%2Fspaces%252F7YLyjaM5Wxa1uBHDuGPS%252Flogo%252FhyPZShxiDyQ4dByFKvcr%252FFull%2520Logo%2520White%2520Text.png%3Falt%3Dmedia%26token%3Df8632f6a-da81-4702-a2a6-c07db719be2a" alt="Poolsea icon" width="300" />
</p>

---

Poolsea is a next generation Proof-of-Stake (PoS) infrastructure service designed to be highly decentralised, distributed, and compatible with new consensus protocol.

Running a Poolsea smart node allows you to stake on Pulsechain with only 16 mln PLS and 1.6 mln PLS worth of Poolsea's POOL token.
You can earn a higher return than you would outside the network by capturing an additional 15% commission on staked PLS as well as POOL rewards.

This repository contains the source code for:

* The Poolsea Smartnode client (CLI), which is used to manage a smart node either locally or remotely (over SSH)
* The Poolsea Smartnode service, which provides an API for client communication and performs background node tasks

The Smartnode service is designed to be run as part of a Docker stack and generally does not need to be installed manually.
See the [Poolsea dockerhub](https://hub.docker.com/u/rocketpool) page for a complete list of Docker images.


## Installation

See the [Smartnode Installer](https://github.com/Seb369888/smartnode-install) repository for supported platforms and installation instructions.


## CLI Commands

The following commands are available via the Smartnode client:


### COMMANDS:
- **auction**, a - Manage Poolsea POOL auctions
  - `poolsea auction status, s` - Get POOL auction status
  - `poolsea auction lots, l` - Get POOL lots for auction
  - `poolsea auction create-lot, t` - Create a new lot
  - `poolsea auction bid-lot, b` - Bid on a lot
  - `poolsea auction claim-lot, c` - Claim POOL from a lot
  - `poolsea auction recover-lot, r` - Recover unclaimed POOL from a lot (returning it to the auction contract)
- **minipool**, m - Manage the node's minipools
  - `poolsea minipool status, s` - Get a list of the node's minipools
  - `poolsea minipool stake, t` - Stake a minipool after the scrub check, moving it from prelaunch to staking.
  - `poolsea minipool refund, r` - Refund PLS belonging to the node from minipools
  - `poolsea minipool exit, e` - Exit staking minipools from the beacon chain
  - `poolsea minipool delegate-upgrade, u` - Upgrade a minipool's delegate contract to the latest version
  - `poolsea minipool delegate-rollback, b` - Roll a minipool's delegate contract back to its previous version
  - `poolsea minipool set-use-latest-delegate, l` - If enabled, the minipool will ignore its current delegate contract and always use whatever the latest delegate is
  - `poolsea minipool find-vanity-address, v` - Search for a custom vanity minipool address
- **network**, e - Manage Poolsea network parameters
  - `poolsea network stats, s` - Get stats about the Poolsea network and its tokens
  - `poolsea network timezone-map, t` - Shows a table of the timezones that node operators belong to
  - `poolsea network node-fee, f` - Get the current network node commission rate
  - `poolsea network rpl-price, p` - Get the current network POOL price in PLS
  - `poolsea network generate-rewards-tree, g` - Generate and save the rewards tree file for the provided interval.
  Note that this is an asynchronous process, so it will return before the file is generated.
  You will need to use `rocketpool service logs api` to follow its progress.
  - `rocketpool network dao-proposals, d` - Get the currently active DAO proposals
- **node**, n - Manage the node
  - `poolsea node status, s` - Get the node's status
  - `poolsea node sync, y` - Get the sync progress of the eth1 and eth2 clients
  - `poolsea node register, r` - Register the node with Poolsea
  - `poolsea node rewards, e` - Get the time and your expected POOL rewards of the next checkpoint
  - `poolsea node set-withdrawal-address, w` - Set the node's withdrawal address
  - `poolsea node confirm-withdrawal-address, f` - Confirm the node's pending withdrawal address if it has been set back to the node's address itself
  - `poolsea node set-timezone, t` - Set the node's timezone location
  - `poolsea node swap-rpl, p` - Swap old POOL for new POOL
  - `poolsea node stake-rpl, k` - Stake POOL against the node
  - `poolsea node claim-rewards, c` - Claim available POOL and PLS rewards for any checkpoint you haven't claimed yet
  - `poolsea node withdraw-rpl, i` - Withdraw POOL staked against the node
  - `poolsea node deposit, d` - Make a deposit and create a minipool
  - `poolsea node send, n` - Send PLS or tokens from the node account to an address
  - `poolsea node set-voting-delegate, sv` - Set the address you want to use when voting on Poolsea governance proposals, or the address you want to delegate your voting power to.
  - `poolsea node clear-voting-delegate, cv` - Remove the address you've set for voting on Poolsea governance proposals.
  - `poolsea node initialize-fee-distributor, z` - Create the fee distributor contract for your node, so you can withdraw priority fees and MEV rewards after the merge
  - `poolsea node distribute-fees, b` - Distribute the priority fee and MEV rewards from your fee distributor to your withdrawal address and the rETH contract (based on your node's average commission` -
  - `poolsea node join-smoothing-pool, js` - Opt your node into the Smoothing Pool
  - `poolsea node leave-smoothing-pool, ls` - Leave the Smoothing Pool
  - `poolsea node sign-message, sm` - Sign an arbitrary message with the node's private key
- **odao**, o - Manage the Poolsea oracle DAO
  - `poolsea odao status, s` - Get oracle DAO status
  - `poolsea odao members, m` - Get the oracle DAO members
  - `poolsea odao member-settings, b` - Get the oracle DAO settings related to oracle DAO members
  - `poolsea odao proposal-settings, a` - Get the oracle DAO settings related to oracle DAO proposals
  - `poolsea odao minipool-settings, i` - Get the oracle DAO settings related to minipools
  - `poolsea odao propose, p` - Make an oracle DAO proposal
  - `poolsea odao proposals, o` - Manage oracle DAO proposals
  - `poolsea odao join, j` - Join the oracle DAO (requires an executed invite proposal)
  - `poolsea odao leave, l` - Leave the oracle DAO (requires an executed leave proposal)
- **queue**, q - Manage the Poolsea deposit queue
  - `poolsea queue status, s` - Get the deposit pool and minipool queue status
  - `poolsea queue process, p` - Process the deposit pool
- **service**, s - Manage Poolsea service
  - `poolsea service install, i` - Install the Poolsea service
  - `poolsea service config, c` - Configure the Poolsea service
  - `poolsea service status, u` - View the Poolsea service status
  - `poolsea service start, s` -  Start the Poolsea service
  - `poolsea service pause, p` -  Pause the Poolsea service
  - `poolsea service stop, o` - Pause the Poolsea service (alias of 'rocketpool service pause')
  - `poolsea service logs, l` - View the Poolsea service logs
  - `poolsea service stats, a` - View the Poolsea service stats
  - `poolsea service compose` - View the Poolsea service docker compose config
  - `poolsea service version, v` - View the Poolsea service version information
  - `poolsea service prune-eth1, n` - Shuts down the main ETH1 client and prunes its database, freeing up disk space, then restarts it when it's done.
  - `poolsea service install-update-tracker, d` - Install the update tracker that provides the available system update count to the metrics dashboard
  - `poolsea service get-config-yaml` - Generate YAML that shows the current configuration schema, including all of the parameters and their descriptions
  - `poolsea service export-eth1-data` - Exports the execution client (eth1) chain data to an external folder. Use this if you want to back up your chain data before switching execution clients.
  - `poolsea service import-eth1-data` - Imports execution client (eth1) chain data from an external folder. Use this if you want to restore the data from an execution client that you previously backed up.
  - `poolsea service resync-eth1` - Deletes the main ETH1 client's chain data and resyncs it from scratch. Only use this as a last resort!
  - `poolsea service resync-eth2` - Deletes the ETH2 client's chain data and resyncs it from scratch. Only use this as a last resort!
  - `poolsea service terminate, t` - Deletes all of the Poolsea Docker containers and volumes, including your ETH1 and ETH2 chain data and your Prometheus database (if metrics are enabled). Only use this if you are cleaning up the Smartnode and want to start over!
- **wallet**, w - Manage the node wallet
  - `poolsea wallet status, s` - Get the node wallet status
  - `poolsea wallet init, i` - Initialize the node wallet
  - `poolsea wallet recover, r` - Recover a node wallet from a mnemonic phrase
  - `poolsea wallet rebuild, b` - Rebuild validator keystores from derived keys
  - `poolsea wallet test-recovery, t` - Test recovering a node wallet without actually generating any of the node wallet or validator key files to ensure the process works as expected
  - `poolsea wallet export, e` - Export the node wallet in JSON format
  - `poolsea wallet purge` - Deletes your node wallet, your validator keys, and restarts your Validator Client while preserving your chain data. WARNING: Only use this if you want to stop validating with this machine!
  - `poolsea wallet set-ens-name` - Send a transaction from the node wallet to configure it's ENS name
- **help**, h - Shows a list of commands or help for one command


### GLOBAL OPTIONS:
 - `poolsea --allow-root, -r` - Allow rocketpool to be run as the root user
 - `poolsea --config-path path, -c path` - Poolsea config asset path (default: "~/.rocketpool")
 - `poolsea --daemon-path path, -d path` - Interact with a Poolsea service daemon at a path on the host OS, running outside of docker
 - `poolsea --maxFee value, -f value` - The max fee (including the priority fee) you want a transaction to cost, in gwei (default: 0)
 - `poolsea --maxPrioFee value, -i value` - The max priority fee you want a transaction to use, in gwei (default: 0)
 - `poolsea --gasLimit value, -l value` - [DEPRECATED] Desired gas limit (default: 0)
 - `poolsea --nonce value` - Use this flag to explicitly specify the nonce that this transaction should use, so it can override an existing 'stuck' transaction
 - `poolsea --debug` - Enable debug printing of API commands
 - `poolsea --secure-session, -s` - Some commands may print sensitive information to your terminal. Use this flag when nobody can see your screen to allow sensitive data to be printed without prompting
 - `poolsea --help, -h` - show help
 - `poolsea --version, -v` - print the version
