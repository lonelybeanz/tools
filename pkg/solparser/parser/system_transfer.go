package parser

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/lonelybeanz/tools/pkg/solparser/consts"
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

func (s *SolParser) ParseSystemTransferEvent(tx *rpc.ParsedInstruction) (*TransferEvent, error) {

	if tx.ProgramId != solana.SystemProgramID {
		return nil, errors.New("not a system transfer")
	}

	byteMsg, err := s.parseInstruction(tx)
	if err != nil {
		return nil, fmt.Errorf("parsing instruction: %w", err)
	}
	msgStr := string(byteMsg)

	if strings.Contains(msgStr, "createAccount") {
		createAccount := &CreateAccount{}
		if err := json.Unmarshal(byteMsg, createAccount); err != nil {
			return nil, fmt.Errorf("unmarshaling system transfer: %w", err)
		}

		s.updateAccountCache(true, createAccount.Info.NewAccount, createAccount.Info.Owner, "")

		return &TransferEvent{
			Type:   "createAccount",
			From:   createAccount.Info.Source,
			To:     createAccount.Info.NewAccount,
			Token:  consts.SOL,
			Amount: fmt.Sprintf("%d", createAccount.Info.Lamports),
		}, nil

	} else if strings.Contains(msgStr, "transfer") {
		transfer := &SystemTransfer{}
		if err := json.Unmarshal(byteMsg, transfer); err != nil {
			return nil, fmt.Errorf("unmarshaling system transfer: %w", err)
		}

		s.updateAccountCache(false, transfer.Info.Source, transfer.Info.Source, "")
		return &TransferEvent{
			Type:   "systemTransfer",
			From:   transfer.Info.Source,
			To:     transfer.Info.Destination,
			Token:  consts.SOL,
			Amount: fmt.Sprintf("%d", transfer.Info.Lamports),
		}, nil

	} else {
		return nil, errors.New("not a system transfer")
	}

}
