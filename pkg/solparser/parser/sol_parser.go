package parser

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/lonelybeanz/tools/pkg/solparser/consts"

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
	instruction          *rpc.ParsedInstruction
	index                int
	innerInst            *rpc.ParsedInnerInstruction // nil for outer instructions
	innerIdx             int
	eventIndexIdentifier int
}

type SolParser struct {
	cli               *rpc.Client
	accountCache      map[string]*AccountInfo
	tokenAccountCache map[string][]string
}

var defaultCache = make(map[string]*AccountInfo)

func NewSolParser(cli *rpc.Client, cache map[string]*AccountInfo) *SolParser {
	if cache == nil {
		cache = make(map[string]*AccountInfo)
	}
	if len(cache) != 0 {
		defaultCache = cache
	}
	return &SolParser{
		cli:               cli,
		accountCache:      cache,
		tokenAccountCache: make(map[string][]string),
	}
}

func (s *SolParser) GetParseFuncByProgramId(programId string) (func(*rpc.ParsedInstruction) (*TransferEvent, error), bool) {
	parseFuncs := map[string]func(*rpc.ParsedInstruction) (*TransferEvent, error){
		solana.SystemProgramID.String():    s.ParseSystemTransferEvent,
		solana.TokenProgramID.String():     s.ParseTokenTransferEvent,
		solana.Token2022ProgramID.String(): s.ParseTokenTransferEvent,
	}
	parseFunc, exists := parseFuncs[programId]
	return parseFunc, exists
}

func (s *SolParser) ParseTransfer(parsedTransaction *rpc.GetParsedTransactionResult) ([]*Transfer, error) {
	if err := validateTransaction(parsedTransaction); err != nil {
		return nil, fmt.Errorf("invalid transaction: %w", err)
	}

	events := []*TransferEvent{}

	outerEvents := s.processInstructions(parsedTransaction, s.getOuterInstructions)
	events = append(events, outerEvents...)

	innerEvents := s.processInstructions(parsedTransaction, s.getInnerInstructions)
	events = append(events, innerEvents...)

	wrapAmount := ""
	for _, event := range events {
		// Fill token amounts
		if err := s.fillTokenAmounts(event); err != nil {
			return nil, fmt.Errorf("filling in token amount: %w", err)
		}

		if to, ok := s.accountCache[event.To]; ok && IsATA(to.Owner, solana.WrappedSol.String(), to.Address) {
			wrapAmount = event.Amount
		}

		// 检查这是否是一次“包装”操作：一个所有者将SOL转入其自身的WSOL ATA
		if from, ok := s.accountCache[event.From]; ok && event.Type == "syncNative" {
			event.To = from.Owner
			event.Amount = wrapAmount
		}
	}

	transfers := []*Transfer{}
	tt := NewTransferTracker("")
	for _, event := range events {
		var transferFrom, transferTo string
		if from, ok := s.accountCache[event.From]; ok && from.IsATA && event.Type == "tokenTransfer" {
			transferFrom = from.Owner
		} else {
			transferFrom = event.From
		}
		if to, ok := s.accountCache[event.To]; ok && to.IsATA && event.Type == "tokenTransfer" {
			transferTo = to.Owner
		} else {
			transferTo = event.To
		}

		if event.Type != "closeAccount" {
			transfers = append(transfers, &Transfer{
				Amount: event.Amount,
				From:   transferFrom,
				To:     transferTo,
				Token:  event.Token,
			})

			amount, _ := new(big.Int).SetString(event.Amount, 10)
			tt.AddTransfer(transferFrom, transferTo, event.Token, amount)
		}
	}

	for _, event := range events {
		//处理WSOL关闭账户，将WSOl账户余额+租金费 为关闭账户的转账
		if event.Type == "closeAccount" && IsATA(event.To, solana.WrappedSol.String(), event.From) {
			//此次交易剩余的WSOL
			net := tt.GetNetBalance(event.To, solana.WrappedSol.String())

			//账户开始金额，包含了租金费和余额
			pre, _ := getBalance(event.From, parsedTransaction)
			//关闭返还的SOL
			transfers = append(transfers, &Transfer{
				From:   event.From,
				To:     event.To,
				Amount: new(big.Int).Add(net, big.NewInt(int64(pre))).String(),
				Token:  consts.SOL,
			})

			//关闭账户的代币
			transfers = append(transfers, &Transfer{
				From:   event.To,
				To:     event.From,
				Amount: net.String(),
				Token:  solana.WrappedSol.String(),
			})
		}
	}

	return transfers, nil
}

func (s *SolParser) processInstructions(
	tx *rpc.GetParsedTransactionResult,
	getInstructions func(*rpc.GetParsedTransactionResult) []InstructionContext,
) []*TransferEvent {
	var events []*TransferEvent
	for _, ctx := range getInstructions(tx) {
		transferInsts, err := s.extractInstructions(tx, ctx)

		if err != nil {
			continue
		}

		if transferInsts == nil {
			continue
		}
		var event *TransferEvent

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

func (s *SolParser) ParseInstructionIntoTransferEvent(parsedTransaction *rpc.GetParsedTransactionResult, idxOuter int, transferIx *rpc.ParsedInstruction) (*TransferEvent, error) {

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
