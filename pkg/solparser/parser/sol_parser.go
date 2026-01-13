package parser

import (
	"context"
	"fmt"
	"time"

	"github.com/lonelybeanz/tools/pkg/solparser/types"

	"github.com/avast/retry-go"
	"github.com/decert-me/solana-go-sdk/common"
	"github.com/decert-me/solana-go-sdk/program/token"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

type TransferInstructions struct {
	transferIx *rpc.ParsedInstruction
}

type SwapInstructions struct {
	swapIx      *rpc.ParsedInstruction
	transferIx1 *rpc.ParsedInstruction
	transferIx2 *rpc.ParsedInstruction
	transferIx3 *rpc.ParsedInstruction //for Lifinity V2
}

type InstructionContext struct {
	instruction *rpc.ParsedInstruction
	index       int
	innerInst   *rpc.ParsedInnerInstruction // nil for outer instructions
	innerIdx    int

	eventIndexIdentifier int
}

type SolParser struct {
	cli *rpc.Client
}

func NewSolParser(cli *rpc.Client) *SolParser {
	return &SolParser{cli: cli}
}

func (s *SolParser) GetParseFuncByProgramId(programId string) (func(*rpc.ParsedInstruction) (*types.TransferEvent, error), bool) {
	parseFuncs := map[string]func(*rpc.ParsedInstruction) (*types.TransferEvent, error){
		solana.SystemProgramID.String():    s.ParseSystemTransferEvent,
		solana.TokenProgramID.String():     s.ParseTokenTransferEvent,
		solana.Token2022ProgramID.String(): s.ParseTokenTransferEvent,
	}
	parseFunc, exists := parseFuncs[programId]
	return parseFunc, exists
}

func (s *SolParser) ParseTransferEvent(parsedTransaction *rpc.GetParsedTransactionResult) ([]*types.TransferEvent, error) {
	if err := validateTransaction(parsedTransaction); err != nil {
		return nil, fmt.Errorf("invalid transaction: %w", err)
	}

	events := []*types.TransferEvent{}

	outerEvents := s.processInstructions(parsedTransaction, s.getOuterInstructions)
	events = append(events, outerEvents...)

	innerEvents := s.processInstructions(parsedTransaction, s.getInnerInstructions)
	events = append(events, innerEvents...)
	return events, nil
}

func (s *SolParser) processInstructions(
	tx *rpc.GetParsedTransactionResult,
	getInstructions func(*rpc.GetParsedTransactionResult) []InstructionContext,
) []*types.TransferEvent {
	var events []*types.TransferEvent
	for _, ctx := range getInstructions(tx) {
		transferInsts, err := s.extractInstructions(tx, ctx)

		if err != nil {
			continue
		}

		if transferInsts == nil {
			continue
		}
		var event *types.TransferEvent

		event, err = s.ParseInstructionIntoTransferEvent(
			tx,
			ctx.eventIndexIdentifier,
			transferInsts.transferIx,
		)

		if err != nil {
			continue
		}
		if event != nil {
			events = append(events, event)
		}
	}
	return events
}

func (s *SolParser) ParseInstructionIntoTransferEvent(parsedTransaction *rpc.GetParsedTransactionResult, idxOuter int, transferIx *rpc.ParsedInstruction) (*types.TransferEvent, error) {

	// feePayer := parsedTransaction.Transaction.Message.AccountKeys[0]
	if transferIx == nil {
		return nil, nil
	}

	parseFunc, exists := s.GetParseFuncByProgramId(transferIx.ProgramId.String())
	if !exists {
		return nil, fmt.Errorf("unsupported swap instruction: %s", transferIx.ProgramId.String())
	}

	event, err := parseFunc(transferIx)
	if err != nil || event == nil {
		return nil, fmt.Errorf("parsing swap event: %w", err)
	}

	// Fill token amounts
	if err := s.fillTokenAmounts(event, transferIx); err != nil {
		return event, err
	}

	// Set base fields
	event.EventIndex = idxOuter

	return event, nil
}

// Get outer instructions context
func (s *SolParser) getOuterInstructions(tx *rpc.GetParsedTransactionResult) []InstructionContext {
	var contexts []InstructionContext
	for idx, inst := range tx.Transaction.Message.Instructions {
		if isTransferInstruction(inst.ProgramId.String()) {
			contexts = append(contexts, InstructionContext{
				instruction:          inst,
				index:                idx,
				eventIndexIdentifier: idx + 1,
			})
		}
	}
	return contexts
}

func (s *SolParser) extractInstructions(
	tx *rpc.GetParsedTransactionResult,
	ctx InstructionContext,
) (*TransferInstructions, error) {
	if !isTransferInstruction(ctx.instruction.ProgramId.String()) {
		return nil, nil
	}

	transInst := &TransferInstructions{
		transferIx: ctx.instruction,
	}

	return transInst, nil

}

// Get inner instructions context
func (s *SolParser) getInnerInstructions(tx *rpc.GetParsedTransactionResult) []InstructionContext {
	var contexts []InstructionContext

	for _, innerInst := range tx.Meta.InnerInstructions {
		for innerIdx, inst := range innerInst.Instructions {
			if isTransferInstruction(inst.ProgramId.String()) {
				finalIdx, err := createUniqueIndex(int(innerInst.Index), innerIdx)
				if err != nil {
					fmt.Printf("error creating unique index: %v", err)
					continue
				}
				contexts = append(contexts, InstructionContext{
					instruction:          inst,
					index:                innerIdx,
					innerInst:            &innerInst,
					innerIdx:             int(innerInst.Index),
					eventIndexIdentifier: finalIdx,
				})
			}

		}
	}
	return contexts
}

// Helper function to fill token amounts
func (s *SolParser) fillTokenAmounts(transEvent *types.TransferEvent, transferIx *rpc.ParsedInstruction) error {
	var err error
	if transEvent.Token, err = s.FillTokenAmtWithTransferIx(transEvent.Token, transferIx); err != nil {
		return fmt.Errorf("filling in token amount: %w", err)
	}

	return nil
}

func (s *SolParser) FillTokenAmtWithTransferIx(tkAmt types.TokenAmt, ix *rpc.ParsedInstruction) (types.TokenAmt, error) {
	transfer, err := s.ParseTransfer(ix)
	if err != nil {
		return tkAmt, err
	}
	tkAmt.Amount = transfer.Info.Amount

	var mintAddress string // token mint address
	var tokenInfo *token.TokenAccount
	if tokenInfo, err = s.RetryGetTokenAccountInfoByTokenAccount(transfer.Info.Destination); err == nil && tokenInfo != nil {
		mintAddress = tokenInfo.Mint.String()
		tkAmt.To = tokenInfo.Owner.String()
	} else if tokenInfo, err = s.RetryGetTokenAccountInfoByTokenAccount(transfer.Info.Source); err == nil && tokenInfo != nil {
		mintAddress = tokenInfo.Mint.String()
		tkAmt.From = tokenInfo.Owner.String()
	} else {
		return tkAmt, err
	}
	tkAmt.Code = mintAddress
	return tkAmt, nil
}

func (s *SolParser) RetryGetTokenAccountInfoByTokenAccount(tokenAccount string) (*token.TokenAccount, error) {
	var tokenInfo *token.TokenAccount
	var err error
	err = retry.Do(func() error {
		tokenInfo, err = s.GetTokenAccountInfoByTokenAccount(tokenAccount)
		if err == nil {
			return nil
		}
		return err
	}, retry.Attempts(3), retry.Delay(1*time.Second), retry.LastErrorOnly(true), retry.DelayType(func(n uint, err error, config *retry.Config) time.Duration {
		return retry.BackOffDelay(n, err, config)
	}))
	return tokenInfo, err

}

func (s *SolParser) GetTokenAccountInfoByTokenAccount(tokenAccount string) (*token.TokenAccount, error) {
	ctx := context.Background()
	accountInfo, e := s.cli.GetAccountInfo(ctx, solana.MustPublicKeyFromBase58(tokenAccount))
	if e != nil {
		return nil, e
	}
	if accountInfo.Value != nil && accountInfo.Value.Owner == solana.SystemProgramID {
		uint64One := uint64(1)
		// 账户本身就是一个 lamport 账户，非Token账户
		t := &token.TokenAccount{
			Mint:            common.PublicKey(solana.SolMint),
			Owner:           common.PublicKeyFromString(tokenAccount),
			Amount:          accountInfo.Value.Lamports,
			Delegate:        nil,
			State:           1, //tokenAccount.Initialized
			IsNative:        &uint64One,
			DelegatedAmount: 0,
			CloseAuthority:  nil,
		}
		return t, nil
	}
	t, err2 := token.TokenAccountFromData(accountInfo.GetBinary())
	if err2 != nil {
		return nil, fmt.Errorf("error decoding token account data: %v", err2)
	}
	return &t, nil
}
