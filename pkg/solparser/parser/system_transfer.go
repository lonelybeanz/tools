package parser

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	types2 "github.com/lonelybeanz/tools/pkg/solparser/types"
)

type SystemTransfer struct {
	// programId:11111111111111111111111111111111
	Info struct {
		Destination string `json:"destination"`
		Lamports    int    `json:"lamports"`
		Source      string `json:"source"`
	} `json:"info"`
	Type string `json:"type"`
}

type CreateAccount struct {
	Info struct {
		NewAccount string `json:"newAccount"`
		Source     string `json:"source"`
		Lamports   int    `json:"lamports"`
		Space      int    `json:"space"`
		Owner      string `json:"owner"`
	} `json:"info"`
	Type string `json:"type"`
}

func (s *SolParser) ParseSystemTransferEvent(tx *rpc.ParsedInstruction) (*types2.TransferEvent, error) {
	event := &types2.TransferEvent{}

	return event, nil
}

func (s *SolParser) ParseSystemTransfer(ix *rpc.ParsedInstruction) (*SystemTransfer, error) {
	if ix.ProgramId != solana.SystemProgramID {
		return nil, errors.New("not a system transfer")
	}

	byteMsg, err := s.parseInstruction(ix)
	if err != nil {
		return nil, fmt.Errorf("parsing instruction: %w", err)
	}
	msgStr := string(byteMsg)
	transfer := &SystemTransfer{}
	if strings.Contains(msgStr, "createAccount") {
		createAccount := &CreateAccount{}
		if err := json.Unmarshal(byteMsg, createAccount); err != nil {
			return nil, fmt.Errorf("unmarshaling system transfer: %w", err)
		}
		transfer.Info.Destination = createAccount.Info.NewAccount
		transfer.Info.Source = createAccount.Info.Source
		transfer.Info.Lamports = createAccount.Info.Lamports
		transfer.Type = "createAccount"
	} else if strings.Contains(msgStr, "transfer") {
		if err := json.Unmarshal(byteMsg, transfer); err != nil {
			return nil, fmt.Errorf("unmarshaling system transfer: %w", err)
		}
	} else {
		return nil, errors.New("not a system transfer")
	}

	return transfer, nil
}
