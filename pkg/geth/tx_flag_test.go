package geth

import (
	"context"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

func TestTxFlag(t *testing.T) {
	client, err := ethclient.Dial("https://bsc-mainnet.core.chainstack.com/8584b635eccbec059338b0095fbe83d2")
	if err != nil {
		t.Log(err)
	}
	txHash := common.HexToHash("0xc7083ac5dbfdd8a0f1ee8315ae08b59091e78a304732ac4fe3e3bd8d2d5473a5")
	tx, _, err := client.TransactionByHash(context.Background(), txHash)
	if err != nil {
		t.Log(err)
	}

	// 获取交易回执
	receipt, err := client.TransactionReceipt(context.Background(), txHash)
	if err != nil {
		t.Log(err)
	}

	// 解析交易回执中的日志
	logs := receipt.Logs

	flag := GetTxFlag(logs, tx.To().Hex(), tx.Data())
	t.Log(flag)

}
