package geth

import (
	"context"
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	types2 "github.com/ethereum/go-ethereum/core/types"
)

type TransferToken struct {
	Token  common.Address
	From   common.Address
	To     common.Address
	Amount *big.Int
	IsWBNB bool
}

func parseTxLogs(ctx context.Context, logs []*types2.Log) (map[common.Hash][]*TransferToken, map[common.Hash]bool) {
	if len(logs) == 0 {
		return nil, nil
	}

	mapTransferTokens := make(map[common.Hash][]*TransferToken)
	swapHash := make(map[common.Hash]bool)

	for _, log := range logs {

		transferToken, isSwap := ParseTokenEventLog(ctx, log)
		if isSwap {
			swapHash[log.TxHash] = true
		}

		if transferToken == nil {
			continue
		}

		mapTransferTokens[log.TxHash] = append(mapTransferTokens[log.TxHash], transferToken)

	}
	return mapTransferTokens, swapHash
}

var tokenParser *ERC20Parser

type ERC20Parser struct {
	TransferTopic   string
	WithdrawalTopic string
	DepositTopic    string
	SwapTpoic       []string
}

func NewERC20Parser() *ERC20Parser {
	return &ERC20Parser{
		TransferTopic:   "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef",
		WithdrawalTopic: "0x7fcf532c15f0a6db0bd6d0e038bea71d30d808c7d98cb3bf7268a95bf5081b65",
		DepositTopic:    "0xe1fffcc4923d04b559f4d29a8bfc6cda04eb5b0d3c460751c2402c5c5cc9109c",
		SwapTpoic: []string{
			"0xd78ad95fa46c994b6551d0da85fc275fe613ce37657fb8d5e3d130840159d822", //: "Topic0V2Swap",
			"0x19b47279256b2a23a1665c810c8d55a1758940ee09377d4f8d26497a3577dc83", //: "PancakeV3",
			"0xc42079f94a6350d7e6235f29174924f928cc2ac818eb64fed8004e115fbcca67", //: "UniswapV3",
			"0xde449b421e7f751324933a2c4afee2ea35f7c7d2b6bdf310e7a7017b4d67bb91", //: "BiV3",
			"0x04206ad2b7c0f463bff3dd4f33c5735b0f2957a351e4f79763a4fa9e775dd237", //: "CLPoolManager",
		},
	}
}

func (parser *ERC20Parser) ParseEventLog(ctx context.Context, log *types2.Log) (*TransferToken, bool, error) {
	if len(log.Topics) < 1 {
		return nil, false, errors.New("no topic found")
	}
	topic := log.Topics[0].Hex()
	var token *TransferToken
	var err error
	var isSwap bool

	if parser.isTransferTopic(topic) {
		token, err = parseTransferEventLog(log.Address, log.Topics, log.Data)
	} else if parser.isWithdrawalTopic(topic) {
		token, err = parseWithdrawalEventLog(log.Address, log.Topics, log.Data)
	} else if parser.isDepositTopic(topic) {
		token, err = parseDepositEventLog(log.Address, log.Topics, log.Data)
	} else if parser.isSwapTopic(topic) {
		isSwap = true
	} else {
		return nil, false, errors.New("not support topic")
	}
	return token, isSwap, err
}

func (parser *ERC20Parser) isTransferTopic(topic string) bool {
	return topic == parser.TransferTopic
}

func (parser *ERC20Parser) isWithdrawalTopic(topic string) bool {
	return topic == parser.WithdrawalTopic
}

func (parser *ERC20Parser) isDepositTopic(topic string) bool {
	return topic == parser.DepositTopic
}

func (parser *ERC20Parser) isSwapTopic(topic string) bool {
	return contains(parser.SwapTpoic, topic)
}

func ParseTokenEventLog(ctx context.Context, log *types2.Log) (*TransferToken, bool) {
	if tokenParser == nil {
		tokenParser = NewERC20Parser()
	}

	token, isSwap, err := tokenParser.ParseEventLog(ctx, log)
	if err != nil {
		return nil, false
	}
	return token, isSwap
}

func parseTransferEventLog(address common.Address, topics []common.Hash, data []byte) (*TransferToken, error) {
	if len(topics) < 3 || len(data) < 32 {
		return nil, errors.New("event log incorrect")
	}

	from := common.HexToAddress(topics[1].Hex())
	to := common.HexToAddress(topics[2].Hex())
	amountStr := data[0:32]
	amount := new(big.Int).SetBytes(amountStr)
	return &TransferToken{
		Token:  address,
		From:   from,
		To:     to,
		Amount: amount,
		IsWBNB: false,
	}, nil
}

func parseWithdrawalEventLog(address common.Address, topics []common.Hash, data []byte) (*TransferToken, error) {
	if len(topics) < 2 || len(data) < 32 {
		return nil, errors.New("event log incorrect")
	}

	to := common.HexToAddress(topics[1].Hex())
	amountStr := data[0:32]
	amount := new(big.Int).SetBytes(amountStr)
	return &TransferToken{
		Token:  address,
		From:   to,
		To:     address,
		Amount: amount,
		IsWBNB: true,
	}, nil
}

func parseDepositEventLog(address common.Address, topics []common.Hash, data []byte) (*TransferToken, error) {
	if len(topics) < 2 || len(data) < 32 {
		return nil, errors.New("event log incorrect")
	}

	to := common.HexToAddress(topics[1].Hex())
	amountStr := data[0:32]
	amount := new(big.Int).SetBytes(amountStr)

	return &TransferToken{
		Token:  address,
		From:   address,
		To:     to,
		Amount: amount,
		IsWBNB: true,
	}, nil
}

func HexToBigInt(hex string) *big.Int {
	return new(big.Int).SetBytes(common.FromHex(hex))
}
