// // (c) 2024, Ava Labs, Inc. All rights reserved.
// // See the file LICENSE for licensing terms.

package teleporter

import (
	"math/big"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/subnet-evm/accounts/abi/bind"
	localnetwork "github.com/ava-labs/teleporter/tests/network"
	"github.com/ava-labs/teleporter/tests/utils"
	teleporterutils "github.com/ava-labs/teleporter/utils/teleporter-utils"
	"github.com/ethereum/go-ethereum/common"
	. "github.com/onsi/gomega"
)

// Tests Teleporter message ID calculation
func CalculateMessageID(network *localnetwork.LocalNetwork, teleporter utils.TeleporterTestInfo) {
	subnetInfo := network.GetPrimaryNetworkInfo()
	teleporterContractAddress := teleporter.TeleporterMessengerAddress(subnetInfo)

	sourceBlockchainID := common.HexToHash("0xabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcd")
	destinationBlockchainID := common.HexToHash("0x1234567812345678123456781234567812345678123456781234567812345678")
	nonce := big.NewInt(42)

	expectedMessageID, err := teleporter.TeleporterMessenger(subnetInfo).CalculateMessageID(
		&bind.CallOpts{},
		sourceBlockchainID,
		destinationBlockchainID,
		nonce,
	)
	Expect(err).Should(BeNil())

	calculatedMessageID, err := teleporterutils.CalculateMessageID(
		teleporterContractAddress,
		ids.ID(sourceBlockchainID),
		ids.ID(destinationBlockchainID),
		nonce,
	)
	Expect(err).Should(BeNil())
	Expect(ids.ID(expectedMessageID)).Should(Equal(calculatedMessageID))
}
