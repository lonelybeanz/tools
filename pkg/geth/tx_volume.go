package geth

import (
	"context"
	"fmt"
	"math"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

var (
	BNB = common.HexToAddress("0xEeeeeEeeeEeEeeEeEeEeeEEEeeeeEeeeeeeeEEeE")
)

/* ============================================================
 * Stage 0: 基础数据结构
 * ============================================================
 */

type NativeTransfer struct {
	From   common.Address
	To     common.Address
	Amount *big.Int
}

func ParseNativeFromTrace(root *TraceCall) []*NativeTransfer {
	var out []*NativeTransfer

	var walk func(c *TraceCall)
	walk = func(c *TraceCall) {
		value := HexToBigInt(c.Value)
		if value != nil && value.Sign() > 0 {
			out = append(out, &NativeTransfer{
				From:   common.HexToAddress(c.From),
				To:     common.HexToAddress(c.To),
				Amount: new(big.Int).Set(value),
			})
		}
		for _, sub := range c.Calls {
			walk(sub)
		}
	}

	walk(root)
	return out
}

type AssetChange struct {
	Tokens map[common.Address]*big.Int // tokenAddress -> amount
}

func CalculateTransactionVolume(
	logs []*types.Log,
	traceRoot *TraceCall,
) map[common.Address]*AssetChange {

	changes := make(map[common.Address]*AssetChange)

	transfer := parseTxLogs(context.Background(), logs)

	transferTracker := NewTransferTracker("")
	for _, v := range transfer {
		for _, vv := range v {
			transferTracker.AddTransfer(vv.From, vv.To, vv.Token, vv.Amount)
		}
	}

	nativeCalls := ParseNativeFromTrace(traceRoot)
	for _, v := range nativeCalls {
		transferTracker.AddTransfer(v.From, v.To, BNB, v.Amount)
	}

	for _, vv := range transferTracker.GetAllAccounts() {
		fmt.Println("------------------------------------------")
		fmt.Println("Address:", vv)
		tokenChanges := &AssetChange{
			Tokens: make(map[common.Address]*big.Int),
		}
		for _, v := range transferTracker.GetAllTokens() {
			net := transferTracker.GetNetBalance(vv, v)
			if net.Cmp(big.NewInt(0)) == 0 {
				continue
			}
			tokenChanges.Tokens[v] = net
			fmt.Println("  Token:", v, "change:", net.String())
		}
		fmt.Println("------------------------------------------")
		if len(tokenChanges.Tokens) == 0 {
			continue
		}
		changes[vv] = tokenChanges
	}
	return changes
}

// ---------------- SwapVolume ----------------
// tokenPrice: tokenAddress -> USD 价格
// 稳定币直接填 1
func SwapVolume(changes map[common.Address]*AssetChange, tokenPrice map[common.Address]float64) float64 {
	maxValue := 0.0

	for _, ac := range changes {
		for token, amt := range ac.Tokens {
			price, ok := tokenPrice[token]
			if !ok {
				price = 0.0
			}
			// Use big.Float for calculation to avoid overflow
			v := new(big.Float).Quo(new(big.Float).SetInt(amt), new(big.Float).SetFloat64(1e18))
			v.Mul(v, new(big.Float).SetFloat64(price))
			value, _ := v.Float64()
			value = math.Abs(value)
			if value > maxValue {
				maxValue = value
			}
		}
	}

	return maxValue
}
