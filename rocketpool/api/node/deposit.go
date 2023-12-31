package node

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"time"

	tndao "github.com/Seb369888/poolsea-go/dao/trustednode"
	"github.com/Seb369888/poolsea-go/deposit"
	v110_network "github.com/Seb369888/poolsea-go/legacy/v1.1.0/network"
	v110_node "github.com/Seb369888/poolsea-go/legacy/v1.1.0/node"
	v110_utils "github.com/Seb369888/poolsea-go/legacy/v1.1.0/utils"
	"github.com/Seb369888/poolsea-go/minipool"
	"github.com/Seb369888/poolsea-go/node"
	"github.com/Seb369888/poolsea-go/rocketpool"
	"github.com/Seb369888/poolsea-go/settings/protocol"
	"github.com/Seb369888/poolsea-go/settings/trustednode"
	rptypes "github.com/Seb369888/poolsea-go/types"
	"github.com/Seb369888/poolsea-go/utils/eth"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/prysmaticlabs/prysm/v3/beacon-chain/core/signing"
	"github.com/urfave/cli"
	"golang.org/x/sync/errgroup"

	"github.com/Seb369888/smartnode/shared/services"
	"github.com/Seb369888/smartnode/shared/services/beacon"
	"github.com/Seb369888/smartnode/shared/services/state"
	"github.com/Seb369888/smartnode/shared/services/wallet"
	"github.com/Seb369888/smartnode/shared/types/api"
	"github.com/Seb369888/smartnode/shared/utils/eth1"
	rputils "github.com/Seb369888/smartnode/shared/utils/rp"
	"github.com/Seb369888/smartnode/shared/utils/validator"
	prdeposit "github.com/prysmaticlabs/prysm/v3/contracts/deposit"
	ethpb "github.com/prysmaticlabs/prysm/v3/proto/prysm/v1alpha1"
	eth2types "github.com/wealdtech/go-eth2-types/v2"
)

const (
	prestakeDepositAmount float64 = 1_000_000.0
	ValidatorEth          float64 = 32_000_000.0
)

