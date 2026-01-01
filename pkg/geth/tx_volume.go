package geth

import (
	"context"
	"fmt"
	"math"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/lonelybeanz/tools/pkg/log"
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

func ParseNativeChange(balanceChangeResult *PrestateTxResult) map[common.Address]*AssetChange {
	changes := make(map[common.Address]*AssetChange)
	if balanceChangeResult.Result == nil {
		return changes
	}
	pre := balanceChangeResult.Result.Pre
	post := balanceChangeResult.Result.Post
	for address, v := range pre {
		addressPost := post[address]
		if addressPost.Balance == "" {
			continue
		}
		change := new(big.Int).Sub(HexToBigInt(addressPost.Balance), HexToBigInt(v.Balance))
		log.Debugf("address:%s %s -> %s", address, v.Balance, addressPost.Balance)
		if change.Sign() != 0 {
			changes[common.HexToAddress(address)] = &AssetChange{
				Tokens: map[common.Address]*big.Int{
					BNB.Address: change,
				},
			}
		}

	}
	return changes
}

type AssetChange struct {
	Tokens map[common.Address]*big.Int // tokenAddress -> amount
}

func CalculateTransactionVolume(
	logs []*types.Log,
	traceRoot *TraceCall,
) map[common.Address]*AssetChange {

	changes := make(map[common.Address]*AssetChange)

	transfer, _ := parseTxLogs(context.Background(), logs)

	transferTracker := NewTransferTracker("")
	for _, v := range transfer {
		for _, vv := range v {
			transferTracker.AddTransfer(vv.From, vv.To, vv.Token, vv.Amount)
		}
	}

	nativeCalls := ParseNativeFromTrace(traceRoot)
	for _, v := range nativeCalls {
		transferTracker.AddTransfer(v.From, v.To, BNB.Address, v.Amount)
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

func CalculateTransactionTokenBalanceChanges(
	logs []*types.Log,
	balanceChangeResult *PrestateTxResult,
) (
	map[common.Address]*AssetChange,
	map[common.Hash]bool,
) {
	var changes map[common.Address]*AssetChange

	// 解析 ERC20 代币转账
	transfer, swapHashs := parseTxLogs(context.Background(), logs)

	// 创建转账追踪器
	transferTracker := NewTransferTracker("")
	for _, v := range transfer {
		for _, vv := range v {
			transferTracker.AddTransfer(vv.From, vv.To, vv.Token, vv.Amount)
		}
	}

	// 从 balance change 计算原生代币转账
	changes = ParseNativeChange(balanceChangeResult)

	// 遍历所有涉及的账户，计算余额变化
	for _, addr := range transferTracker.GetAllAccounts() {
		tokenChanges := &AssetChange{
			Tokens: make(map[common.Address]*big.Int),
		}

		// 获取该账户所有代币的净余额变化
		for _, token := range transferTracker.GetAllTokens() {
			netBalance := transferTracker.GetNetBalance(addr, token)
			if netBalance.Cmp(big.NewInt(0)) != 0 {
				tokenChanges.Tokens[token] = netBalance
			}
		}

		if len(tokenChanges.Tokens) > 0 {
			changes[addr] = tokenChanges
		}
	}

	return changes, swapHashs
}

// ---------------- MaxSwapVolumeUSD ----------------
// tokenPrice: tokenAddress -> USD 价格
// 稳定币直接填 1
func MaxSwapVolumeUSD(changes map[common.Address]*AssetChange, tokenPrice map[common.Address]*TokenPrice) float64 {
	maxValue := 0.0

	for _, ac := range changes {
		var price float64
		for token, amt := range ac.Tokens {
			tokenPrice, ok := tokenPrice[token]
			if !ok {
				continue
			}
			price = tokenPrice.Price
			baseDecimal := tokenPrice.Decimal

			log.Debugf("amt: %s,token: %+v", amt.String(), tokenPrice)

			v := new(big.Float).Quo(new(big.Float).SetInt(amt), new(big.Float).SetFloat64(math.Pow10(baseDecimal)))
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
