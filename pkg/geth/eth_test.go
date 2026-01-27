package geth

import (
	"context"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

func TestGetTransactionReceipt(t *testing.T) {
	client, _ := ethclient.Dial("https://docs-demo.bsc.quiknode.pro")
	txHash := common.HexToHash("0x7e22ba22e143e18576635db193488417c291c8c143d908c6cc843b708be73fac")
	receipt, err := GetTransactionReceipt(context.Background(), client, txHash)
	if err != nil {
		t.Fatalf("Failed to get receipt for %s: %v", txHash.Hex(), err)
	}
	t.Logf("Transaction %s: Successs=%d, Logs Count=%d\n", txHash.Hex(), receipt.Status, len(receipt.Logs))
}

func TestGetTransactionByHash(t *testing.T) {
	client, _ := ethclient.Dial("https://docs-demo.bsc.quiknode.pro")
	txHash := common.HexToHash("0x7e22ba22e143e18576635db193488417c291c8c143d908c6cc843b708be73fac")
	tx, err := GetTransactionByHash(context.Background(), client, txHash)
	if err != nil {
		t.Fatalf("Failed to get receipt for %s: %v", txHash.Hex(), err)
	}
	t.Logf("Transaction %s: To=%s, TxHash=%s\n", txHash.Hex(), tx.To(), tx.Hash().Hex())
}
