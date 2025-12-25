package geth

import (
	"context"
	"fmt"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/lonelybeanz/tools/pkg/log"
)

func TestTxVolume(t *testing.T) {
	client, err := ethclient.Dial("https://bsc-mainnet.core.chainstack.com/880")
	if err != nil {
		t.Log(err)
	}
	txHash := "0xc7083ac5dbfdd8a0f1ee8315ae08b59091e78a304732ac4fe3e3bd8d2d5473a5"
	// 获取交易回执
	receipt, err := client.TransactionReceipt(context.Background(), common.HexToHash(txHash))
	if err != nil {
		t.Log(err)
	}

	// 解析交易回执中的日志
	logs := receipt.Logs

	traces, err := TraceTransaction(rpcURL, txHash)
	if err != nil {
		log.Errorf("trace_transaction 错误:%v", err)
	}
	vR := CalculateTransactionVolume(logs, traces)
	t.Log(vR)

}

func TestSwapVolume(t *testing.T) {
	client, err := ethclient.Dial("https://bsc-mainnet.core.chainstack.com/880")
	if err != nil {
		t.Log(err)
	}
	txHash := "0x3bbc751d4466f6287faf4955429d12cd0f99c02209d454ac0bf139434396f06c"
	// 获取交易回执
	receipt, err := client.TransactionReceipt(context.Background(), common.HexToHash(txHash))
	if err != nil {
		t.Log(err)
	}

	// 解析交易回执中的日志
	logs := receipt.Logs

	traces, err := TraceTransaction(rpcURL, txHash)
	if err != nil {
		log.Errorf("trace_transaction 错误:%v", err)
	}
	changes := CalculateTransactionVolume(logs, traces)
	t.Log(changes)

	// 定义价格 map
	tokenPrice := map[common.Address]float64{
		common.HexToAddress("0x8AC76a51cc950d9822D68b5bA99a72108E0598D6"): 1.0,
		common.HexToAddress("0x55d398326f99059fF775485246999027B3197955"): 1.0,
		BNB: 837.0,
		common.HexToAddress("0xbb4CdB9CBd36B01bD1cBaEBF2De08d9173bc095c"): 837.0,
	}

	swapVolume := SwapVolume(changes, tokenPrice)

	fmt.Println("Transaction swap volume (USD):", swapVolume)
}
