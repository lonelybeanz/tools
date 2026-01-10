package parser

import (
	"context"
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
	testParser = &SolParser{cli: testRpc}
}

func TestSolParser_ParseTransferEvent(t *testing.T) {
	Before(t)
	intOne := uint64(1)
	intPtr := &intOne
	ctx := context.Background()
	opts := &rpc.GetParsedTransactionOpts{MaxSupportedTransactionVersion: intPtr,
		Commitment: rpc.CommitmentConfirmed}
	sig := solana.MustSignatureFromBase58("531Di8ronp8z6aCjAGbw3vFcfJsja222jj5zNHtauH4eymXuN1hQLNaAQMaewf4GmL9f3JznEz3PaEA7G4TD7EzX")
	p, err := testRpc.GetParsedTransaction(ctx, sig, opts)
	if err != nil {
		t.Error(err)
	}
	z, err := testParser.ParseTransferEvent(p)
	if err != nil {
		t.Error(err)
	}
	for i, d := range z {
		t.Logf("Swap Event %d %d:\n", i, d.EventIndex)
		t.Logf("  Sender: %s\n", d.Token.From)
		t.Logf("  Receiver: %s\n", d.Token.To)
		t.Logf("  In Token: %s Amount: %s\n", d.Token.Code, d.Token.Amount)
	}

}