func canNodeDeposit(c *cli.Context, amountWei *big.Int, minNodeFee float64, salt *big.Int) (*api.CanNodeDepositResponse, error) {

	// Get services
	if err := services.RequireNodeRegistered(c); err != nil {
		return nil, err
	}
	w, err := services.GetWallet(c)
	if err != nil {
		return nil, err
	}
	ec, err := services.GetEthClient(c)
	if err != nil {
		return nil, err
	}
	rp, err := services.GetRocketPool(c)
	if err != nil {
		return nil, err
	}
	bc, err := services.GetBeaconClient(c)
	if err != nil {
		return nil, err
	}

	// Get eth2 config
	eth2Config, err := bc.GetEth2Config()
	if err != nil {
		return nil, err
	}

	// Response
	response := api.CanNodeDepositResponse{}

	isAtlasDeployed, err := state.IsAtlasDeployed(rp, nil)
	if err != nil {
		return nil, fmt.Errorf("error checking if Atlas has been deployed: %w", err)
	}
	response.IsAtlasDeployed = isAtlasDeployed
	if !isAtlasDeployed {
		return legacyCanNodeDeposit(c, amountWei, minNodeFee, salt, w, ec, rp, bc, eth2Config)
	}

	// Get node account
	nodeAccount, err := w.GetNodeAccount()
	if err != nil {
		return nil, err
	}

	// Adjust the salt
	if salt.Cmp(big.NewInt(0)) == 0 {
		nonce, err := ec.NonceAt(context.Background(), nodeAccount.Address, nil)
		if err != nil {
			return nil, err
		}
		salt.SetUint64(nonce)
	}

	// Data
	var wg1 errgroup.Group
	var ethMatched *big.Int
	var ethMatchedLimit *big.Int
	var pendingMatchAmount *big.Int
	var minipoolAddress common.Address
	var depositPoolBalance *big.Int

	// Check credit balance
	wg1.Go(func() error {
		ethBalanceWei, err := node.GetNodeDepositCredit(rp, nodeAccount.Address, nil)
		if err == nil {
			response.CreditBalance = ethBalanceWei
		}
		return err
	})

	// Check node balance
	wg1.Go(func() error {
		ethBalanceWei, err := ec.BalanceAt(context.Background(), nodeAccount.Address, nil)
		if err == nil {
			response.NodeBalance = ethBalanceWei
		}
		return err
	})

	// Check node deposits are enabled
	wg1.Go(func() error {
		depositEnabled, err := protocol.GetNodeDepositEnabled(rp, nil)
		if err == nil {
			response.DepositDisabled = !depositEnabled
		}
		return err
	})

	// Get node staking information
	wg1.Go(func() error {
		ethMatched, ethMatchedLimit, pendingMatchAmount, err = rputils.CheckCollateral(rp, nodeAccount.Address, nil)
		if err != nil {
			return fmt.Errorf("error checking collateral for node %s: %w", nodeAccount.Address.Hex(), err)
		}
		return nil
	})

	// Get deposit pool balance
	wg1.Go(func() error {
		var err error
		depositPoolBalance, err = deposit.GetBalance(rp, nil)
		return err
	})

	// Wait for data
	if err := wg1.Wait(); err != nil {
		return nil, err
	}

	// Check for insufficient balance
	totalBalance := big.NewInt(0).Add(response.NodeBalance, response.CreditBalance)
	response.InsufficientBalance = (amountWei.Cmp(totalBalance) > 0)

	// Check if the credit balance can be used
	response.DepositBalance = depositPoolBalance
	response.CanUseCredit = (depositPoolBalance.Cmp(eth.EthToWei(1)) >= 0)

	// Check data
	validatorEthWei := eth.EthToWei(ValidatorEth)
	matchRequest := big.NewInt(0).Sub(validatorEthWei, amountWei)
	availableToMatch := big.NewInt(0).Sub(ethMatchedLimit, ethMatched)
	availableToMatch.Sub(availableToMatch, pendingMatchAmount)
	response.InsufficientRplStake = (availableToMatch.Cmp(matchRequest) == -1)

	// Update response
	response.CanDeposit = !(response.InsufficientBalance || response.InsufficientRplStake || response.InvalidAmount || response.DepositDisabled)
	if !response.CanDeposit {
		return &response, nil
	}

	if response.CanDeposit && !response.CanUseCredit && response.NodeBalance.Cmp(amountWei) < 0 {
		// Can't use credit and there's not enough ETH in the node wallet to deposit so error out
		response.InsufficientBalanceWithoutCredit = true
		response.CanDeposit = false
	}

	// Break before the gas estimator if depositing won't work
	if !response.CanDeposit {
		return &response, nil
	}

	// Get gas estimate
	opts, err := w.GetNodeAccountTransactor()
	if err != nil {
		return nil, err
	}

	// Get how much credit to use
	if response.CanUseCredit {
		remainingAmount := big.NewInt(0).Sub(amountWei, response.CreditBalance)
		if remainingAmount.Cmp(big.NewInt(0)) > 0 {
			// Send the remaining amount if the credit isn't enough to cover the whole deposit
			opts.Value = remainingAmount
		}
	} else {
		opts.Value = amountWei
	}

	// Get the next validator key
	validatorKey, err := w.GetNextValidatorKey()
	if err != nil {
		return nil, err
	}

	// Get the next minipool address and withdrawal credentials
	minipoolAddress, err = minipool.GetExpectedAddress(rp, nodeAccount.Address, salt, nil)
	if err != nil {
		return nil, err
	}
	response.MinipoolAddress = minipoolAddress
	withdrawalCredentials, err := minipool.GetMinipoolWithdrawalCredentials(rp, minipoolAddress, nil)
	if err != nil {
		return nil, err
	}

	// Get validator deposit data and associated parameters
	depositAmount := uint64(1e9) // 1 ETH in gwei
	depositData, depositDataRoot, err := validator.GetDepositData(validatorKey, withdrawalCredentials, eth2Config, depositAmount)
	if err != nil {
		return nil, err
	}
	pubKey := rptypes.BytesToValidatorPubkey(depositData.PublicKey)
	signature := rptypes.BytesToValidatorSignature(depositData.Signature)

	// Do a final sanity check
	err = validateDepositInfo(eth2Config, uint64(depositAmount), pubKey, withdrawalCredentials, signature)
	if err != nil {
		return nil, fmt.Errorf("Your deposit failed the validation safety check: %w\n"+
			"For your safety, this deposit will not be submitted and your ETH will not be staked.\n"+
			"PLEASE REPORT THIS TO THE POOLSEA DEVELOPERS and include the following information:\n"+
			"\tDomain Type: 0x%s\n"+
			"\tGenesis Fork Version: 0x%s\n"+
			"\tGenesis Validator Root: 0x%s\n"+
			"\tDeposit Amount: %d gwei\n"+
			"\tValidator Pubkey: %s\n"+
			"\tWithdrawal Credentials: %s\n"+
			"\tSignature: %s\n",
			err,
			hex.EncodeToString(eth2types.DomainDeposit[:]),
			hex.EncodeToString(eth2Config.GenesisForkVersion),
			hex.EncodeToString(eth2types.ZeroGenesisValidatorsRoot),
			depositAmount,
			pubKey.Hex(),
			withdrawalCredentials.Hex(),
			signature.Hex(),
		)
	}

	// Run the deposit gas estimator
	if response.CanUseCredit {
		gasInfo, err := node.EstimateDepositWithCreditGas(rp, amountWei, minNodeFee, pubKey, signature, depositDataRoot, salt, minipoolAddress, opts)
		if err != nil {
			return nil, err
		}
		response.GasInfo = gasInfo
	} else {
		gasInfo, err := node.EstimateDepositGas(rp, amountWei, minNodeFee, pubKey, signature, depositDataRoot, salt, minipoolAddress, opts)
		if err != nil {
			return nil, err
		}
		response.GasInfo = gasInfo
	}

	return &response, nil

}

