package ictt

import (
	"context"
	"math/big"

	"github.com/ava-labs/subnet-evm/accounts/abi/bind"
	erc20tokenhome "github.com/ava-labs/teleporter/abi-bindings/go/ictt/TokenHome/ERC20TokenHome"
	erc20tokenremote "github.com/ava-labs/teleporter/abi-bindings/go/ictt/TokenRemote/ERC20TokenRemote"
	localnetwork "github.com/ava-labs/teleporter/tests/network"
	"github.com/ava-labs/teleporter/tests/utils"
	"github.com/ethereum/go-ethereum/crypto"
	. "github.com/onsi/gomega"
)

/**
 * Deploy an ERC20TokenHome on the primary network
 * Deploys ERC20TokenRemote to Subnet A
 * Transfers C-Chain example ERC20 tokens to Subnet A
 * Transfer tokens from Subnet A to C-Chain
 */
func ERC20TokenHomeERC20TokenRemote(network *localnetwork.LocalNetwork, teleporter utils.TeleporterTestInfo) {
	cChainInfo := network.GetPrimaryNetworkInfo()
	subnetAInfo, _ := network.GetTwoSubnets()
	fundedAddress, fundedKey := network.GetFundedAccountInfo()

	ctx := context.Background()

	// Deploy an ExampleERC20 on the primary network as the token to be transferred
	exampleERC20Address, exampleERC20 := utils.DeployExampleERC20Decimals(
		ctx,
		fundedKey,
		cChainInfo,
		erc20TokenHomeDecimals,
	)

	exampleERC20Decimals, err := exampleERC20.Decimals(&bind.CallOpts{})
	Expect(err).Should(BeNil())

	// Create an ERC20TokenHome for transferring the ERC20 token
	erc20TokenHomeAddress, erc20TokenHome := utils.DeployERC20TokenHome(
		ctx,
		teleporter,
		fundedKey,
		cChainInfo,
		fundedAddress,
		exampleERC20Address,
		exampleERC20Decimals,
	)

	// Token representation on subnet A will have same name, symbol, and decimals
	tokenName, err := exampleERC20.Name(&bind.CallOpts{})
	Expect(err).Should(BeNil())
	tokenSymbol, err := exampleERC20.Symbol(&bind.CallOpts{})
	Expect(err).Should(BeNil())
	tokenDecimals, err := exampleERC20.Decimals(&bind.CallOpts{})
	Expect(err).Should(BeNil())

	// Deploy an ERC20TokenRemote to Subnet A
	erc20TokenRemoteAddress, erc20TokenRemote := utils.DeployERC20TokenRemote(
		ctx,
		teleporter,
		fundedKey,
		subnetAInfo,
		fundedAddress,
		cChainInfo.BlockchainID,
		erc20TokenHomeAddress,
		exampleERC20Decimals,
		tokenName,
		tokenSymbol,
		tokenDecimals,
	)

	utils.RegisterERC20TokenRemoteOnHome(
		ctx,
		teleporter,
		cChainInfo,
		erc20TokenHomeAddress,
		subnetAInfo,
		erc20TokenRemoteAddress,
		fundedKey,
	)

	// Generate new recipient to receive transferred tokens
	recipientKey, err := crypto.GenerateKey()
	Expect(err).Should(BeNil())
	recipientAddress := crypto.PubkeyToAddress(recipientKey.PublicKey)

	// Send tokens from C-Chain to recipient on subnet A
	input := erc20tokenhome.SendTokensInput{
		DestinationBlockchainID:            subnetAInfo.BlockchainID,
		DestinationTokenTransferrerAddress: erc20TokenRemoteAddress,
		Recipient:                          recipientAddress,
		PrimaryFeeTokenAddress:             exampleERC20Address,
		PrimaryFee:                         big.NewInt(1e18),
		SecondaryFee:                       big.NewInt(0),
		RequiredGasLimit:                   utils.DefaultERC20RequiredGas,
	}
	amount := new(big.Int).Mul(big.NewInt(1e18), big.NewInt(13))

	receipt, transferredAmount := utils.SendERC20TokenHome(
		ctx,
		cChainInfo,
		erc20TokenHome,
		erc20TokenHomeAddress,
		exampleERC20,
		input,
		amount,
		fundedKey,
	)

	// Relay the message to Subnet A and check for message delivery
	receipt = teleporter.RelayTeleporterMessage(
		ctx,
		receipt,
		cChainInfo,
		subnetAInfo,
		true,
		fundedKey,
	)

	utils.CheckERC20TokenRemoteWithdrawal(
		ctx,
		erc20TokenRemote,
		receipt,
		recipientAddress,
		transferredAmount,
	)

	// Check that the recipient received the tokens
	balance, err := erc20TokenRemote.BalanceOf(&bind.CallOpts{}, recipientAddress)
	Expect(err).Should(BeNil())
	Expect(balance).Should(Equal(transferredAmount))

	// Transfer back to home chain
	// Fund recipient with gas tokens on subnet A
	utils.SendNativeTransfer(
		ctx,
		subnetAInfo,
		fundedKey,
		recipientAddress,
		big.NewInt(1e18),
	)
	inputB := erc20tokenremote.SendTokensInput{
		DestinationBlockchainID:            cChainInfo.BlockchainID,
		DestinationTokenTransferrerAddress: erc20TokenHomeAddress,
		Recipient:                          recipientAddress,
		PrimaryFeeTokenAddress:             erc20TokenRemoteAddress,
		PrimaryFee:                         big.NewInt(1e10),
		SecondaryFee:                       big.NewInt(0),
		RequiredGasLimit:                   utils.DefaultERC20RequiredGas,
	}

	receipt, transferredAmount = utils.SendERC20TokenRemote(
		ctx,
		subnetAInfo,
		erc20TokenRemote,
		erc20TokenRemoteAddress,
		inputB,
		utils.BigIntSub(transferredAmount, inputB.PrimaryFee),
		recipientKey,
	)

	receipt = teleporter.RelayTeleporterMessage(
		ctx,
		receipt,
		subnetAInfo,
		cChainInfo,
		true,
		fundedKey,
	)

	utils.CheckERC20TokenHomeWithdrawal(
		ctx,
		erc20TokenHomeAddress,
		exampleERC20,
		receipt,
		recipientAddress,
		transferredAmount,
	)

	// Check that the recipient received the tokens
	balance, err = exampleERC20.BalanceOf(&bind.CallOpts{}, recipientAddress)
	Expect(err).Should(BeNil())
	Expect(balance).Should(Equal(transferredAmount))
}
