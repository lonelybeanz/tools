package geth

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

func TestTt(t *testing.T) {
	tt := NewTransferTracker("")
	token := common.HexToAddress("0x55d398326f99059fF775485246999027B3197955")
	from, to := common.HexToAddress("0xDb18729070d3aBdC72F9cd57D3b949540Cc4486a"), common.HexToAddress("0x9999b0CdD35d7F3B281BA02EfC0d228486940515")
	tt.AddTransfer(from, to, token, big.NewInt(1))

	t.Log(tt.GetNetBalance(from, token))

}
