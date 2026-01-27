package parser

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/lonelybeanz/tools/pkg/solparser/consts"
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

type InitializeAccount3 struct {
	Info struct {
		Mint    string `json:"mint"`
		Owner   string `json:"owner"`
		Account string `json:"account"`
	} `json:"info"`
	Type string `json:"type"`
}

type SyncNative struct {
	Info struct {
		Account string `json:"account"`
	} `json:"info"`
	Type string `json:"type"`
}

func (s *SolParser) ParseTokenTransferEvent(tx *rpc.ParsedInstruction) (*TransferEvent, error) {
	byteMsg, err := s.parseInstruction(tx)
	if err != nil {
		return nil, fmt.Errorf("parsing instruction: %w", err)
	}

	msgStr := string(byteMsg)

	var instructionType string
	if strings.Contains(msgStr, "transferChecked") {
		instructionType = "transferChecked"
	} else if strings.Contains(msgStr, "transfer") {
		instructionType = "transfer"
	} else if strings.Contains(msgStr, "closeAccount") {
		instructionType = "closeAccount"
	} else if strings.Contains(msgStr, "initializeAccount3") {
		instructionType = "initializeAccount3"
	} else if strings.Contains(msgStr, "syncNative") {
		instructionType = "syncNative"
	} else {
		return nil, fmt.Errorf("not a valid transfer instruction %s", tx.ProgramId.String())
	}

	switch instructionType {
	case "transferChecked":
		transfer1 := &TokenTransferChecked{}
		if err := json.Unmarshal(byteMsg, transfer1); err != nil {
			return nil, fmt.Errorf("unmarshaling checked transfer: %w", err)
		}
		s.updateAccountCache(true, transfer1.Info.Source, "", transfer1.Info.Mint)
		s.updateAccountCache(true, transfer1.Info.Destination, "", transfer1.Info.Mint)
		return &TransferEvent{
			Type:   "tokenTransfer",
			From:   transfer1.Info.Source,
			To:     transfer1.Info.Destination,
			Token:  transfer1.Info.Mint,
			Amount: transfer1.Info.TokenAmount.Amount,
		}, nil

	case "transfer":
		transfer := &TokenTransfer{}
		if err := json.Unmarshal(byteMsg, transfer); err != nil {
			return nil, fmt.Errorf("unmarshaling transfer: %w", err)
		}
		transfer.InstructionType = "tokenTransfer"

		s.updateAccountCache(true, transfer.Info.Source, "", "")
		s.updateAccountCache(true, transfer.Info.Destination, "", "")

		return &TransferEvent{
			Type:   "tokenTransfer",
			From:   transfer.Info.Source,
			To:     transfer.Info.Destination,
			Amount: transfer.Info.Amount,
		}, nil

	case "closeAccount":
		closeAccount := &CloseAccount{}
		if err := json.Unmarshal(byteMsg, closeAccount); err != nil {
			return nil, fmt.Errorf("unmarshaling close account: %w", err)
		}
		s.updateAccountCache(true, closeAccount.Info.Account, closeAccount.Info.Owner, "")

		return &TransferEvent{
			Type:   "closeAccount",
			From:   closeAccount.Info.Account,
			To:     closeAccount.Info.Destination,
			Token:  consts.SOL,
			Amount: "2039280",
		}, nil

	case "initializeAccount3":
		initializeAccount3 := &InitializeAccount3{}
		if err := json.Unmarshal(byteMsg, initializeAccount3); err != nil {
			return nil, fmt.Errorf("unmarshaling initialize account: %w", err)
		}
		s.updateAccountCache(true, initializeAccount3.Info.Account, initializeAccount3.Info.Owner, initializeAccount3.Info.Mint)
		s.updateTokenAccountCache(initializeAccount3.Info.Mint, initializeAccount3.Info.Account)

		return nil, nil
	case "syncNative":
		syncNative := &SyncNative{}
		if err := json.Unmarshal(byteMsg, syncNative); err != nil {
			return nil, fmt.Errorf("unmarshaling sync native: %w", err)
		}
		s.updateAccountCache(true, syncNative.Info.Account, "", solana.WrappedSol.String())
		s.updateTokenAccountCache(solana.WrappedSol.String(), syncNative.Info.Account)

		accountInfo := s.accountCache[syncNative.Info.Account]
		if accountInfo == nil {
			return nil, fmt.Errorf("account info not found for %s", syncNative.Info.Account)
		}
		return &TransferEvent{
			Type:   "syncNative",
			From:   syncNative.Info.Account,
			To:     "",
			Token:  solana.WrappedSol.String(),
			Amount: "",
		}, nil
	default:
		return nil, fmt.Errorf("not a valid transfer instruction %s", tx.ProgramId.String())
	}

}
