package geth

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

func TestERC20Change(t *testing.T) {
	rpc := rpcURL // 或 archive RPC
	client, err := ethclient.Dial(rpc)
	if err != nil {
		t.Fatal(err)
	}
	user := "0x6dcad62fc4cf34a8a1ad812d99ca861f4aaadc26"
	token := common.HexToAddress("0x81663d5149cADBbc48CF1a7F21b05719Ee1420A9") // USDT / USDC 等
	blockNum := big.NewInt(73766674)                                           // 交易所在区块
	volumeERC20, err := ERC20Volume(client, token, common.HexToAddress(user), blockNum)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("ERC20 swap volume:", volumeERC20.String())
}
