package ictt

import (
	"context"
	"math/big"

	"github.com/ava-labs/subnet-evm/accounts/abi/bind"
	erc20tokenhome "github.com/ava-labs/teleporter/abi-bindings/go/ictt/TokenHome/ERC20TokenHome"
	localnetwork "github.com/ava-labs/teleporter/tests/network"
	"github.com/ava-labs/teleporter/tests/utils"
	"github.com/ethereum/go-ethereum/crypto"
	. "github.com/onsi/gomega"
)

/*
*
  - Deploy a ERC20TokenHome on the primary network
  - Deploys NativeTokenRemote to Subnet A and Subnet B
  - Transfers C-Chain example ERC20 tokens to Subnet A as Subnet A's native token
  - Transfers C-Chain example ERC20 tokens to Subnet B as Subnet B's native token
    to collateralize the token transferrer on Subnet B
  - Transfer tokens from Subnet A to Subnet B through multi-hop
  - Transfer back tokens from Subnet B to Subnet A through multi-hop
*/
func ERC20TokenHomeNativeTokenRemoteMultiHop(network *localnetwork.LocalNetwork, teleporter utils.TeleporterTestInfo) {
	cChainInfo := network.GetPrimaryNetworkInfo()
	subnetAInfo, subnetBInfo := network.GetTwoSubnets()
	fundedAddress, fundedKey := network.GetFundedAccountInfo()

	ctx := context.Background()

	// Deploy an ExampleERC20 on subnet A as the token to be transferred
	exampleERC20Address, exampleERC20 := utils.DeployExampleERC20Decimals(
		ctx,
		fundedKey,
		cChainInfo,
		erc20TokenHomeDecimals,
	)

	exampleERC20Decimals, err := exampleERC20.Decimals(&bind.CallOpts{})
	Expect(err).Should(BeNil())

	erc20TokenHomeAddress, erc20TokenHome := utils.DeployERC20TokenHome(
		ctx,
		teleporter,
		fundedKey,
		cChainInfo,
		fundedAddress,
		exampleERC20Address,
		exampleERC20Decimals,
	)

	// Deploy a NativeTokenRemote to Subnet A
	nativeTokenRemoteAddressA, nativeTokenRemoteA := utils.DeployNativeTokenRemote(
		ctx,
		teleporter,
		subnetAInfo,
		"SUBA",
		fundedAddress,
		cChainInfo.BlockchainID,
		erc20TokenHomeAddress,
		exampleERC20Decimals,
		initialReserveImbalance,
		burnedFeesReportingRewardPercentage,
	)

	// Deploy a NativeTokenRemote to Subnet B
	nativeTokenRemoteAddressB, nativeTokenRemoteB := utils.DeployNativeTokenRemote(
		ctx,
		teleporter,
		subnetBInfo,
		"SUBB",
		fundedAddress,
		cChainInfo.BlockchainID,
		erc20TokenHomeAddress,
		exampleERC20Decimals,
		initialReserveImbalance,
		burnedFeesReportingRewardPercentage,
	)

	// Register both NativeTokenDestinations on the ERC20TokenHome
	collateralAmountA := utils.RegisterTokenRemoteOnHome(
		ctx,
		teleporter,
		cChainInfo,
		erc20TokenHomeAddress,
		subnetAInfo,
		nativeTokenRemoteAddressA,
		initialReserveImbalance,
		utils.GetTokenMultiplier(decimalsShift),
		multiplyOnRemote,
		fundedKey,
	)

	collateralAmountB := utils.RegisterTokenRemoteOnHome(
		ctx,
		teleporter,
		cChainInfo,
		erc20TokenHomeAddress,
		subnetBInfo,
		nativeTokenRemoteAddressB,
		initialReserveImbalance,
		utils.GetTokenMultiplier(decimalsShift),
		multiplyOnRemote,
		fundedKey,
	)

	// Add collateral for both NativeTokenDestinations
	utils.AddCollateralToERC20TokenHome(
		ctx,
		cChainInfo,
		erc20TokenHome,
		erc20TokenHomeAddress,
		exampleERC20,
		subnetAInfo.BlockchainID,
		nativeTokenRemoteAddressA,
		collateralAmountA,
		fundedKey,
	)

	utils.AddCollateralToERC20TokenHome(
		ctx,
		cChainInfo,
		erc20TokenHome,
		erc20TokenHomeAddress,
		exampleERC20,
		subnetBInfo.BlockchainID,
		nativeTokenRemoteAddressB,
		collateralAmountB,
		fundedKey,
	)

	// Generate new recipient to receive transferred tokens
	recipientKey, err := crypto.GenerateKey()
	Expect(err).Should(BeNil())
	recipientAddress := crypto.PubkeyToAddress(recipientKey.PublicKey)

	// These are set during the initial transferring, and used in the multi-hop transfers
	amount := big.NewInt(0).Mul(big.NewInt(1e18), big.NewInt(10))

	// Send tokens from C-Chain to Subnet A
	inputA := erc20tokenhome.SendTokensInput{
		DestinationBlockchainID:            subnetAInfo.BlockchainID,
		DestinationTokenTransferrerAddress: nativeTokenRemoteAddressA,
		Recipient:                          recipientAddress,
		PrimaryFeeTokenAddress:             exampleERC20Address,
		PrimaryFee:                         big.NewInt(1e18),
		SecondaryFee:                       big.NewInt(0),
		RequiredGasLimit:                   utils.DefaultNativeTokenRequiredGas,
	}

	receipt, transferredAmountA := utils.SendERC20TokenHome(
		ctx,
		cChainInfo,
		erc20TokenHome,
		erc20TokenHomeAddress,
		exampleERC20,
		inputA,
		amount,
		fundedKey,
	)

	// Relay the message to subnet A and check for a native token mint withdrawal
	teleporter.RelayTeleporterMessage(
		ctx,
		receipt,
		cChainInfo,
		subnetAInfo,
		true,
		fundedKey,
	)

	// Verify the recipient received the tokens
	utils.CheckBalance(ctx, recipientAddress, transferredAmountA, subnetAInfo.RPCClient)

	// Send tokens from C-Chain to Subnet B
	inputB := erc20tokenhome.SendTokensInput{
		DestinationBlockchainID:            subnetBInfo.BlockchainID,
		DestinationTokenTransferrerAddress: nativeTokenRemoteAddressB,
		Recipient:                          recipientAddress,
		PrimaryFeeTokenAddress:             exampleERC20Address,
		PrimaryFee:                         big.NewInt(1e18),
		SecondaryFee:                       big.NewInt(0),
		RequiredGasLimit:                   utils.DefaultNativeTokenRequiredGas,
	}

	receipt, transferredAmountB := utils.SendERC20TokenHome(
		ctx,
		cChainInfo,
		erc20TokenHome,
		erc20TokenHomeAddress,
		exampleERC20,
		inputB,
		amount,
		fundedKey,
	)

	// Relay the message to subnet B and check for a native token mint withdrawal
	teleporter.RelayTeleporterMessage(
		ctx,
		receipt,
		cChainInfo,
		subnetBInfo,
		true,
		fundedKey,
	)

	// Verify the recipient received the tokens
	utils.CheckBalance(ctx, recipientAddress, transferredAmountB, subnetBInfo.RPCClient)

	// Multi-hop transfer to Subnet B
	// Send half of the received amount to account for gas expenses
	amountToSend := new(big.Int).Div(transferredAmountA, big.NewInt(2))

	utils.SendNativeMultiHopAndVerify(
		ctx,
		teleporter,
		fundedKey,
		recipientAddress,
		subnetAInfo,
		nativeTokenRemoteA,
		nativeTokenRemoteAddressA,
		subnetBInfo,
		nativeTokenRemoteB,
		nativeTokenRemoteAddressB,
		cChainInfo,
		amountToSend,
		big.NewInt(0),
	)

	// Multi-hop transfer back to Subnet A
	secondaryFeeAmount := new(big.Int).Div(amountToSend, big.NewInt(4))
	utils.SendNativeMultiHopAndVerify(
		ctx,
		teleporter,
		fundedKey,
		recipientAddress,
		subnetBInfo,
		nativeTokenRemoteB,
		nativeTokenRemoteAddressB,
		subnetAInfo,
		nativeTokenRemoteA,
		nativeTokenRemoteAddressA,
		cChainInfo,
		amountToSend,
		secondaryFeeAmount,
	)
}
