package geth

import (
	"encoding/hex"
	"math/big"
	"strings"
)

// ERC20 方法签名
var transferSig = "a9059cbb"     // transfer(address,uint256)
var transferFromSig = "23b872dd" // transferFrom(address,address,uint256)

// TraceCall 递归结构
type TraceCall struct {
	Type    string      `json:"type"`
	From    string      `json:"from"`
	To      string      `json:"to"`
	Value   string      `json:"value"`
	GasUsed string      `json:"gasUsed"`
	Input   string      `json:"input"` // ERC20 调用
	Error   string      `json:"error,omitempty"`
	Calls   []TraceCall `json:"calls,omitempty"`
}

// 递归找到所有的call
func (call *TraceCall) getAllCalls() []TraceCall {
	var calls []TraceCall
	calls = append(calls, *call)
	for _, c := range call.Calls {
		calls = append(calls, c.getAllCalls()...)
	}
	return calls
}

// AssetChange 记录每个地址的资产变化
type AssetChange struct {
	BNB    *big.Int
	Tokens map[string]*big.Int // tokenAddress -> amount
}

// ParseERC20Transfer 解析 transfer / transferFrom
// 返回真正的 owner 地址和接收地址
func ParseERC20Transfer(callFrom string, input string) (fromAddr string, toAddr string, amount *big.Int, ok bool) {
	if len(input) < 10 {
		return
	}
	sig := input[2:10]
	data, err := hex.DecodeString(input[10:])
	if err != nil {
		return
	}

	if sig == transferSig && len(data) >= 64 {
		fromAddr = callFrom
		toAddr = "0x" + hex.EncodeToString(data[0:32])[24:]
		amount = new(big.Int).SetBytes(data[32:])
		ok = true
	} else if sig == transferFromSig && len(data) >= 96 {
		fromAddr = "0x" + hex.EncodeToString(data[0:32])[24:]
		toAddr = "0x" + hex.EncodeToString(data[32:64])[24:]
		amount = new(big.Int).SetBytes(data[64:])
		ok = true
	}
	return
}

// ParseTrace 遍历 call 树，累加每个地址资产变化
func ParseTrace(call TraceCall, changes map[string]*AssetChange) {
	// 初始化 map
	if changes[call.From] == nil {
		changes[call.From] = &AssetChange{BNB: big.NewInt(0), Tokens: make(map[string]*big.Int)}
	}
	if changes[call.To] == nil {
		changes[call.To] = &AssetChange{BNB: big.NewInt(0), Tokens: make(map[string]*big.Int)}
	}

	// BNB
	if len(call.Value) > 2 {
		val := new(big.Int)
		val.SetString(call.Value[2:], 16)
		changes[call.From].BNB.Sub(changes[call.From].BNB, val)
		changes[call.To].BNB.Add(changes[call.To].BNB, val)
	}

	// ERC20
	fromAddr, toAddr, amount, ok := ParseERC20Transfer(call.From, call.Input)
	if ok {
		tokenAddr := strings.ToLower(call.To)
		// 初始化 map
		if changes[fromAddr] == nil {
			changes[fromAddr] = &AssetChange{BNB: big.NewInt(0), Tokens: make(map[string]*big.Int)}
		}
		if changes[toAddr] == nil {
			changes[toAddr] = &AssetChange{BNB: big.NewInt(0), Tokens: make(map[string]*big.Int)}
		}
		if changes[fromAddr].Tokens[tokenAddr] == nil {
			changes[fromAddr].Tokens[tokenAddr] = big.NewInt(0)
		}
		if changes[toAddr].Tokens[tokenAddr] == nil {
			changes[toAddr].Tokens[tokenAddr] = big.NewInt(0)
		}
		changes[fromAddr].Tokens[tokenAddr].Sub(changes[fromAddr].Tokens[tokenAddr], amount)
		changes[toAddr].Tokens[tokenAddr].Add(changes[toAddr].Tokens[tokenAddr], amount)
	}

	// 遍历内部调用
	for _, c := range call.Calls {
		ParseTrace(c, changes)
	}
}

// TransactionAssetChanges 返回每个地址的净资产变化
// 并过滤掉净资产变化为 0 的地址（Router / 聚合器等）
func TransactionAssetChanges(traceRoot TraceCall) map[string]*AssetChange {
	changes := make(map[string]*AssetChange)
	ParseTrace(traceRoot, changes)

	// 过滤净资产变化为 0 的地址
	for addr, ac := range changes {
		if ac.BNB.Sign() == 0 {
			allZero := true
			for _, amt := range ac.Tokens {
				if amt.Sign() != 0 {
					allZero = false
					break
				}
			}
			if allZero {
				delete(changes, addr)
			}
		}
	}

	return changes
}



// ---------------- SwapVolume ----------------
// tokenPrice: tokenAddress -> USD 价格
// 稳定币直接填 1
func SwapVolume(changes map[string]*AssetChange, tokenPrice map[string]float64, bnbPriceUSD float64) float64 {
	maxValue := 0.0

	for _, ac := range changes {
		value := float64(ac.BNB.Int64()) / 1e18 * bnbPriceUSD
		for token, amt := range ac.Tokens {
			price, ok := tokenPrice[strings.ToLower(token)]
			if !ok {
				price = 0.0
			}
			value += float64(amt.Int64()) / 1e18 * price
		}
		if value > maxValue {
			maxValue = value
		}
	}

	return maxValue
}