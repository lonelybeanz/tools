package geth

import (
	"context"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

func TestTxFlag(t *testing.T) {
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		t.Log(err)
	}
	txHash := common.HexToHash("0xe68013ffc1dd79dba56c2747bb97ef32fd4e7f29c6e88eaed518720bff590a6e")
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
