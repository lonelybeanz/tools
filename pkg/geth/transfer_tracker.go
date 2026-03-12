package geth

import (
	"fmt"
	"math"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/lonelybeanz/tools/pkg/log"
)

// TransactionTracker 用于跟踪账户间的代币转账
type TransferTracker struct {
	// 存储所有转账记录
	transfers []*TransferRecord
	TxHash    string
}

// TransferRecord 表示一次转账记录
type TransferRecord struct {
	From   common.Address // 转出账户
	To     common.Address // 转入账户
	Token  common.Address // 代币地址
	Amount *big.Int       // 转账数量
}

// NewTransactionTracker 创建一个新的TransactionTracker实例
func NewTransferTracker(txHash string) *TransferTracker {
	return &TransferTracker{
		TxHash:    txHash,
		transfers: make([]*TransferRecord, 0),
	}
}

// AddTransfer 添加一笔转账记录
func (tt *TransferTracker) AddTransfer(from, to, token common.Address, amount *big.Int) {
	record := &TransferRecord{
		From:   from,
		To:     to,
		Token:  token,
		Amount: new(big.Int).Set(amount), // 创建amount的副本以避免外部修改
	}

	tt.transfers = append(tt.transfers, record)

	log.Debugf("{%s} Added transfer:[%s] %s -> %s (%s)", tt.TxHash, token.String(), from.String(), to.String(), amount.String())
}

// GetTransfers returns all transfer records.
func (tt *TransferTracker) GetTransfers() []*TransferRecord {
	return tt.transfers
}

// GetIncoming 计算账户收到的特定代币总量
func (tt *TransferTracker) GetIncoming(account, token common.Address) *big.Int {
	total := new(big.Int)

	for _, tx := range tt.transfers {
		// 如果是转入账户且代币匹配
		if tx.To == account && tx.Token == token {
			total.Add(total, tx.Amount)
		}
	}

	return total
}

// GetOutgoing 计算账户转出的特定代币总量
func (tt *TransferTracker) GetOutgoing(account, token common.Address) *big.Int {
	total := new(big.Int)

	for _, tx := range tt.transfers {
		// 如果是转出账户且代币匹配
		if tx.From == account && tx.Token == token {
			total.Add(total, tx.Amount)
		}
	}

	return total
}

func (tt *TransferTracker) GetAllTokens() []common.Address {
	tokens := make([]common.Address, 0)

	for _, tx := range tt.transfers {
		if !contains(tokens, tx.Token) {
			tokens = append(tokens, tx.Token)
		}
	}

	return tokens
}

func (tt *TransferTracker) GetAllFrom() []common.Address {
	addresses := make([]common.Address, 0)
	seen := make(map[common.Address]bool)

	for _, tx := range tt.transfers {
		if !seen[tx.From] {
			seen[tx.From] = true
			addresses = append(addresses, tx.From)
		}
	}

	return addresses
}

func (tt *TransferTracker) GetAllTo() []common.Address {
	addresses := make([]common.Address, 0)
	seen := make(map[common.Address]bool)

	for _, tx := range tt.transfers {
		if !seen[tx.To] {
			seen[tx.To] = true
			addresses = append(addresses, tx.To)
		}
	}

	return addresses
}

func (tt *TransferTracker) GetAllAccounts() []common.Address {
	accounts := make([]common.Address, 0)

	for _, tx := range tt.transfers {
		if !contains(accounts, tx.From) {
			accounts = append(accounts, tx.From)
		}
		if !contains(accounts, tx.To) {
			accounts = append(accounts, tx.To)
		}
	}

	return accounts
}

func (tt *TransferTracker) GetTokenTransactions(token common.Address) []*TransferRecord {
	transactions := make([]*TransferRecord, 0)

	for _, tx := range tt.transfers {
		if tx.Token == token {
			transactions = append(transactions, tx)
		}
	}

	return transactions
}

// GetTransferCounts returns the number of incoming and outgoing transfers for a specific account and token.
func (tt *TransferTracker) GetTransferCounts(account, token common.Address) (inCount, outCount int) {
	for _, tx := range tt.transfers {
		if tx.Token == token {
			if tx.To == account {
				inCount++
			}
			if tx.From == account {
				outCount++
			}
		}
	}
	return
}

// TraceUltimateSource traces a transfer backward to find the original sender in a chain.
// It skips over "intermediate" addresses, which are defined as addresses that, for a given token,
// have a net balance change of zero and exactly one incoming and one outgoing transfer.
// The trace stops at forks (multiple inputs/outputs), merges, or designated non-intermediate addresses.
//
// `address`: The address to start tracing back from.
// `token`: The token being transferred.
// `nonIntermediate`: A set of addresses (e.g., your own wallets) that are considered final sources/sinks and will stop the trace.
func (tt *TransferTracker) TraceUltimateSource(address, token common.Address, nonIntermediate map[common.Address]bool) common.Address {
	current := address
	visited := make(map[common.Address]bool) // To prevent infinite loops in case of cycles

	for {
		if visited[current] {
			return current // Cycle detected, stop here.
		}
		visited[current] = true

		// If the current address is a designated non-intermediate address (e.g., one of our own), it's the source.
		if nonIntermediate[current] {
			return current
		}

		netBalance := tt.GetNetBalance(current, token)
		inCount, outCount := tt.GetTransferCounts(current, token)

		// An address is intermediate if it's a simple 1-in-1-out pass-through for the token.
		if netBalance.Sign() == 0 && inCount == 1 && outCount == 1 {
			// It's an intermediate node. Find the single incoming transfer to trace back.
			var sourceFound bool
			for _, tx := range tt.transfers {
				if tx.To == current && tx.Token == token {
					current = tx.From // Move to the previous address in the chain.
					sourceFound = true
					break
				}
			}
			if !sourceFound {
				// Should not happen if inCount is 1, but as a safeguard.
				return current
			}
		} else {
			// Not an intermediate node, so it's the source in this context.
			return current
		}
	}
}

