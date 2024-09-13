package staking

import (
	"context"
	"math/big"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/subnet-evm/accounts/abi/bind"
	"github.com/ava-labs/teleporter/tests/interfaces"
	"github.com/ava-labs/teleporter/tests/utils"
	. "github.com/onsi/gomega"
)

/*
 * Registers a native token staking validator on a subnet. The steps are as follows:
 * - Deploy the NativeTokenStakingManager
 * - Initiate validator registration
 * - Deliver the Warp message to the P-Chain (not implemented)
 * - Aggregate P-Chain signatures on the response Warp message
 * - Deliver the Warp message to the subnet
 * - Verify that the validator is registered in the staking contract
 *
 * Delists the validator from the subnet. The steps are as follows:
 * - Initiate validator delisting
 * - Deliver the Warp message to the P-Chain (not implemented)
 * - Aggregate P-Chain signatures on the response Warp message
 * - Deliver the Warp message to the subnet
 * - Verify that the validator is delisted from the staking contract
 */
func NativeTokenStakingManager(network interfaces.LocalNetwork) {
	// Get the subnets info
	cChainInfo := network.GetPrimaryNetworkInfo()
	subnetAInfo, _ := utils.GetTwoSubnets(network)
	fundedAddress, fundedKey := network.GetFundedAccountInfo()
	pChainInfo := utils.GetPChainInfo(cChainInfo)

	signatureAggregator := utils.NewSignatureAggregator(
		cChainInfo.NodeURIs[0],
		[]ids.ID{
			subnetAInfo.SubnetID,
			ids.Empty, // Primary network subnet ID
		},
	)

	// Deploy the staking manager contract
	stakingManagerContractAddress, stakingManager := utils.DeployAndInitializeNativeTokenStakingManager(
		context.Background(),
		fundedKey,
		subnetAInfo,
		pChainInfo,
	)

	//
	// Register a validator
	//
	stakeAmount := new(big.Int).SetUint64(utils.DefaultMinStakeAmount)
	weight, err := stakingManager.ValueToWeight(
		&bind.CallOpts{},
		stakeAmount,
	)
	Expect(err).Should(BeNil())
	// Iniatiate validator registration
	validationID := utils.InitializeAndCompleteNativeValidatorRegistration(
		network,
		signatureAggregator,
		fundedKey,
		subnetAInfo,
		pChainInfo,
		stakingManager,
		stakingManagerContractAddress,
		stakeAmount,
	)

	// Make sure minimum stake duration has passed
	time.Sleep(time.Duration(utils.DefaultMinStakeDurationSeconds) * time.Second)

	// Send a loopback transaction to self to force a block production
	// before delisting the validator.
	utils.SendNativeTransfer(
		context.Background(),
		subnetAInfo,
		fundedKey,
		fundedAddress,
		big.NewInt(10),
	)

	//
	// Delist the validator
	//
	utils.InitializeAndCompleteEndNativeValidation(
		network,
		signatureAggregator,
		fundedKey,
		subnetAInfo,
		pChainInfo,
		stakingManager,
		stakingManagerContractAddress,
		validationID,
		weight,
		1,
	)

}
