package parser

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

func validateTransaction(tx *rpc.GetParsedTransactionResult) error {
	if tx == nil || tx.Transaction == nil {
		return errors.New("parsedTransaction is nil")
	}
	if len(tx.Transaction.Message.AccountKeys) == 0 {
		return errors.New("no instructions found")
	}
	if len(tx.Transaction.Signatures) == 0 {
		return errors.New("no signatures found")
	}
	return nil
}

func createUniqueIndex(outerIdx, innerIdx int) (int, error) {
	prefIdx := fmt.Sprintf("%d%d", outerIdx+1, innerIdx+1)
	return strconv.Atoi(prefIdx)
}

func isTransferInstruction(programID string) bool {
	switch programID {
	case solana.TokenProgramID.String(),
		solana.Token2022ProgramID.String(),
		solana.SystemProgramID.String():
		return true
	default:
		return false
	}
}

func IsTokenProgramId(program solana.PublicKey) bool {
	return program == solana.TokenProgramID || program == solana.Token2022ProgramID
}

func IsSystemProgrmId(program solana.PublicKey) bool {
	return program == solana.SystemProgramID
}
