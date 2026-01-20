package solparser

import (
	"context"
	"fmt"
	"testing"

	"github.com/lonelybeanz/tools/pkg/solparser/parser"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

func TestParseSwapEvent(t *testing.T) {
	// Initialize RPC client
	client := rpc.New("https://api.mainnet-beta.solana.com")
	uint64One := uint64(1)

	// Create parser instance
	p := parser.NewSolParser(client, nil)

	// Transaction signature to parse
	sig := solana.MustSignatureFromBase58("gkkuKB6uMXgePdbWkwxpMA6c3as5PzPErwBpNweYjWZsa521TvepmV73foYWbDnVd8jJYpMqPEUseyFEZvBHQYC")

	// Get parsed transaction
	opts := &rpc.GetParsedTransactionOpts{
		MaxSupportedTransactionVersion: &uint64One,
		Commitment:                     rpc.CommitmentConfirmed,
	}

	parsedTx, err := client.GetParsedTransaction(context.Background(), sig, opts)
	if err != nil {
		panic(err)
	}

	// Parse swap events
	events, err := p.ParseTransferEvent(parsedTx)
	if err != nil {
		panic(err)
	}

	// Process swap events
	for _, event := range events {
		fmt.Printf("Swap Event:\n")
		fmt.Printf("  Sender: %s\n", event.Token.From)
		fmt.Printf("  Receiver: %s\n", event.Token.To)
		fmt.Printf("  In Token: %s Amount: %s\n", event.Token.Code, event.Token.Amount)

	}
}
