package parser

import (
	"github.com/gagliardetto/solana-go/rpc"
)

func getBalance(address string, txResp *rpc.GetParsedTransactionResult) (uint64, uint64) {
	message := txResp.Transaction.Message
	txMeta := txResp.Meta
	allAccountKeys := message.AccountKeys

	for i, account := range allAccountKeys {
		if account.PublicKey.String() == address {
			return txMeta.PreBalances[i], txMeta.PostBalances[i]

		}
	}

	return 0, 0
}
