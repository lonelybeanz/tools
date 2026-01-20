package parser

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/lonelybeanz/tools/pkg/solparser/consts"
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

type TokenAccountInfo struct {
	Mint  string `json:"mint"`
	Owner string `json:"owner"`
}

type SolParser struct {
	cli               *rpc.Client
	tokenAccountCache map[string]TokenAccountInfo
}

func NewSolParser(cli *rpc.Client, cache map[string]TokenAccountInfo) *SolParser {
	if cache == nil {
		cache = make(map[string]TokenAccountInfo)
	}
	return &SolParser{
		cli:               cli,
		tokenAccountCache: cache,
	}
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

	for _, event := range events {
		// Fill token amounts
		if err := s.fillTokenAmounts(event); err != nil {
			return nil, fmt.Errorf("filling in token amount: %w", err)
		}
		sourcePk, errSource := solana.PublicKeyFromBase58(event.Token.From)
		destPk, errDest := solana.PublicKeyFromBase58(event.Token.To)

		// 检查这是否是一次“包装”操作：一个所有者将SOL转入其自身的WSOL ATA
		if errSource == nil && errDest == nil && IsATA(sourcePk, solana.WrappedSol, destPk) && event.Type == "transferAta" {
			events = append(events, &types.TransferEvent{
				EventIndex: event.EventIndex,
				Type:       "wrap",
				Token: types.TokenAmt{
					From:   destPk.String(), // 系统程序
					Code:   solana.WrappedSol.String(),
					Amount: event.Token.Amount,
					To:     sourcePk.String(),
				},
			}) // 接收WSOL的所有者
		}
	}

	tt := NewTransferTracker("")
	//处理WSOL关闭账户，将WSOl账户余额+租金费 为关闭账户的转账
	for _, event := range events {
		amount, _ := new(big.Int).SetString(event.Token.Amount, 10)
		tt.AddTransfer(event.Token.From, event.Token.To, event.Token.Code, amount)
	}

	for _, event := range events {
		if event.Type == "closeAccount" {
			net := tt.GetNetBalance(event.Token.To, solana.WrappedSol.String())
			rentFee, _ := new(big.Int).SetString(event.Token.Amount, 10)
			all := new(big.Int).Add(net, rentFee)
			event.Token.Amount = all.String()
			event.Token.Code = consts.SOL

			events = append(events, &types.TransferEvent{
				EventIndex: event.EventIndex,
				Type:       "closeAccount2",
				Token: types.TokenAmt{
					From:   event.Token.To,
					Code:   solana.WrappedSol.String(),
					Amount: net.String(),
					To:     event.Token.From,
				},
			})
		}
	}

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
func (s *SolParser) fillTokenAmounts(transEvent *types.TransferEvent) error {
	if err := s.FillTokenAmtWithTransferIx(transEvent); err != nil {
		return fmt.Errorf("filling in token amount: %w", err)
	}
	return nil
}

func (s *SolParser) FillTokenAmtWithTransferIx(transEvent *types.TransferEvent) error {

	// 遵循以“所有者”为中心的模型来追踪资金流动

	// 场景1: Token Program (e.g. SPL Token, WSOL)
	// 对于SPL代币转账，我们将ATA地址解析为其所有者地址。
	if transEvent.Type == "tokenTransfer" {
		sourceInfo, _ := s.RetryGetTokenAccountInfoByTokenAccount(transEvent.Token.From)
		destInfo, _ := s.RetryGetTokenAccountInfoByTokenAccount(transEvent.Token.To)

		if sourceInfo != nil {
			transEvent.Token.From = sourceInfo.Owner.String()
			transEvent.Token.Code = sourceInfo.Mint.String()
		}

		if destInfo != nil {
			transEvent.Token.To = destInfo.Owner.String()
			if transEvent.Token.Code == "" { // 如果无法从源头确定代币，则从目标确定
				transEvent.Token.Code = destInfo.Mint.String()
			}
		}
		return nil
	}

	// 场景2: System Program (普通SOL转账 或 SOL -> WSOL包装)
	if transEvent.Type == "systemTransfer" {
		sourcePk, errSource := solana.PublicKeyFromBase58(transEvent.Token.From)
		destPk, errDest := solana.PublicKeyFromBase58(transEvent.Token.To)

		// 检查这是否是一次“包装”操作：一个所有者将SOL转入其自身的WSOL ATA
		if errSource == nil && errDest == nil && IsATA(sourcePk, solana.WrappedSol, destPk) {
			// 这是Wrap操作。我们将其建模为所有者收到了WSOL。
			transEvent.Token.Code = consts.SOL
			transEvent.Token.From = sourcePk.String() // 虚拟来源，表示“铸造”
			transEvent.Token.To = destPk.String()     // 接收WSOL的所有者
			transEvent.Type = "transferAta"
		}
		// 如果不是Wrap操作，那么它就是一笔普通的SOL转账，原始的tkAmt是正确的。
		return nil
	}

	return nil
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
	if s.tokenAccountCache != nil {
		if tokenAccountInfo, ok := s.tokenAccountCache[tokenAccount]; ok {
			t := &token.TokenAccount{
				Mint:  common.PublicKeyFromString(tokenAccountInfo.Mint),
				Owner: common.PublicKeyFromString(tokenAccountInfo.Owner),
			}
			return t, nil
		}
	}

	ctx := context.Background()
	accountInfo, e := s.cli.GetAccountInfo(ctx, solana.MustPublicKeyFromBase58(tokenAccount))
	if e != nil {
		return nil, e
	}
	if accountInfo.Value != nil && accountInfo.Value.Owner == solana.SystemProgramID {
		uint64One := uint64(1)
		// 账户本身就是一个 lamport 账户，非Token账户
		t := &token.TokenAccount{
			Mint:            common.PublicKeyFromString(consts.SOL),
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

// SerializeTokenAccount 将TokenAccount序列化为[]byte
func (s *SolParser) SerializeTokenAccount(acc *token.TokenAccount) []byte {
	data := make([]byte, 165) // TokenAccount固定大小为165字节

	// 写入Mint地址 (32字节)
	copy(data[0:32], acc.Mint.Bytes())

	// 写入Owner地址 (32字节)
	copy(data[32:64], acc.Owner.Bytes())

	// 写入Amount (8字节)
	binary.LittleEndian.PutUint64(data[64:72], acc.Amount)

	// 写入Delegate (4字节标志 + 32字节地址)
	if acc.Delegate != nil {
		copy(data[72:76], []byte{1, 0, 0, 0}) // Some标记
		copy(data[76:108], acc.Delegate.Bytes())
	} else {
		copy(data[72:76], []byte{0, 0, 0, 0}) // None标记
	}

	// 写入State (1字节)
	data[108] = byte(acc.State)

	// 写入IsNative (4字节标志 + 8字节数值)
	if acc.IsNative != nil {
		copy(data[109:113], []byte{1, 0, 0, 0}) // Some标记
		binary.LittleEndian.PutUint64(data[113:121], *acc.IsNative)
	} else {
		copy(data[109:113], []byte{0, 0, 0, 0}) // None标记
	}

	// 写入DelegatedAmount (8字节)
	binary.LittleEndian.PutUint64(data[121:129], acc.DelegatedAmount)

	// 写入CloseAuthority (4字节标志 + 32字节地址)
	if acc.CloseAuthority != nil {
		copy(data[129:133], []byte{1, 0, 0, 0}) // Some标记
		copy(data[133:165], acc.CloseAuthority.Bytes())
	} else {
		copy(data[129:133], []byte{0, 0, 0, 0}) // None标记
	}

	return data
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
