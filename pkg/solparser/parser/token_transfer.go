package parser

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gagliardetto/solana-go/rpc"
	types2 "github.com/lonelybeanz/tools/pkg/solparser/types"
)

type TokenTransfer struct {
	Info struct {
		Amount      string `json:"amount"`
		Authority   string `json:"authority"`
		Destination string `json:"destination"`
		Source      string `json:"source"`
	} `json:"info"`
	InstructionType string `json:"type"`
}

type TokenTransferChecked struct {
	Info struct {
		Authority   string `json:"authority"`
		Destination string `json:"destination"`
		Mint        string `json:"mint"`
		Source      string `json:"source"`
		TokenAmount struct {
			Amount         string  `json:"amount"`
			Decimals       int     `json:"decimals"`
			UiAmount       float64 `json:"uiAmount"`
			UiAmountString string  `json:"uiAmountString"`
		} `json:"tokenAmount"`
	} `json:"info"`
	InstructionType string `json:"type"`
}

type CloseAccount struct {
	Info struct {
		Account     string `json:"account"`
		Owner       string `json:"owner"`
		Destination string `json:"destination"`
	} `json:"info"`
	Type string `json:"type"`
}

func (s *SolParser) ParseTokenTransferEvent(tx *rpc.ParsedInstruction) (*types2.TransferEvent, error) {
	tokenTransfer, err := s.ParseTokenTransfer(tx)
	if err != nil {
		return nil, err
	}
	if tokenTransfer == nil {
		return nil, nil
	}
	event := &types2.TransferEvent{
		Token: types2.TokenAmt{
			From:   tokenTransfer.Info.Source,
			To:     tokenTransfer.Info.Destination,
			Amount: tokenTransfer.Info.Amount,
		},
	}

	return event, nil
}

func (s *SolParser) ParseTokenTransfer(ix *rpc.ParsedInstruction) (*TokenTransfer, error) {
	byteMsg, err := s.parseInstruction(ix)
	if err != nil {
		return nil, fmt.Errorf("parsing instruction: %w", err)
	}

	msgStr := string(byteMsg)
	if strings.Contains(msgStr, "transferChecked") {
		transfer1 := &TokenTransferChecked{}
		if err := json.Unmarshal(byteMsg, transfer1); err != nil {
			return nil, fmt.Errorf("unmarshaling checked transfer: %w", err)
		}
		return &TokenTransfer{
			Info: struct {
				Amount      string `json:"amount"`
				Authority   string `json:"authority"`
				Destination string `json:"destination"`
				Source      string `json:"source"`
			}{
				Amount:      transfer1.Info.TokenAmount.Amount,
				Authority:   transfer1.Info.Authority,
				Destination: transfer1.Info.Destination,
				Source:      transfer1.Info.Source,
			},
		}, nil
	} else if strings.Contains(msgStr, "transfer") {
		transfer := &TokenTransfer{}
		if err := json.Unmarshal(byteMsg, transfer); err != nil {
			return nil, fmt.Errorf("unmarshaling transfer: %w", err)
		}
		return transfer, nil
	} else if strings.Contains(msgStr, "closeAccount") {
		closeAccount := &CloseAccount{}
		if err := json.Unmarshal(byteMsg, closeAccount); err != nil {
			return nil, fmt.Errorf("unmarshaling close account: %w", err)
		}
		return &TokenTransfer{
			Info: struct {
				Amount      string `json:"amount"`
				Authority   string `json:"authority"`
				Destination string `json:"destination"`
				Source      string `json:"source"`
			}{
				Amount:      "2039280",
				Authority:   closeAccount.Info.Owner,
				Destination: closeAccount.Info.Destination,
				Source:      closeAccount.Info.Account,
			},
			InstructionType: "closeAccount",
		}, nil
	}

	return nil, fmt.Errorf("not a valid transfer instruction %s", ix.ProgramId.String())
}
