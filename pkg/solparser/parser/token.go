package parser

import (
	"errors"
	"fmt"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

func (s *SolParser) parseInstruction(ix *rpc.ParsedInstruction) ([]byte, error) {
	if ix == nil {
		return nil, errors.New("parsed instruction is nil")
	}

	if ix.Parsed == nil && len(ix.Data) == 0 {
		return nil, errors.New("instruction has no parseable data")
	}

	if ix.Data == nil {
		return ix.Parsed.MarshalJSON()
	}
	return ix.Data, nil
}

func (s *SolParser) ParseTransfer(ix *rpc.ParsedInstruction) (*TokenTransfer, error) {
	switch ix.ProgramId {
	case solana.TokenProgramID, solana.Token2022ProgramID:
		return s.ParseTokenTransfer(ix)
	case solana.SystemProgramID:
		// 将 Solana System Program 的 transfer struct 转换为 Token Transfer
		solTransfer, err := s.ParseSystemTransfer(ix)
		if err != nil {
			return nil, err
		}
		tokenTransfer := &TokenTransfer{
			Info: struct {
				Amount      string `json:"amount"`
				Authority   string `json:"authority"`
				Destination string `json:"destination"`
				Source      string `json:"source"`
			}{
				Amount:      fmt.Sprintf("%d", solTransfer.Info.Lamports),
				Authority:   solTransfer.Info.Source,
				Destination: solTransfer.Info.Destination,
				Source:      solTransfer.Info.Source,
			},
			InstructionType: "transfer",
		}
		return tokenTransfer, nil
	default:
		return nil, fmt.Errorf("not a valid transfer instruction %s", ix.ProgramId.String())
	}
}
