package geth

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

// ERC20Volume 查询 ERC20 在指定区块前后的差值
func ERC20Volume(client *ethclient.Client, token common.Address, user common.Address, blockNumber *big.Int) (*big.Int, error) {
	// balanceOf 方法签名 70a08231
	data := append(common.FromHex("0x70a08231"), common.LeftPadBytes(user.Bytes(), 32)...)

	// balance before
	msg := ethereum.CallMsg{To: &token, Data: data}
	balBefore, err := client.CallContract(context.Background(), msg, new(big.Int).Sub(blockNumber, big.NewInt(1)))
	if err != nil {
		return nil, err
	}
	before := new(big.Int).SetBytes(balBefore)

	// balance after
	balAfter, err := client.CallContract(context.Background(), msg, blockNumber)
	if err != nil {
		return nil, err
	}
	after := new(big.Int).SetBytes(balAfter)

	diff := new(big.Int).Sub(after, before)
	return diff.Abs(diff), nil
}
