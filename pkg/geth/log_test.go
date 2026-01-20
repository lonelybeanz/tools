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

	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		t.Log(err)
	}
	txHash := "0xc54d1c314aa964c012d25dadbcc1c49fb5702b0aab2625c1f70abd2af137a260"
	// 获取交易回执
	receipt, err := client.TransactionReceipt(context.Background(), common.HexToHash(txHash))
	if err != nil {
		t.Log(err)
	}

	// 解析交易回执中的日志
	logs := receipt.Logs

	transfer, _ := parseTxLogs(context.Background(), logs)

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
		transferTracker.AddTransfer(v.From, v.To, BNB.Address, v.Amount)
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

	// --- Generate and print flow diagram ---
	tokenDetails := make(map[common.Address]*TokenPrice)
	allTokens := transferTracker.GetAllTokens()
	// A simple way to populate some known tokens.
	// For a real application, you might have a more robust way to get token info.
	knownTokens := map[common.Address]*TokenPrice{
		BNB.Address:  &BNB,
		WBNB.Address: &WBNB,
		USDT.Address: &USDT,
		USDC.Address: &USDC,
	}
	for _, tokenAddr := range allTokens {
		if details, ok := knownTokens[tokenAddr]; ok {
			tokenDetails[tokenAddr] = details
		}
	}

	fmt.Println("\n--- Finding Original Sources ---")
	for _, account := range transferTracker.GetAllAccounts() {
		for _, token := range transferTracker.GetAllTokens() {
			// We only care about accounts that had a net positive balance
			if transferTracker.GetNetBalance(account, token).Sign() > 0 {
				sources := transferTracker.FindOriginalSources(account, token)

				// For cleaner output, let's get token symbol
				var tokenSymbol string
				if details, ok := tokenDetails[token]; ok {
					tokenSymbol = details.Symbol
				} else {
					tokenSymbol = token.Hex()
				}

				// Format sources for printing
				var sourceStrs []string
				for _, s := range sources {
					sourceStrs = append(sourceStrs, s.Hex())
				}

				fmt.Printf("Sources for %s receiving %s: %v\n", account.Hex(), tokenSymbol, sourceStrs)
				fmt.Println("------------------------------------------")
				fmt.Println("\n--- Finding Final Destinations ---")
				destinations := transferTracker.FindFinalDestinations(account, token)
				var destStrs []string
				for _, d := range destinations {
					destStrs = append(destStrs, d.Hex())
				}
				fmt.Printf("Final Destinations for %s receiving %s: %v\n", account.Hex(), tokenSymbol, destStrs)
				fmt.Println("------------------------------------------")

			}
		}
	}

	// --- Generate and print flow diagram ---
	tokenDetails = make(map[common.Address]*TokenPrice)
	allTokens = transferTracker.GetAllTokens()
	// A simple way to populate some known tokens.
	// For a real application, you might have a more robust way to get token info.
	knownTokens = map[common.Address]*TokenPrice{
		BNB.Address:  &BNB,
		WBNB.Address: &WBNB,
		USDT.Address: &USDT,
		USDC.Address: &USDC,
	}
	for _, tokenAddr := range allTokens {
		if details, ok := knownTokens[tokenAddr]; ok {
			tokenDetails[tokenAddr] = details
		}
	}

	dotGraph := transferTracker.ToDOT(tokenDetails)
	fmt.Println("\n--- Transfer Graph (DOT format) ---")
	fmt.Println(dotGraph)
	fmt.Println("--- End of Graph ---")
	fmt.Println("\n提示: 复制以上DOT格式的文本并粘贴到Graphviz在线渲染工具中（如: https://dreampuf.github.io/GraphvizOnline/）即可查看可视化流转图。")
}
