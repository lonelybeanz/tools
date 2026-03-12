package geth

import (
	"encoding/hex"
	"strings"

	"github.com/ethereum/go-ethereum/core/types"
)

var (
	SwapDexTopic = map[string]string{
		"0xd78ad95fa46c994b6551d0da85fc275fe613ce37657fb8d5e3d130840159d822": "Topic0V2Swap",
		"0x19b47279256b2a23a1665c810c8d55a1758940ee09377d4f8d26497a3577dc83": "PancakeV3",
		"0xc42079f94a6350d7e6235f29174924f928cc2ac818eb64fed8004e115fbcca67": "UniswapV3",
		"0xde449b421e7f751324933a2c4afee2ea35f7c7d2b6bdf310e7a7017b4d67bb91": "BiV3",
		"0x04206ad2b7c0f463bff3dd4f33c5735b0f2957a351e4f79763a4fa9e775dd237": "CLPoolManager",
		"0xfec331350fce78ba658e082a71da20ac9f8d798a99b3c79681c8440cbfe77e07": "1Inch OrderFilled",
	}
	memeBot = []string{
		"0xceCCE97529EffC360f68b3F26eEF4ad74eBF5705",
		"0x10ED43C718714eb63d5aA57B78B54704E256024E",
		"0x1A0A18AC4BECDDbd6389559687d1A73d8927E416",
		"0x1b81D678ffb9C0263b24A97847620C99d213eB14",
		"0xDe44500b5d1479DF5C003bf48915b3E24Df3e8dD",
		"0xb300000b72DEAEb607a12d5f54773D1C19c7028d",
		"0x6aba0315493b7e6989041C91181337b662fB1b90",
		"0xc0e6EEF914d7BB0D4e6F72bc64ed69383fDb06E4",
		"0x1a1ec25DC08e98e5E93F1104B5e5cdD298707d31",
		"0x00000047bB99ea4D791bb749D970DE71EE0b1A34",
		"0x5c952063c7fc8610FFDB798152D69F0B9550762b",
		"0x13f4EA83D0bd40E75C8222255bc855a974568Dd4",
		"0x8f3930B7594232805dd780dC3B02F02cBf44016A",
		"0x111111125421cA6dc452d289314280a0f8842A65",
		"0xCA980F000771f70B15647069E9E541ef73F71f2f",
		"0xDa77c035e4d5A748b4aB6674327Fa446F17098A2",
		"0x1de460f363AF910f51726DEf188F9004276Bf4bc",
		"0xc205f591D395d59ad5bcB8bD824d8FA67ab4d15A",
	}
)

func GetTxFlag(logs []*types.Log, to string, data []byte) string {
	if len(logs) == 0 {
		goto SkipLogsCheck
	}

	for _, log := range logs {
		if len(log.Topics) > 0 {
			if _, ok := SwapDexTopic[strings.ToLower(log.Topics[0].Hex())]; ok {
				return "Swap"
			}
		}
	}

SkipLogsCheck:

	if to == "" {
		goto SkipToCheck
	}

	switch strings.ToLower(to) {
	case strings.ToLower("0x5c952063c7fc8610FFDB798152D69F0B9550762b"):
		return "Fourmeme"
	case strings.ToLower("0x1de460f363AF910f51726DEf188F9004276Bf4bc"):
		return "Gmgn"
	case strings.ToLower("0xc205f591D395d59ad5bcB8bD824d8FA67ab4d15A"):
		return "Debot"
	case strings.ToLower("0xCA980F000771f70B15647069E9E541ef73F71f2f"):
		return "Dragun"
	default:
		if Contains(memeBot, to) {
			return "Swap"
		}
	}

SkipToCheck:

	if data == nil {
		return ""
	}

	if len(data) < 4 {
		return "Transfer"
	} else {
		selectOp := hex.EncodeToString(data[:4])
		switch selectOp {
		case "f340fa01":
			return "Deposit"
		case "095ea7b3":
			return "Approve"
		case "2e1a7d4d":
			return "Withdraw"
		case "a9059cbb":
			return "Transfer"
		case "23b872dd":
			return "TransferFrom"
		default:
			return "other"
		}
	}
}

// 判断一个字符串是否在切片中
func Contains[T comparable](slice []T, value T) bool {
	for _, item := range slice {
		if item == value {
			return true
		}
	}
	return false
}
