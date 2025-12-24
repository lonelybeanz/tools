package geth

import (
	"math/big"

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
