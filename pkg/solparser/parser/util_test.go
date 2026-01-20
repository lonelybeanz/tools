package parser

import (
	"testing"

	"github.com/gagliardetto/solana-go"
)

func Test_IsATA(t *testing.T) {
	t.Run("ATA", func(t *testing.T) {
		ownerAddress := solana.MustPublicKeyFromBase58("3L9UZWLAprLtB2xddEHsCmgXbPc2PidgSjtHGZd2MzB3")
		mintAddress := solana.MustPublicKeyFromBase58("So11111111111111111111111111111111111111112")
		tokenAccountAddress := solana.MustPublicKeyFromBase58("6YxGz7dpWinrNe1n2S8Sq4RcRBjgzoZXLgHo2JMYgtnN")
		result := IsATA(ownerAddress, mintAddress,tokenAccountAddress)
		if !result {
			t.Errorf("Expected true, got false")
		}
	})
}
