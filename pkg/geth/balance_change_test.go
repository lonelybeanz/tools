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
	user := "0xF7E64B41AE1c04F15f6c35493Bc5f8c99E505Ad5"
	token := common.HexToAddress("0x2c6579c11027f93e13F21Da25C2Acf2B1709b499") // USDT / USDC 等
	blockNum := big.NewInt(72751992)                                           // 交易所在区块
	volumeERC20, err := ERC20Volume(client, token, common.HexToAddress(user), blockNum)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("ERC20 swap volume:", volumeERC20.String())
}