// TraceUltimateSink traces a transfer forward to find the final recipient in a chain.
// It skips over "intermediate" addresses using the same logic as TraceUltimateSource.
func (tt *TransferTracker) TraceUltimateSink(address, token common.Address, nonIntermediate map[common.Address]bool) common.Address {
	current := address
	visited := make(map[common.Address]bool) // To prevent infinite loops

	for {
		if visited[current] {
			return current // Cycle detected, stop here.
		}
		visited[current] = true

		if nonIntermediate[current] {
			return current
		}

		netBalance := tt.GetNetBalance(current, token)
		inCount, outCount := tt.GetTransferCounts(current, token)

		if netBalance.Sign() == 0 && inCount == 1 && outCount == 1 {
			var sinkFound bool
			for _, tx := range tt.transfers {
				if tx.From == current && tx.Token == token {
					current = tx.To // Move to the next address in the chain.
					sinkFound = true
					break
				}
			}
			if !sinkFound {
				return current
			}
		} else {
			return current
		}
	}
}

// GetNetBalance 计算账户特定代币的净余额（收到 - 转出）
func (tt *TransferTracker) GetNetBalance(account, token common.Address) *big.Int {
	incoming := tt.GetIncoming(account, token)
	outgoing := tt.GetOutgoing(account, token)

	// 计算净余额 = 收到 - 转出
	net := new(big.Int).Sub(incoming, outgoing)
	return net
}

// GetTransactionsByAccount 获取与特定账户相关的所有交易
func (tt *TransferTracker) GetTransactionsByAccount(account common.Address) []*TransferRecord {
	records := make([]*TransferRecord, 0)

	for _, tx := range tt.transfers {
		// 如果是转出账户或转入账户
		if tx.From == account || tx.To == account {
			records = append(records, tx)
		}
	}

	return records
}

// GetTransactionsByToken 获取特定代币的所有交易
func (tt *TransferTracker) GetTransactionsByToken(token common.Address) []*TransferRecord {
	records := make([]*TransferRecord, 0)

	for _, tx := range tt.transfers {
		// 如果是该代币的交易
		if tx.Token == token {
			records = append(records, tx)
		}
	}

	return records
}

func contains[T comparable](slice []T, value T) bool {
	for _, item := range slice {
		if item == value {
			return true
		}
	}
	return false
}

// ToDOT generates a string representation of the transfer graph in DOT format.
// This can be used with tools like Graphviz to visualize the flow.
// tokenDetails is a map from token address to its details (symbol, decimals).
func (tt *TransferTracker) ToDOT(tokenDetails map[common.Address]*TokenPrice) string {
	var sb strings.Builder
	sb.WriteString("digraph transfers {\n")
	sb.WriteString("  rankdir=LR;\n")
	sb.WriteString(`  node [shape=box, style="rounded,filled", fillcolor=lightblue];` + "\n")
	sb.WriteString("  edge [fontsize=10];\n\n")

	// Aggregate transfers between the same two accounts for the same token
	type transferKey struct {
		from, to, token common.Address
	}
	aggregated := make(map[transferKey]*big.Int)

	for _, tx := range tt.transfers {
		key := transferKey{from: tx.From, to: tx.To, token: tx.Token}
		if _, ok := aggregated[key]; !ok {
			aggregated[key] = new(big.Int)
		}
		aggregated[key].Add(aggregated[key], tx.Amount)
	}

	for key, amount := range aggregated {
		from, to, tokenAddr := key.from, key.to, key.token

		details, ok := tokenDetails[tokenAddr]
		var label string
		if ok && details.Decimal > 0 {
			// Format amount with decimals
			fAmount := new(big.Float).SetInt(amount)
			powerOf10 := new(big.Float).SetFloat64(math.Pow10(details.Decimal))
			fAmount.Quo(fAmount, powerOf10)

			label = fmt.Sprintf(`"%s %s"`, fAmount.Text('f', 6), details.Symbol)
		} else {
			// Fallback to amount in wei and token address
			label = fmt.Sprintf(`"%s\n%s"`, amount.String(), tokenAddr.Hex())
		}

		sb.WriteString(fmt.Sprintf(`  "%s" -> "%s" [label=%s];`+"\n", from.Hex(), to.Hex(), label))
	}

	sb.WriteString("}\n")
	return sb.String()
}