func legacyCanNodeDeposit(c *cli.Context, amountWei *big.Int, minNodeFee float64, salt *big.Int, w *wallet.Wallet, ec *services.ExecutionClientManager, rp *rocketpool.RocketPool, bc *services.BeaconClientManager, eth2Config beacon.Eth2Config) (*api.CanNodeDepositResponse, error) {

	// Services
	cfg, err := services.GetConfig(c)
	if err != nil {
		return nil, err
	}

	// Response
	response := api.CanNodeDepositResponse{}

	// Get the legacy contract addresses
	rocketNodeDepositAddress := cfg.Smartnode.GetV110NodeDepositAddress()
	rocketNodeStakingAddress := cfg.Smartnode.GetV110NodeStakingAddress()
	rocketMinipoolFactoryAddress := cfg.Smartnode.GetV110MinipoolFactoryAddress()

	// Check if amount is zero
	amountIsZero := (amountWei.Cmp(big.NewInt(0)) == 0)

	// Get node account
	nodeAccount, err := w.GetNodeAccount()
	if err != nil {
		return nil, err
	}

	// Adjust the salt
	if salt.Cmp(big.NewInt(0)) == 0 {
		nonce, err := ec.NonceAt(context.Background(), nodeAccount.Address, nil)
		if err != nil {
			return nil, err
		}
		salt.SetUint64(nonce)
	}

	// Data
	var wg1 errgroup.Group
	var isTrusted bool
	var minipoolCount uint64
	var minipoolLimit uint64
	var minipoolAddress common.Address

	// Check node balance
	wg1.Go(func() error {
		ethBalanceWei, err := ec.BalanceAt(context.Background(), nodeAccount.Address, nil)
		if err == nil {
			response.InsufficientBalance = (amountWei.Cmp(ethBalanceWei) > 0)
		}
		return err
	})

	// Check node deposits are enabled
	wg1.Go(func() error {
		depositEnabled, err := protocol.GetNodeDepositEnabled(rp, nil)
		if err == nil {
			response.DepositDisabled = !depositEnabled
		}
		return err
	})

	// Get trusted status
	wg1.Go(func() error {
		var err error
		isTrusted, err = tndao.GetMemberExists(rp, nodeAccount.Address, nil)
		return err
	})

	// Get node staking information
	wg1.Go(func() error {
		var err error
		minipoolCount, err = minipool.GetNodeMinipoolCount(rp, nodeAccount.Address, nil)
		return err
	})
	wg1.Go(func() error {
		var err error
		minipoolLimit, err = v110_node.GetNodeMinipoolLimit(rp, nodeAccount.Address, nil, &rocketNodeStakingAddress)
		return err
	})

	// Get consensus status
	wg1.Go(func() error {
		networkPricesAddress := cfg.Smartnode.GetV110NetworkPricesAddress()

		var err error
		inConsensus, err := v110_network.InConsensus(rp, nil, &networkPricesAddress)
		response.InConsensus = inConsensus
		return err
	})

	// Get gas estimate
	wg1.Go(func() error {
		opts, err := w.GetNodeAccountTransactor()
		if err != nil {
			return err
		}
		opts.Value = amountWei

		// Get the deposit type
		depositType, err := v110_node.GetDepositType(rp, amountWei, nil, &rocketNodeDepositAddress)
		if err != nil {
			return err
		}

		// Get the next validator key
		validatorKey, err := w.GetNextValidatorKey()
		if err != nil {
			return err
		}

		// Get the next minipool address and withdrawal credentials
		minipoolAddress, err = v110_utils.GenerateAddress(rp, nodeAccount.Address, depositType, salt, nil, nil, &rocketMinipoolFactoryAddress)
		if err != nil {
			return err
		}
		withdrawalCredentials, err := minipool.GetMinipoolWithdrawalCredentials(rp, minipoolAddress, nil)
		if err != nil {
			return err
		}

		// Get validator deposit data and associated parameters
		depositAmount := eth.GweiToWei(16_000_000).Uint64()
		depositData, depositDataRoot, err := validator.GetDepositData(validatorKey, withdrawalCredentials, eth2Config, depositAmount)
		if err != nil {
			return err
		}
		pubKey := rptypes.BytesToValidatorPubkey(depositData.PublicKey)
		signature := rptypes.BytesToValidatorSignature(depositData.Signature)

		// Do a final sanity check
		err = validateDepositInfo(eth2Config, uint64(depositAmount), pubKey, withdrawalCredentials, signature)
		if err != nil {
			return fmt.Errorf("Your deposit failed the validation safety check: %w\n"+
				"For your safety, this deposit will not be submitted and your ETH will not be staked.\n"+
				"PLEASE REPORT THIS TO THE POOLSEA DEVELOPERS and include the following information:\n"+
				"\tDomain Type: 0x%s\n"+
				"\tGenesis Fork Version: 0x%s\n"+
				"\tGenesis Validator Root: 0x%s\n"+
				"\tDeposit Amount: %d gwei\n"+
				"\tValidator Pubkey: %s\n"+
				"\tWithdrawal Credentials: %s\n"+
				"\tSignature: %s\n",
				err,
				hex.EncodeToString(eth2types.DomainDeposit[:]),
				hex.EncodeToString(eth2Config.GenesisForkVersion),
				hex.EncodeToString(eth2types.ZeroGenesisValidatorsRoot),
				depositAmount,
				pubKey.Hex(),
				withdrawalCredentials.Hex(),
				signature.Hex(),
			)
		}

		// Run the deposit gas estimator
		gasInfo, err := v110_node.EstimateDepositGas(rp, minNodeFee, pubKey, signature, depositDataRoot, salt, minipoolAddress, opts, &rocketNodeDepositAddress)
		if err == nil {
			response.GasInfo = gasInfo
		}
		return err
	})

	// Wait for data
	if err := wg1.Wait(); err != nil {
		return nil, err
	}

	// Check data
	response.InsufficientRplStake = (minipoolCount >= minipoolLimit)
	response.MinipoolAddress = minipoolAddress
	response.InvalidAmount = (!isTrusted && amountIsZero)

	// Check oracle node unbonded minipool limit
	if isTrusted && amountIsZero {

		// Data
		var wg2 errgroup.Group
		var unbondedMinipoolCount uint64
		var unbondedMinipoolsMax uint64

		// Get unbonded minipool details
		wg2.Go(func() error {
			var err error
			unbondedMinipoolCount, err = tndao.GetMemberUnbondedValidatorCount(rp, nodeAccount.Address, nil)
			return err
		})
		wg2.Go(func() error {
			var err error
			unbondedMinipoolsMax, err = trustednode.GetMinipoolUnbondedMax(rp, nil)
			return err
		})

		// Wait for data
		if err := wg2.Wait(); err != nil {
			return nil, err
		}

		// Check unbonded minipool limit
		response.UnbondedMinipoolsAtMax = (unbondedMinipoolCount >= unbondedMinipoolsMax)

	}

	// Update & return response
	response.CanDeposit = !(response.InsufficientBalance || response.InsufficientRplStake || response.InvalidAmount || response.UnbondedMinipoolsAtMax || response.DepositDisabled || !response.InConsensus)
	return &response, nil

}

