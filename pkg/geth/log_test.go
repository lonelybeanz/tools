package geth

import (
	"context"
	"fmt"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/lonelybeanz/tools/pkg/log"
)

func TestParseTxLogs(t *testing.T) {

	client, err := ethclient.Dial("https://bsc-mainnet.core.chainstack.com/880")
	if err != nil {
		t.Log(err)
	}
	txHash := "0xf6a17ef264df100099f74c5b209eb2e5d2a5324f8148ccffa814442fd33f5bf3"
	// 获取交易回执
	receipt, err := client.TransactionReceipt(context.Background(), common.HexToHash(txHash))
	if err != nil {
		t.Log(err)
	}

	// 解析交易回执中的日志
	logs := receipt.Logs

	transfer := parseTxLogs(context.Background(), logs)

	t.Logf("transfers: %+v\n", transfer)

	transferTracker := NewTransferTracker(txHash)
	for _, v := range transfer {
		for _, vv := range v {
			transferTracker.AddTransfer(vv.From, vv.To, vv.Token, vv.Amount)
		}
	}

	traces, err := TraceTransaction(rpcURL, txHash)
	if err != nil {
		log.Errorf("trace_transaction 错误:%v", err)
	}
	nativeCalls := ParseNativeFromTrace(traces)
	for _, v := range nativeCalls {
		transferTracker.AddTransfer(v.From, v.To, BNB, v.Amount)
	}

	for _, vv := range transferTracker.GetAllAccounts() {
		fmt.Println("Address:", vv)
		for _, v := range transferTracker.GetAllTokens() {
			net := transferTracker.GetNetBalance(vv, v)
			if net.Cmp(big.NewInt(0)) == 0 {
				continue
			}
			fmt.Println("  Token:", v, "change:", net.String())
		}
		fmt.Println("------------------------------------------")
	}

}
