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

func parseTxLogs(ctx context.Context, logs []*types2.Log) map[common.Hash][]*TransferToken {
	if len(logs) == 0 {
		return nil
	}

	mapTransferTokens := make(map[common.Hash][]*TransferToken)

	for _, log := range logs {

		transferToken := ParseTokenEventLog(ctx, log)
		if transferToken == nil {
			continue
		}

		mapTransferTokens[log.TxHash] = append(mapTransferTokens[log.TxHash], transferToken)

	}
	return mapTransferTokens
}

var tokenParser *ERC20Parser

type ERC20Parser struct {
	TransferTopic   string
	WithdrawalTopic string
	DepositTopic    string
}

func NewERC20Parser() *ERC20Parser {
	return &ERC20Parser{
		TransferTopic:   "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef",
		WithdrawalTopic: "0x7fcf532c15f0a6db0bd6d0e038bea71d30d808c7d98cb3bf7268a95bf5081b65",
		DepositTopic:    "0xe1fffcc4923d04b559f4d29a8bfc6cda04eb5b0d3c460751c2402c5c5cc9109c",
	}
}

func (parser *ERC20Parser) ParseEventLog(ctx context.Context, log *types2.Log) (*TransferToken, error) {
	if len(log.Topics) < 1 {
		return nil, errors.New("no topic found")
	}
	topic := log.Topics[0].Hex()
	var token *TransferToken
	var err error

	if parser.isTransferTopic(topic) {
		token, err = parseTransferEventLog(log.Address, log.Topics, log.Data)
	} else if parser.isWithdrawalTopic(topic) {
		token, err = parseWithdrawalEventLog(log.Address, log.Topics, log.Data)
	} else if parser.isDepositTopic(topic) {
		token, err = parseDepositEventLog(log.Address, log.Topics, log.Data)
	} else {
		return nil, errors.New("not support topic")
	}
	return token, err
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

func ParseTokenEventLog(ctx context.Context, log *types2.Log) *TransferToken {
	if tokenParser == nil {
		tokenParser = NewERC20Parser()
	}

	token, err := tokenParser.ParseEventLog(ctx, log)
	if err != nil {
		return nil
	}
	return token
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
