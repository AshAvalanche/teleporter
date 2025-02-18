package teleporter

import (
	"context"
	"math/big"

	"github.com/ava-labs/subnet-evm/accounts/abi/bind"
	testmessenger "github.com/ava-labs/teleporter/abi-bindings/go/teleporter/tests/TestMessenger"
	localnetwork "github.com/ava-labs/teleporter/tests/network"
	"github.com/ava-labs/teleporter/tests/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	. "github.com/onsi/gomega"
)

func DeliverToNonExistentContract(network *localnetwork.LocalNetwork, teleporter utils.TeleporterTestInfo) {
	subnetAInfo := network.GetPrimaryNetworkInfo()
	subnetBInfo, _ := network.GetTwoSubnets()
	fundedAddress, fundedKey := network.GetFundedAccountInfo()

	deployerKey, err := crypto.GenerateKey()
	Expect(err).Should(BeNil())
	deployerAddress := crypto.PubkeyToAddress(deployerKey.PublicKey)

	//
	// Fund the deployer address on Subnet B
	//
	ctx := context.Background()
	log.Info("Funding address on subnet B", "address", deployerAddress.Hex())

	fundAmount := big.NewInt(0).Mul(big.NewInt(1e18), big.NewInt(10)) // 10eth
	fundDeployerTx := utils.CreateNativeTransferTransaction(
		ctx, subnetBInfo, fundedKey, deployerAddress, fundAmount,
	)
	utils.SendTransactionAndWaitForSuccess(ctx, subnetBInfo, fundDeployerTx)

	//
	// Deploy ExampleMessenger to Subnet A, but not to Subnet B
	// Send a message that should fail to be executed on Subnet B
	//
	log.Info("Deploying ExampleMessenger to Subnet A")
	_, subnetAExampleMessenger := utils.DeployTestMessenger(
		ctx,
		fundedKey,
		fundedAddress,
		teleporter.TeleporterRegistryAddress(subnetAInfo),
		subnetAInfo,
	)

	// Derive the eventual address of the destination contract on Subnet B
	nonce, err := subnetBInfo.RPCClient.NonceAt(ctx, deployerAddress, nil)
	Expect(err).Should(BeNil())
	destinationContractAddress := crypto.CreateAddress(deployerAddress, nonce)

	//
	// Call the example messenger contract on Subnet A
	//
	log.Info("Calling ExampleMessenger on Subnet A")
	message := "Hello, world!"
	optsA, err := bind.NewKeyedTransactorWithChainID(
		fundedKey, subnetAInfo.EVMChainID)
	Expect(err).Should(BeNil())
	tx, err := subnetAExampleMessenger.SendMessage(
		optsA,
		subnetBInfo.BlockchainID,
		destinationContractAddress,
		common.BigToAddress(common.Big0),
		big.NewInt(0),
		testmessenger.SendMessageRequiredGas,
		message,
	)
	Expect(err).Should(BeNil())

	// Wait for the transaction to be mined
	receipt := utils.WaitForTransactionSuccess(ctx, subnetAInfo, tx.Hash())

	sendEvent, err := utils.GetEventFromLogs(
		receipt.Logs,
		teleporter.TeleporterMessenger(subnetAInfo).ParseSendCrossChainMessage,
	)
	Expect(err).Should(BeNil())
	Expect(sendEvent.DestinationBlockchainID[:]).Should(Equal(subnetBInfo.BlockchainID[:]))

	teleporterMessageID := sendEvent.MessageID

	//
	// Relay the message to the destination
	//
	log.Info("Relaying the message to the destination")
	receipt = teleporter.RelayTeleporterMessage(ctx, receipt, subnetAInfo, subnetBInfo, true, fundedKey)
	receiveEvent, err :=
		utils.GetEventFromLogs(receipt.Logs, teleporter.TeleporterMessenger(subnetAInfo).ParseReceiveCrossChainMessage)
	Expect(err).Should(BeNil())

	//
	// Check that the message was successfully relayed
	//
	log.Info("Checking the message was successfully relayed")
	delivered, err := teleporter.TeleporterMessenger(subnetBInfo).MessageReceived(
		&bind.CallOpts{},
		teleporterMessageID,
	)
	Expect(err).Should(BeNil())
	Expect(delivered).Should(BeTrue())

	//
	// Check that the message was not successfully executed
	//
	log.Info("Checking the message was not successfully executed")
	executionFailedEvent, err := utils.GetEventFromLogs(
		receipt.Logs,
		teleporter.TeleporterMessenger(subnetAInfo).ParseMessageExecutionFailed,
	)
	Expect(err).Should(BeNil())
	Expect(executionFailedEvent.MessageID).Should(Equal(receiveEvent.MessageID))

	//
	// Deploy the contract on Subnet B
	//
	log.Info("Deploying the contract on Subnet B")
	exampleMessengerContractB, subnetBExampleMessenger := utils.DeployTestMessenger(
		ctx,
		deployerKey,
		deployerAddress,
		teleporter.TeleporterRegistryAddress(subnetBInfo),
		subnetBInfo,
	)

	// Confirm that it was deployed at the expected address
	Expect(exampleMessengerContractB).Should(Equal(destinationContractAddress))

	//
	// Call retryMessageExecution on Subnet B
	//
	log.Info("Calling retryMessageExecution on Subnet B")
	receipt = utils.RetryMessageExecutionAndWaitForAcceptance(
		ctx,
		subnetAInfo.BlockchainID,
		teleporter.TeleporterMessenger(subnetBInfo),
		subnetBInfo,
		receiveEvent.Message,
		fundedKey,
	)
	log.Info("Checking the message was successfully executed")
	messageExecutedEvent, err := utils.GetEventFromLogs(
		receipt.Logs,
		teleporter.TeleporterMessenger(subnetAInfo).ParseMessageExecuted,
	)
	Expect(err).Should(BeNil())
	Expect(messageExecutedEvent.MessageID).Should(Equal(receiveEvent.MessageID))

	//
	// Verify we received the expected string
	//
	log.Info("Verifying we received the expected string")
	_, currMessage, err := subnetBExampleMessenger.GetCurrentMessage(&bind.CallOpts{}, subnetAInfo.BlockchainID)
	Expect(err).Should(BeNil())
	Expect(currMessage).Should(Equal(message))
}
