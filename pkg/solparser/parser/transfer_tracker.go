package parser

import (
	"fmt"
	"math"
	"math/big"
	"strings"

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
	From   string   // 转出账户
	To     string   // 转入账户
	Token  string   // 代币地址
	Amount *big.Int // 转账数量
}

// NewTransactionTracker 创建一个新的TransactionTracker实例
func NewTransferTracker(txHash string) *TransferTracker {
	return &TransferTracker{
		TxHash:    txHash,
		transfers: make([]*TransferRecord, 0),
	}
}

// AddTransfer 添加一笔转账记录
func (tt *TransferTracker) AddTransfer(from, to, token string, amount *big.Int) {
	record := &TransferRecord{
		From:   from,
		To:     to,
		Token:  token,
		Amount: new(big.Int).Set(amount), // 创建amount的副本以避免外部修改
	}

	tt.transfers = append(tt.transfers, record)

	log.Debugf("{%s} Added transfer:[%s] %s -> %s (%s)", tt.TxHash, token, from, to, amount.String())
}

// GetIncoming 计算账户收到的特定代币总量
func (tt *TransferTracker) GetIncoming(account, token string) *big.Int {
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
func (tt *TransferTracker) GetOutgoing(account, token string) *big.Int {
	total := new(big.Int)

	for _, tx := range tt.transfers {
		// 如果是转出账户且代币匹配
		if tx.From == account && tx.Token == token {
			total.Add(total, tx.Amount)
		}
	}

	return total
}

func (tt *TransferTracker) GetAllTokens() []string {
	tokens := make([]string, 0)

	for _, tx := range tt.transfers {
		if !contains(tokens, tx.Token) {
			tokens = append(tokens, tx.Token)
		}
	}

	return tokens
}

func (tt *TransferTracker) GetAllFrom() []string {
	addresses := make([]string, 0)
	seen := make(map[string]bool)

	for _, tx := range tt.transfers {
		if !seen[tx.From] {
			seen[tx.From] = true
			addresses = append(addresses, tx.From)
		}
	}

	return addresses
}

func (tt *TransferTracker) GetAllTo() []string {
	addresses := make([]string, 0)
	seen := make(map[string]bool)

	for _, tx := range tt.transfers {
		if !seen[tx.To] {
			seen[tx.To] = true
			addresses = append(addresses, tx.To)
		}
	}

	return addresses
}

func (tt *TransferTracker) GetAllAccounts() []string {
	accounts := make([]string, 0)

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

func (tt *TransferTracker) GetTokenTransactions(token string) []*TransferRecord {
	transactions := make([]*TransferRecord, 0)

	for _, tx := range tt.transfers {
		if tx.Token == token {
			transactions = append(transactions, tx)
		}
	}

	return transactions
}

// GetNetBalance 计算账户特定代币的净余额（收到 - 转出）
func (tt *TransferTracker) GetNetBalance(account, token string) *big.Int {
	incoming := tt.GetIncoming(account, token)
	outgoing := tt.GetOutgoing(account, token)

	// 计算净余额 = 收到 - 转出
	net := new(big.Int).Sub(incoming, outgoing)
	return net
}

// GetTransactionsByAccount 获取与特定账户相关的所有交易
func (tt *TransferTracker) GetTransactionsByAccount(account string) []*TransferRecord {
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
func (tt *TransferTracker) GetTransactionsByToken(token string) []*TransferRecord {
	records := make([]*TransferRecord, 0)

	for _, tx := range tt.transfers {
		// 如果是该代币的交易
		if tx.Token == token {
			records = append(records, tx)
		}
	}

	return records
}

// getIncomingTransfers is a helper to get all incoming transfers for an account and token.
func (tt *TransferTracker) getIncomingTransfers(account, token string) []*TransferRecord {
	records := make([]*TransferRecord, 0)
	for _, tx := range tt.transfers {
		if tx.To == account && tx.Token == token {
			records = append(records, tx)
		}
	}
	return records
}

// getOutgoingTransfers is a helper to get all outgoing transfers for an account and token.
func (tt *TransferTracker) getOutgoingTransfers(account, token string) []*TransferRecord {
	records := make([]*TransferRecord, 0)
	for _, tx := range tt.transfers {
		if tx.From == account && tx.Token == token {
			records = append(records, tx)
		}
	}
	return records
}

// FindOriginalSources 追踪一个账户收到特定代币的来源。
// 它优先考虑“经济来源”。如果一个账户转出一种代币并收到另一种代币（类似Swap），
// 那么该账户本身被视为其收到代币的来源。
// 如果不是类似Swap的场景（例如简单的转账），它将回退到追踪代币的物理路径，
// 找到在此交易中最初发送该代币的账户。
func (tt *TransferTracker) FindOriginalSources(account, token string) []string {
	// --- 经济来源启发式判断 ---
	// 检查账户是否收到了该代币。
	hasReceivedToken := false
	for _, tx := range tt.transfers {
		if tx.To == account && tx.Token == token {
			hasReceivedToken = true
			break
		}
	}
	if !hasReceivedToken {
		return nil // 没有收到此代币，无需寻找来源。
	}

	// 检查账户是否转出了任何 *其他* 代币。如果是，这很可能是一次Swap，
	// 账户本身就是经济来源。
	hasSentOtherToken := false
	for _, tx := range tt.transfers {
		if tx.From == account && tx.Token != token {
			hasSentOtherToken = true
			break
		}
	}

	if hasSentOtherToken {
		// 类似Swap的活动，账户本身是来源。
		return []string{account}
	}

	// --- 物理来源追踪 (回退) ---
	// 如果不是Swap（例如，简单的收款），则追踪物理来源。
	memo := make(map[string][]string)
	path := make(map[string]bool)
	return tt.findSourcesRecursive(account, token, memo, path)
}

func (tt *TransferTracker) findSourcesRecursive(account, token string, memo map[string][]string, path map[string]bool) []string {
	if sources, ok := memo[account]; ok {
		return sources
	}
	if path[account] { // Cycle detected
		return nil
	}
	path[account] = true
	defer func() { path[account] = false }() // backtrack

	incomingTransfers := tt.getIncomingTransfers(account, token)

	if len(incomingTransfers) == 0 {
		// This account has no incoming transfers for this token, so it's an original source.
		return []string{account}
	}

	var finalSources []string
	sourceMap := make(map[string]bool)

	for _, tx := range incomingTransfers {
		sources := tt.findSourcesRecursive(tx.From, token, memo, path)
		for _, source := range sources {
			if !sourceMap[source] {
				sourceMap[source] = true
				finalSources = append(finalSources, source)
			}
		}
	}

	memo[account] = finalSources
	return finalSources
}

// FindFinalDestinations traces forward to find accounts that were the ultimate recipients of a token from a given account.
// A final destination is an account that received tokens (directly or indirectly) but did not send any for this token within the transaction.
func (tt *TransferTracker) FindFinalDestinations(account, token string) []string {
	memo := make(map[string][]string)
	path := make(map[string]bool)
	return tt.findDestsRecursive(account, token, memo, path)
}

func (tt *TransferTracker) findDestsRecursive(account, token string, memo map[string][]string, path map[string]bool) []string {
	if dests, ok := memo[account]; ok {
		return dests
	}
	if path[account] { // Cycle detected
		return nil
	}
	path[account] = true
	defer func() { path[account] = false }() // backtrack

	outgoingTransfers := tt.getOutgoingTransfers(account, token)

	if len(outgoingTransfers) == 0 {
		return []string{account}
	}

	var finalDests []string
	destMap := make(map[string]bool)

	for _, tx := range outgoingTransfers {
		dests := tt.findDestsRecursive(tx.To, token, memo, path)
		for _, dest := range dests {
			if !destMap[dest] {
				destMap[dest] = true
				finalDests = append(finalDests, dest)
			}
		}
	}

	memo[account] = finalDests
	return finalDests
}

// TokenInfo is a simplified struct for token details needed for the diagram.
type TokenInfo struct {
	Symbol  string
	Decimal int
}

// ToDOT generates a string representation of the transfer graph in DOT format.
func (tt *TransferTracker) ToDOT(tokenDetails map[string]*TokenInfo) string {
	var sb strings.Builder
	sb.WriteString("digraph transfers {\n")
	sb.WriteString("  rankdir=LR;\n")
	sb.WriteString(`  node [shape=box, style="rounded,filled", fillcolor=lightblue];` + "\n")
	sb.WriteString("  edge [fontsize=10];\n\n")

	type transferKey struct {
		from, to, token string
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
			fAmount := new(big.Float).SetInt(amount)
			powerOf10 := new(big.Float).SetFloat64(math.Pow10(details.Decimal))
			fAmount.Quo(fAmount, powerOf10)
			label = fmt.Sprintf(`"%s %s"`, fAmount.Text('f', 6), details.Symbol)
		} else {
			label = fmt.Sprintf(`"%s\n%s"`, amount.String(), tokenAddr)
		}

		sb.WriteString(fmt.Sprintf(`  "%s" -> "%s" [label=%s];`+"\n", from, to, label))
	}

	sb.WriteString("}\n")
	return sb.String()
}

func contains[T comparable](slice []T, value T) bool {
	for _, item := range slice {
		if item == value {
			return true
		}
	}
	return false
}