func nodeDeposit(c *cli.Context, amountWei *big.Int, minNodeFee float64, salt *big.Int, useCreditBalance bool, submit bool) (*api.NodeDepositResponse, error) {

	// Get services
	if err := services.RequireNodeRegistered(c); err != nil {
		return nil, err
	}
	w, err := services.GetWallet(c)
	if err != nil {
		return nil, err
	}
	ec, err := services.GetEthClient(c)
	if err != nil {
		return nil, err
	}
	rp, err := services.GetRocketPool(c)
	if err != nil {
		return nil, err
	}
	bc, err := services.GetBeaconClient(c)
	if err != nil {
		return nil, err
	}

	// Get eth2 config
	eth2Config, err := bc.GetEth2Config()
	if err != nil {
		return nil, err
	}

	isAtlasDeployed, err := state.IsAtlasDeployed(rp, nil)
	if err != nil {
		return nil, fmt.Errorf("error checking if Atlas has been deployed: %w", err)
	}
	if !isAtlasDeployed {
		return legacyNodeDeposit(c, amountWei, minNodeFee, salt, submit, w, ec, rp, bc, eth2Config)
	}

	// Get node account
	nodeAccount, err := w.GetNodeAccount()
	if err != nil {
		return nil, err
	}

	// Response
	response := api.NodeDepositResponse{}

	// Adjust the salt
	if salt.Cmp(big.NewInt(0)) == 0 {
		nonce, err := ec.NonceAt(context.Background(), nodeAccount.Address, nil)
		if err != nil {
			return nil, err
		}
		salt.SetUint64(nonce)
	}

	// Make sure ETH2 is on the correct chain
	depositContractInfo, err := getDepositContractInfo(c)
	if err != nil {
		return nil, err
	}
	if depositContractInfo.RPNetwork != depositContractInfo.BeaconNetwork ||
		depositContractInfo.RPDepositContract != depositContractInfo.BeaconDepositContract {
		return nil, fmt.Errorf("Beacon network mismatch! Expected %s on chain %d, but beacon is using %s on chain %d.",
			depositContractInfo.RPDepositContract.Hex(),
			depositContractInfo.RPNetwork,
			depositContractInfo.BeaconDepositContract.Hex(),
			depositContractInfo.BeaconNetwork)
	}

	// Get the scrub period
	scrubPeriodUnix, err := trustednode.GetScrubPeriod(rp, nil)
	if err != nil {
		return nil, err
	}
	scrubPeriod := time.Duration(scrubPeriodUnix) * time.Second
	response.ScrubPeriod = scrubPeriod

	// Get transactor
	opts, err := w.GetNodeAccountTransactor()
	if err != nil {
		return nil, err
	}

	// Get the node's credit balance
	creditBalanceWei, err := node.GetNodeDepositCredit(rp, nodeAccount.Address, nil)
	if err != nil {
		return nil, err
	}

	// Get how much credit to use
	if useCreditBalance {
		remainingAmount := big.NewInt(0).Sub(amountWei, creditBalanceWei)
		if remainingAmount.Cmp(big.NewInt(0)) > 0 {
			// Send the remaining amount if the credit isn't enough to cover the whole deposit
			opts.Value = remainingAmount
		}
	} else {
		opts.Value = amountWei
	}

	// Create and save a new validator key
	validatorKey, err := w.CreateValidatorKey()
	if err != nil {
		return nil, err
	}

	// Get the next minipool address and withdrawal credentials
	minipoolAddress, err := minipool.GetExpectedAddress(rp, nodeAccount.Address, salt, nil)
	if err != nil {
		return nil, err
	}
	withdrawalCredentials, err := minipool.GetMinipoolWithdrawalCredentials(rp, minipoolAddress, nil)
	if err != nil {
		return nil, err
	}

	// Get validator deposit data and associated parameters
	depositAmount := uint64(1e9) // 1 ETH in gwei
	depositData, depositDataRoot, err := validator.GetDepositData(validatorKey, withdrawalCredentials, eth2Config, depositAmount)
	if err != nil {
		return nil, err
	}
	pubKey := rptypes.BytesToValidatorPubkey(depositData.PublicKey)
	signature := rptypes.BytesToValidatorSignature(depositData.Signature)

	// Make sure a validator with this pubkey doesn't already exist
	status, err := bc.GetValidatorStatus(pubKey, nil)
	if err != nil {
		return nil, fmt.Errorf("Error checking for existing validator status: %w\nYour funds have not been deposited for your own safety.", err)
	}
	if status.Exists {
		return nil, fmt.Errorf("**** ALERT ****\n"+
			"Your minipool %s has the following as a validator pubkey:\n\t%s\n"+
			"This key is already in use by validator %d on the Beacon chain!\n"+
			"Poolsea will not allow you to deposit this validator for your own safety so you do not get slashed.\n"+
			"PLEASE REPORT THIS TO THE POOLSEA DEVELOPERS.\n"+
			"***************\n", minipoolAddress.Hex(), pubKey.Hex(), status.Index)
	}

	// Do a final sanity check
	err = validateDepositInfo(eth2Config, depositAmount, pubKey, withdrawalCredentials, signature)
	if err != nil {
		return nil, fmt.Errorf("Your deposit failed the validation safety check: %w\n"+
			"For your safety, this deposit will not be submitted and your ETH will not be staked.\n"+
			"PLEASE REPORT THIS TO THE POOLSEA DEVELOPERS and include the following information:\n"+
			"\tDomain Type: 0x%s\n"+
			"\tGenesis Fork Version: 0x%s\n"+
			"\tGenesis Validator Root: 0x%s\n"+
			"\tDeposit Amount: %d gwei\n"+
			"\tValidator Pubkey: %s\n"+
			"\tWithdrawal Credentials: %s\n"+
			"\tSignature: %s\n",
			err,
			hex.EncodeToString(eth2types.DomainDeposit[:]),
			hex.EncodeToString(eth2Config.GenesisForkVersion),
			hex.EncodeToString(eth2types.ZeroGenesisValidatorsRoot),
			depositAmount,
			pubKey.Hex(),
			withdrawalCredentials.Hex(),
			signature.Hex(),
		)
	}

	// Override the provided pending TX if requested
	err = eth1.CheckForNonceOverride(c, opts)
	if err != nil {
		return nil, fmt.Errorf("Error checking for nonce override: %w", err)
	}

	// Do not send transaction unless requested
	opts.NoSend = !submit

	// Deposit
	var tx *types.Transaction
	if useCreditBalance {
		tx, err = node.DepositWithCredit(rp, amountWei, minNodeFee, pubKey, signature, depositDataRoot, salt, minipoolAddress, opts)
	} else {
		tx, err = node.Deposit(rp, amountWei, minNodeFee, pubKey, signature, depositDataRoot, salt, minipoolAddress, opts)
	}
	if err != nil {
		return nil, err
	}

	// Save wallet
	if err := w.Save(); err != nil {
		return nil, err
	}

	// Print transaction if requested
	if !submit {
		b, err := tx.MarshalBinary()
		if err != nil {
			return nil, err
		}
		fmt.Printf("%x\n", b)
	}

	response.TxHash = tx.Hash()
	response.MinipoolAddress = minipoolAddress
	response.ValidatorPubkey = pubKey

	// Return response
	return &response, nil

}

