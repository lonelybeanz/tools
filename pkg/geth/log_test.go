package geth

import (
	"context"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

func TestParseTxLogs(t *testing.T) {

	client, err := ethclient.Dial("https://bsc-mainnet.core.chainstack.com/8584b635eccbec059338b0095fbe83d2")
	if err != nil {
		t.Log(err)
	}
	// 获取交易回执
	receipt, err := client.TransactionReceipt(context.Background(), common.HexToHash("0xc7083ac5dbfdd8a0f1ee8315ae08b59091e78a304732ac4fe3e3bd8d2d5473a5"))
	if err != nil {
		t.Log(err)
	}

	// 解析交易回执中的日志
	logs := receipt.Logs

	transfer := parseTxLogs(context.Background(), logs)

	t.Logf("transfers: %+v\n", transfer)

	transferTracker := NewTransferTracker("0xc7083ac5dbfdd8a0f1ee8315ae08b59091e78a304732ac4fe3e3bd8d2d5473a5")
	for _, v := range transfer {
		for _, vv := range v {
			transferTracker.AddTransfer(vv.From, vv.To, vv.Address, vv.Amount)
		}
	}

	for _, v := range transferTracker.GetAllTokens() {
		for _, vv := range transferTracker.GetAllAccounts() {
			net := transferTracker.GetNetBalance(vv, v)
			if net.Cmp(big.NewInt(0)) == 0 {
				continue
			}
			t.Logf("address: %s, token: %s, net: %s\n", vv, v, net.String())
		}

	}
}
