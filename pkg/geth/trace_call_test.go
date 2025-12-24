package geth

import (
	"fmt"
	"testing"

	"strings"

	"github.com/lonelybeanz/tools/pkg/log"
)

func TestCall(t *testing.T) {
	txHash := "0x4a137bbc86195026718d422fbf57cd5c0e0b3977a68c5ccf890bed1b70fa3d1e"
	traces, err := TraceTransaction(rpcURL, txHash)
	if err != nil {
		log.Errorf("trace_transaction 错误:%v", err)
	}

	changes := TransactionAssetChanges(*traces)

	for addr, ac := range changes {
		fmt.Println("Address:", addr)
		fmt.Println("  BNB change:", ac.BNB.String())
		for token, amt := range ac.Tokens {
			fmt.Println("  Token:", token, "change:", amt.String())
		}
	}

}

func TestSwapVolume(t *testing.T) {
	txHash := "0xc7083ac5dbfdd8a0f1ee8315ae08b59091e78a304732ac4fe3e3bd8d2d5473a5"
	traces, err := TraceTransaction(rpcURL, txHash)
	if err != nil {
		log.Errorf("trace_transaction 错误:%v", err)
	}

	changes := TransactionAssetChanges(*traces)

	// 定义价格 map
	tokenPrice := map[string]float64{
		strings.ToLower("0x55d398326f99059fF775485246999027B3197955"): 1.0,
		strings.ToLower("0xbb4CdB9CBd36B01bD1cBaEBF2De08d9173bc095c"): 837.0,
	}

	swapVolume := SwapVolume(changes, tokenPrice, 837.0)

	fmt.Println("Transaction swap volume (USD):", swapVolume)

	fmt.Println("Address changes:")
	for addr, ac := range changes {
		fmt.Println("Address:", addr)
		fmt.Println("  BNB change:", ac.BNB.String())
		for token, amt := range ac.Tokens {
			fmt.Println("  Token:", token, "change:", amt.String())
		}
	}
}