func legacyNodeDeposit(c *cli.Context, amountWei *big.Int, minNodeFee float64, salt *big.Int, submit bool, w *wallet.Wallet, ec *services.ExecutionClientManager, rp *rocketpool.RocketPool, bc *services.BeaconClientManager, eth2Config beacon.Eth2Config) (*api.NodeDepositResponse, error) {

	// Services
	cfg, err := services.GetConfig(c)
	if err != nil {
		return nil, err
	}
	// Get the legacy contract address
	rocketNodeDepositAddress := cfg.Smartnode.GetV110NodeDepositAddress()
	rocketMinipoolFactoryAddress := cfg.Smartnode.GetV110MinipoolFactoryAddress()

	// Get node account
	nodeAccount, err := w.GetNodeAccount()
	if err != nil {
		return nil, err
	}

	// Response
	response := api.NodeDepositResponse{}

	// Adjust the salt
	if salt.Cmp(big.NewInt(0)) == 0 {
		nonce, err := ec.NonceAt(context.Background(), nodeAccount.Address, nil)
		if err != nil {
			return nil, err
		}
		salt.SetUint64(nonce)
	}

	// Make sure ETH2 is on the correct chain
	depositContractInfo, err := getDepositContractInfo(c)
	if err != nil {
		return nil, err
	}
	if depositContractInfo.RPNetwork != depositContractInfo.BeaconNetwork ||
		depositContractInfo.RPDepositContract != depositContractInfo.BeaconDepositContract {
		return nil, fmt.Errorf("Beacon network mismatch! Expected %s on chain %d, but beacon is using %s on chain %d.",
			depositContractInfo.RPDepositContract.Hex(),
			depositContractInfo.RPNetwork,
			depositContractInfo.BeaconDepositContract.Hex(),
			depositContractInfo.BeaconNetwork)
	}

	// Get the scrub period
	scrubPeriodUnix, err := trustednode.GetScrubPeriod(rp, nil)
	if err != nil {
		return nil, err
	}
	scrubPeriod := time.Duration(scrubPeriodUnix) * time.Second
	response.ScrubPeriod = scrubPeriod

	// Get transactor
	opts, err := w.GetNodeAccountTransactor()
	if err != nil {
		return nil, err
	}
	opts.Value = amountWei

	// Get the deposit type
	depositType, err := v110_node.GetDepositType(rp, amountWei, nil, &rocketNodeDepositAddress)
	if err != nil {
		return nil, err
	}

	// Create and save a new validator key
	validatorKey, err := w.CreateValidatorKey()
	if err != nil {
		return nil, err
	}

	// Get the next minipool address and withdrawal credentials
	minipoolAddress, err := v110_utils.GenerateAddress(rp, nodeAccount.Address, depositType, salt, nil, nil, &rocketMinipoolFactoryAddress)
	if err != nil {
		return nil, err
	}
	withdrawalCredentials, err := minipool.GetMinipoolWithdrawalCredentials(rp, minipoolAddress, nil)
	if err != nil {
		return nil, err
	}

	// Get validator deposit data and associated parameters
	depositAmount := eth.GweiToWei(16_000_000).Uint64()
	depositData, depositDataRoot, err := validator.GetDepositData(validatorKey, withdrawalCredentials, eth2Config, depositAmount)
	if err != nil {
		return nil, err
	}
	pubKey := rptypes.BytesToValidatorPubkey(depositData.PublicKey)
	signature := rptypes.BytesToValidatorSignature(depositData.Signature)

	// Make sure a validator with this pubkey doesn't already exist
	status, err := bc.GetValidatorStatus(pubKey, nil)
	if err != nil {
		return nil, fmt.Errorf("Error checking for existing validator status: %w\nYour funds have not been deposited for your own safety.", err)
	}
	if status.Exists {
		return nil, fmt.Errorf("**** ALERT ****\n"+
			"Your minipool %s has the following as a validator pubkey:\n\t%s\n"+
			"This key is already in use by validator %d on the Beacon chain!\n"+
			"Poolsea will not allow you to deposit this validator for your own safety so you do not get slashed.\n"+
			"PLEASE REPORT THIS TO THE POOLSEA DEVELOPERS.\n"+
			"***************\n", minipoolAddress.Hex(), pubKey.Hex(), status.Index)
	}

	// Do a final sanity check
	err = validateDepositInfo(eth2Config, depositAmount, pubKey, withdrawalCredentials, signature)
	if err != nil {
		return nil, fmt.Errorf("Your deposit failed the validation safety check: %w\n"+
			"For your safety, this deposit will not be submitted and your ETH will not be staked.\n"+
			"PLEASE REPORT THIS TO THE POOLSEA DEVELOPERS and include the following information:\n"+
			"\tDomain Type: 0x%s\n"+
			"\tGenesis Fork Version: 0x%s\n"+
			"\tGenesis Validator Root: 0x%s\n"+
			"\tDeposit Amount: %d gwei\n"+
			"\tValidator Pubkey: %s\n"+
			"\tWithdrawal Credentials: %s\n"+
			"\tSignature: %s\n",
			err,
			hex.EncodeToString(eth2types.DomainDeposit[:]),
			hex.EncodeToString(eth2Config.GenesisForkVersion),
			hex.EncodeToString(eth2types.ZeroGenesisValidatorsRoot),
			depositAmount,
			pubKey.Hex(),
			withdrawalCredentials.Hex(),
			signature.Hex(),
		)
	}

	// Override the provided pending TX if requested
	err = eth1.CheckForNonceOverride(c, opts)
	if err != nil {
		return nil, fmt.Errorf("Error checking for nonce override: %w", err)
	}

	// Do not send transaction unless requested
	opts.NoSend = !submit

	// Deposit
	tx, err := v110_node.Deposit(rp, minNodeFee, pubKey, signature, depositDataRoot, salt, minipoolAddress, opts, &rocketNodeDepositAddress)
	if err != nil {
		return nil, err
	}

	// Save wallet
	if err := w.Save(); err != nil {
		return nil, err
	}

	// Print transaction if requested
	if !submit {
		b, err := tx.MarshalBinary()
		if err != nil {
			return nil, err
		}
		fmt.Printf("%x\n", b)
	}

	response.TxHash = tx.Hash()
	response.MinipoolAddress = minipoolAddress
	response.ValidatorPubkey = pubKey

	// Return response
	return &response, nil

}

func validateDepositInfo(eth2Config beacon.Eth2Config, depositAmount uint64, pubkey rptypes.ValidatorPubkey, withdrawalCredentials common.Hash, signature rptypes.ValidatorSignature) error {

	// Get the deposit domain based on the eth2 config
	depositDomain, err := signing.ComputeDomain(eth2types.DomainDeposit, eth2Config.GenesisForkVersion, eth2types.ZeroGenesisValidatorsRoot)
	if err != nil {
		return err
	}

	// Create the deposit struct
	depositData := new(ethpb.Deposit_Data)
	depositData.Amount = depositAmount
	depositData.PublicKey = pubkey.Bytes()
	depositData.WithdrawalCredentials = withdrawalCredentials.Bytes()
	depositData.Signature = signature.Bytes()

	// Validate the signature
	err = prdeposit.VerifyDepositSignature(depositData, depositDomain)
	return err

}
