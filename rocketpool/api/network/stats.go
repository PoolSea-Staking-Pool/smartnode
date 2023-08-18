package network

import (
	"context"
	"fmt"

	"github.com/Seb369888/poolsea-go/deposit"
	v110_node "github.com/Seb369888/poolsea-go/legacy/v1.1.0/node"
	"github.com/Seb369888/poolsea-go/minipool"
	"github.com/Seb369888/poolsea-go/network"
	"github.com/Seb369888/poolsea-go/node"
	"github.com/Seb369888/poolsea-go/tokens"
	"github.com/Seb369888/poolsea-go/utils/eth"
	rpstate "github.com/Seb369888/poolsea-go/utils/state"
	"github.com/ethereum/go-ethereum/common"
	"github.com/urfave/cli"
	"golang.org/x/sync/errgroup"

	"github.com/Seb369888/smartnode/shared/services"
	"github.com/Seb369888/smartnode/shared/services/state"
	"github.com/Seb369888/smartnode/shared/types/api"
)

func getStats(c *cli.Context) (*api.NetworkStatsResponse, error) {

	// Get services
	if err := services.RequireRocketStorage(c); err != nil {
		return nil, err
	}
	rp, err := services.GetRocketPool(c)
	if err != nil {
		return nil, err
	}
	cfg, err := services.GetConfig(c)
	if err != nil {
		return nil, err
	}

	// Response
	response := api.NetworkStatsResponse{}

	// Sync
	var wg errgroup.Group

	// Get the deposit pool balance
	wg.Go(func() error {
		balance, err := deposit.GetBalance(rp, nil)
		if err == nil {
			response.DepositPoolBalance = eth.WeiToEth(balance)
		}
		return err
	})

	// Get the total minipool capacity
	wg.Go(func() error {
		minipoolQueueCapacity, err := minipool.GetQueueCapacity(rp, nil)
		if err == nil {
			response.MinipoolCapacity = eth.WeiToEth(minipoolQueueCapacity.Total)
		}
		return err
	})

	// Get the ETH utilization rate
	wg.Go(func() error {
		stakerUtilization, err := network.GetETHUtilizationRate(rp, nil)
		if err == nil {
			response.StakerUtilization = stakerUtilization
		}
		return err
	})

	// Get node fee
	wg.Go(func() error {
		nodeFee, err := network.GetNodeFee(rp, nil)
		if err == nil {
			response.NodeFee = nodeFee
		}
		return err
	})

	// Get node count
	wg.Go(func() error {
		nodeCount, err := node.GetNodeCount(rp, nil)
		if err == nil {
			response.NodeCount = nodeCount
		}
		return err
	})

	// Get minipool counts
	wg.Go(func() error {
		minipoolCounts, err := minipool.GetMinipoolCountPerStatus(rp, nil)
		if err != nil {
			return err
		}
		response.InitializedMinipoolCount = minipoolCounts.Initialized.Uint64()
		response.PrelaunchMinipoolCount = minipoolCounts.Prelaunch.Uint64()
		response.StakingMinipoolCount = minipoolCounts.Staking.Uint64()
		response.WithdrawableMinipoolCount = minipoolCounts.Withdrawable.Uint64()
		response.DissolvedMinipoolCount = minipoolCounts.Dissolved.Uint64()

		finalizedCount, err := minipool.GetFinalisedMinipoolCount(rp, nil)
		if err != nil {
			return err
		}
		response.FinalizedMinipoolCount = finalizedCount

		return nil
	})

	// Get RPL price
	wg.Go(func() error {
		rplPrice, err := network.GetRPLPrice(rp, nil)
		if err == nil {
			response.RplPrice = eth.WeiToEth(rplPrice)
		}
		return err
	})

	// Get total RPL staked
	wg.Go(func() error {
		totalStaked, err := node.GetTotalRPLStake(rp, nil)
		if err == nil {
			response.TotalRplStaked = eth.WeiToEth(totalStaked)
		}
		return err
	})

	// Get total effective RPL staked
	wg.Go(func() error {
		isAtlasDeployed, err := state.IsAtlasDeployed(rp, nil)
		if err != nil {
			return fmt.Errorf("error checking if Atlas is deployed: %w", err)
		}
		if !isAtlasDeployed {
			legacyNodeStakingAddress := cfg.Smartnode.GetV110NodeStakingAddress()
			effectiveStaked, err := v110_node.GetTotalEffectiveRPLStake(rp, nil, &legacyNodeStakingAddress)
			if err != nil {
				return err
			}
			response.EffectiveRplStaked = eth.WeiToEth(effectiveStaked)
			return nil
		} else {
			multicallerAddress := common.HexToAddress(cfg.Smartnode.GetMulticallAddress())
			balanceBatcherAddress := common.HexToAddress(cfg.Smartnode.GetBalanceBatcherAddress())
			contracts, err := rpstate.NewNetworkContracts(rp, multicallerAddress, balanceBatcherAddress, isAtlasDeployed, nil)
			if err != nil {
				return fmt.Errorf("error getting network contracts: %w", err)
			}
			totalEffectiveStake, err := rpstate.GetTotalEffectiveRplStake(rp, contracts)
			if err != nil {
				return fmt.Errorf("error getting total effective stake: %w", err)
			}
			response.EffectiveRplStaked = eth.WeiToEth(totalEffectiveStake)
			return nil
		}
	})

	// Get rETH price
	wg.Go(func() error {
		rethPrice, err := tokens.GetRETHExchangeRate(rp, nil)
		if err == nil {
			response.RethPrice = rethPrice
		}
		return err
	})

	// Get smoothing pool status
	wg.Go(func() error {
		smoothingPoolNodes, err := node.GetSmoothingPoolRegisteredNodeCount(rp, nil)
		if err == nil {
			response.SmoothingPoolNodes = smoothingPoolNodes
		}
		return err
	})

	// Get smoothing pool balance
	wg.Go(func() error {
		// Get the Smoothing Pool contract's balance
		smoothingPoolContract, err := rp.GetContract("poolseaSmoothingPool", nil)
		if err != nil {
			return fmt.Errorf("error getting smoothing pool contract: %w", err)
		}
		response.SmoothingPoolAddress = *smoothingPoolContract.Address

		smoothingPoolBalance, err := rp.Client.BalanceAt(context.Background(), *smoothingPoolContract.Address, nil)
		if err != nil {
			return fmt.Errorf("error getting smoothing pool balance: %w", err)
		}

		response.SmoothingPoolBalance = eth.WeiToEth(smoothingPoolBalance)
		return nil
	})

	// Wait for data
	if err := wg.Wait(); err != nil {
		return nil, err
	}

	// Get the TVL
	activeMinipools := response.InitializedMinipoolCount +
		response.PrelaunchMinipoolCount +
		response.StakingMinipoolCount +
		response.WithdrawableMinipoolCount +
		response.DissolvedMinipoolCount
	tvl := float64(activeMinipools)*32_000_000 + response.DepositPoolBalance + response.MinipoolCapacity + (response.TotalRplStaked * response.RplPrice)
	response.TotalValueLocked = tvl

	// Return response
	return &response, nil

}
