package parser

import (
	"context"
	"fmt"
	"math/big"
	"testing"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

var (
	testParser *SolParser
	testRpc    *rpc.Client
)

func Before(t *testing.T) {
	testRpc = rpc.New("https://api.mainnet-beta.solana.com")
	// proxyURL, _ := url.Parse("http://127.0.0.1:7890")
	// httpClient := &http.Client{
	// 	Transport: &http.Transport{
	// 		Proxy: http.ProxyURL(proxyURL),
	// 	},
	// 	Timeout: 15 * time.Second,
	// }
	// cluster := rpc.MainNetBeta
	// testRpc = rpc.NewWithCustomRPCClient(jsonrpc.NewClientWithOpts(
	// 	cluster.RPC,
	// 	&jsonrpc.RPCClientOpts{
	// 		HTTPClient:    httpClient,
	// 		CustomHeaders: map[string]string{},
	// 	},
	// ))
	testParser = NewSolParser(testRpc)
}

func TestSolParser_ParseTransferEvent(t *testing.T) {
	Before(t)
	intOne := uint64(1)
	intPtr := &intOne
	ctx := context.Background()
	opts := &rpc.GetParsedTransactionOpts{MaxSupportedTransactionVersion: intPtr,
		Commitment: rpc.CommitmentConfirmed}
	sig := solana.MustSignatureFromBase58("RbhumT4HTnMG2kLR5vbx9LvVYxtoqZAY3fRxxL8cyEoPjt3vWdkuSwfCEDUnTm4BtuiWMwdJbZ8oEWkq8pBcX3i")
	p, err := testRpc.GetParsedTransaction(ctx, sig, opts)
	if err != nil {
		t.Error(err)
	}
	z, err := testParser.ParseTransferEvent(p)
	if err != nil {
		t.Error(err)
	}
	tt := NewTransferTracker("")
	for i, d := range z {
		t.Logf("Transfer Event %d %d:\n", i, d.EventIndex)
		t.Logf("  From:%s\n", d.Token.From)
		t.Logf("  To:%s\n", d.Token.To)
		t.Logf("  In Token: %s Amount: %s\n", d.Token.Code, d.Token.Amount)
		amount, _ := new(big.Int).SetString(d.Token.Amount, 10)
		tt.AddTransfer(d.Token.From, d.Token.To, d.Token.Code, amount)
	}
	for _, account := range tt.GetAllAccounts() {
		for _, token := range tt.GetAllTokens() {
			t.Logf("Account: %s Token: %s\n", account, token)
			t.Logf("  Net Balance: %s\n", tt.GetNetBalance(account, token))

		}
	}

	// --- Generate and print flow diagram ---
	tokenDetails := map[string]*TokenInfo{
		"So11111111111111111111111111111111111111111": {Symbol: "SOL", Decimal: 9},
		"So11111111111111111111111111111111111111112": {Symbol: "WSOL", Decimal: 9},
		// Add other known tokens like USDC, USDT for Solana
		"EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v": {Symbol: "USDC", Decimal: 6},
	}

	dotGraph := tt.ToDOT(tokenDetails)
	fmt.Println("\n--- Transfer Graph (DOT format) ---")
	fmt.Println(dotGraph)
	fmt.Println("--- End of Graph ---")
	fmt.Println("\n提示: 复制以上DOT格式的文本并粘贴到Graphviz在线渲染工具中（如: https://dreampuf.github.io/GraphvizOnline/）即可查看可视化流转图。")

}
